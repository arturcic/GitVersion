package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type projectXML struct {
	name          xml.Name
	attrs         []xml.Attr
	content       []projectXMLContent
	defaultNS     string
	preserveSpace bool
	selfClosing   bool
}

type projectXMLContent struct {
	element *projectXML
	token   xml.Token
	cdata   bool
}

// RawToken retains the prefixes and the input offsets distinguish CDATA from
// ordinary character data and empty tags from explicit closing tags.
func parseProjectXML(input string) (*projectXML, error) {
	decoder := xml.NewDecoder(strings.NewReader(input))
	// The caller already decoded the file according to its BOM.
	decoder.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	var root *projectXML
	var doctype bool
	var stack []*projectXML
	for {
		start := decoder.InputOffset()
		token, err := decoder.RawToken()
		if err == io.EOF {
			if root == nil || len(stack) != 0 {
				return nil, fmt.Errorf("incomplete XML document")
			}
			return root, nil
		}
		if err != nil {
			return nil, err
		}
		var parent *projectXML
		if len(stack) != 0 {
			parent = stack[len(stack)-1]
		}
		switch token := token.(type) {
		case xml.StartElement:
			element := &projectXML{
				name: token.Name, attrs: token.Copy().Attr,
				selfClosing: strings.HasSuffix(input[start:decoder.InputOffset()], "/>"),
			}
			if parent != nil {
				element.defaultNS, element.preserveSpace = parent.defaultNS, parent.preserveSpace
				parent.content = append(parent.content, projectXMLContent{element: element})
			} else {
				if root != nil {
					return nil, fmt.Errorf("multiple root elements")
				}
				root = element
			}
			attributes := map[xml.Name]bool{}
			for _, attr := range element.attrs {
				if attributes[attr.Name] {
					return nil, fmt.Errorf("duplicate XML attribute %q", attr.Name.Local)
				}
				attributes[attr.Name] = true
				switch attr.Name {
				case xml.Name{Local: "xmlns"}:
					element.defaultNS = attr.Value
				case xml.Name{Space: "xml", Local: "space"}:
					element.preserveSpace = attr.Value == "preserve"
				}
			}
			stack = append(stack, element)
		case xml.EndElement:
			if parent == nil || parent.name != token.Name {
				return nil, fmt.Errorf("unexpected XML closing element %q", token.Name.Local)
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			cdata := strings.HasPrefix(input[start:decoder.InputOffset()], "<![CDATA[")
			whitespace := strings.Trim(string(token), " \r\n\t") == ""
			if parent == nil && !whitespace {
				return nil, fmt.Errorf("text outside root element")
			}
			if parent != nil && (cdata || !whitespace || parent.preserveSpace) {
				parent.content = append(parent.content, projectXMLContent{token: token.Copy(), cdata: cdata})
			}
		case xml.Comment, xml.ProcInst:
			if parent != nil {
				parent.content = append(parent.content, projectXMLContent{token: xml.CopyToken(token)})
			}
		case xml.Directive:
			if root != nil || doctype || !strings.HasPrefix(string(token), "DOCTYPE ") {
				return nil, fmt.Errorf("unexpected XML directive")
			}
			doctype = true
		}
	}
}

func (element *projectXML) matches(name string) bool {
	return element.name.Local == name && element.name.Space == "" && element.defaultNS == ""
}

func (element *projectXML) collectPropertyGroups(groups *[]*projectXML) {
	for _, child := range element.content {
		if child.element != nil {
			if child.element.matches("PropertyGroup") {
				*groups = append(*groups, child.element)
			}
			child.element.collectPropertyGroups(groups)
		}
	}
}

func (element *projectXML) lastElement(name string) *projectXML {
	for i := len(element.content) - 1; i >= 0; i-- {
		if child := element.content[i].element; child != nil && child.matches(name) {
			return child
		}
	}
	return nil
}

func (element *projectXML) value() string {
	var text strings.Builder
	for _, child := range element.content {
		if child.element != nil {
			text.WriteString(child.element.value())
		} else if data, ok := child.token.(xml.CharData); ok {
			text.Write(data)
		}
	}
	return text.String()
}

func projectXMLName(name xml.Name) string {
	if name.Space != "" {
		return name.Space + ":" + name.Local
	}
	return name.Local
}

var projectXMLText = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\r", "&#xD;")
var projectXMLAttribute = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "\r", "&#xD;", "\n", "&#xA;", "\t", "&#x9;")

func (element *projectXML) render() string {
	var output strings.Builder
	element.writeXML(&output, 0, false)
	return output.String()
}

func (element *projectXML) writeXML(output *strings.Builder, depth int, mixed bool) {
	name := projectXMLName(element.name)
	output.WriteString("<" + name)
	for _, attr := range element.attrs {
		output.WriteString(" " + projectXMLName(attr.Name) + `="` + projectXMLAttribute.Replace(attr.Value) + `"`)
	}
	if element.selfClosing && len(element.content) == 0 {
		output.WriteString(" />")
		return
	}
	output.WriteByte('>')
	for _, child := range element.content {
		if data, ok := child.token.(xml.CharData); ok {
			mixed = true
			if child.cdata {
				output.WriteString("<![CDATA[" + string(data) + "]]>")
			} else {
				output.WriteString(projectXMLText.Replace(string(data)))
			}
			continue
		}
		if !mixed {
			output.WriteString("\n" + strings.Repeat("  ", depth+1))
		}
		if child.element != nil {
			child.element.writeXML(output, depth+1, mixed)
		} else {
			switch token := child.token.(type) {
			case xml.Comment:
				output.WriteString("<!--" + string(token) + "-->")
			case xml.ProcInst:
				output.WriteString("<?" + token.Target + " " + string(token.Inst) + "?>")
			}
		}
	}
	if len(element.content) != 0 && !mixed {
		output.WriteString("\n" + strings.Repeat("  ", depth))
	}
	output.WriteString("</" + name + ">")
}

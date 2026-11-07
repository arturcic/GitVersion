package config

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"gopkg.in/yaml.v3"
)

// These are the pinned C# assembly's embedded workflow resources.
//
//go:embed presets/*/*.yml
var workflowPresets embed.FS

type ValidationError struct {
	Code, PropertyPath, Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("Configuration property '%s': %s", e.PropertyPath, e.Reason)
}

func validation(code, path, reason string) error {
	return &ValidationError{Code: "GVCONFIG_" + code, PropertyPath: path, Reason: reason}
}

type YAMLError struct{ Cause error }

func (e *YAMLError) Error() string { return e.Cause.Error() }
func (e *YAMLError) Unwrap() error { return e.Cause }

type LayoutError struct{ Cause error }

func (e *LayoutError) Error() string { return e.Cause.Error() }
func (e *LayoutError) Unwrap() error { return e.Cause }

func readConfigurationMap(data []byte) (*yaml.Node, error) {
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("configuration exceeds 1 MiB")
	}
	var doc, extra yaml.Node
	d := yaml.NewDecoder(bytes.NewReader(data))
	if err := d.Decode(&doc); err != nil {
		if err == io.EOF {
			return documentMap(), nil
		}
		return nil, &YAMLError{Cause: err}
	}
	if err := d.Decode(&extra); err != io.EOF {
		return nil, &YAMLError{Cause: fmt.Errorf("multiple configuration documents are unsupported")}
	}
	if doc.Content[0].Tag == "!!null" {
		return documentMap(), nil
	}
	return doc.Content[0], nil
}

type ResolvedConfiguration struct {
	flat *yaml.Node
}

// ResolveConfiguration reads file content in the selected public layout.
// Runtime overrides accept either layout and take precedence over the file.
func ResolveConfiguration(data, overrides []byte, version string) (ResolvedConfiguration, error) {
	var result ResolvedConfiguration
	selected, err := SelectConfigurationVersion(&version)
	if err != nil {
		return result, err
	}
	workflow := "GitFlow/v1"
	var layers []*yaml.Node
	for _, layer := range []struct {
		data           []byte
		layout, source string
	}{{data, selected, "configuration file"}, {overrides, "", "runtime override configuration"}} {
		node, err := readConfigurationMap(layer.data)
		if err != nil {
			return result, err
		}
		flat, err := normalizeDocument(node, layer.layout, layer.source)
		if err != nil {
			var property *ValidationError
			if errors.As(err, &property) {
				return result, err
			}
			return result, &LayoutError{Cause: err}
		}
		if err := validateFields(flat, rootField, "", selected); err != nil {
			return result, err
		}
		if v := nodeField(flat, "workflow"); v != nil && v.Tag != "!!null" {
			workflow = v.Value
		}
		layers = append(layers, flat)
	}
	if strings.TrimSpace(workflow) == "" {
		return result, validation("WORKFLOW", "workflow", "Expected an embedded workflow name or null for default configuration.")
	}
	switch workflow {
	case "GitFlow/v1", "GitHubFlow/v1", "TrunkBased/preview1":
	default:
		return result, validation("WORKFLOW", "workflow", "Unknown workflow. Select an embedded workflow.")
	}
	raw, err := workflowPresets.ReadFile("presets/" + workflow + ".yml")
	if err != nil {
		return result, err
	}
	preset, err := readConfigurationMap(raw)
	if err != nil {
		return result, err
	}
	effective, err := normalizeDocument(preset, "v7", "embedded workflow")
	if err != nil {
		return result, err
	}
	for _, layer := range layers {
		mergeConfiguration(effective, layer)
	}
	if err := finalizeResolved(effective, selected); err != nil {
		return result, err
	}
	return ResolvedConfiguration{flat: effective}, nil
}

func nodeField(node *yaml.Node, name string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == name {
			return node.Content[i+1]
		}
	}
	return nil
}

func mergeConfiguration(target, source *yaml.Node) {
	for i := 0; i < len(source.Content); i += 2 {
		key, value := source.Content[i], source.Content[i+1]
		existing := nodeField(target, key.Value)
		if existing == nil {
			putDocumentField(target, key.Value, cloneNode(value))
		} else if existing.Kind == yaml.MappingNode && value.Kind == yaml.MappingNode {
			mergeConfiguration(existing, value)
		} else {
			*existing = *cloneNode(value)
		}
	}
}

func stringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func listNode() *yaml.Node { return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"} }

func fieldValue(node *yaml.Node, name string) string {
	if field := nodeField(node, name); field != nil && field.Tag != "!!null" {
		return field.Value
	}
	return ""
}

func publicPath(path, version string, output bool) string {
	if version != "v7" || path == "workflow" {
		return path
	}
	if output {
		return "output." + path
	}
	return "calculation." + path
}

func finalizeResolved(flat *yaml.Node, version string) error {
	if next := fieldValue(flat, "next-version"); next != "" {
		mode := semver.ParseMode(fieldValue(flat, "semantic-version-format"))
		if mode == "" {
			mode = semver.Strict
		}
		if _, err := semver.ParseDetailed(next, nil, mode); err != nil {
			return validation("VALUE", publicPath("next-version", version, false), "Expected a semantic version.")
		}
	}
	checkRegex := func(path string, value *yaml.Node) error {
		if value != nil && value.Tag != "!!null" {
			if _, err := patterns.Compile(value.Value, patterns.IgnoreCase); err != nil {
				return validation("REGEX", publicPath(path, version, false), "Invalid regular expression.")
			}
		}
		return nil
	}
	for _, name := range strings.Fields("tag-prefix version-in-branch-pattern major-version-bump-message minor-version-bump-message patch-version-bump-message no-bump-message version-bump-reset-message regex") {
		if err := checkRegex(name, nodeField(flat, name)); err != nil {
			return err
		}
	}
	if ignores := nodeField(flat, "ignore"); ignores != nil {
		for _, name := range []string{"branches", "tags", "paths"} {
			if list := nodeField(ignores, name); list != nil {
				for _, value := range list.Content {
					if err := checkRegex("ignore."+name, value); err != nil {
						return err
					}
				}
			}
		}
	}
	if formats := nodeField(flat, "merge-message-formats"); formats != nil {
		for i := 0; i < len(formats.Content); i += 2 {
			if err := checkRegex("merge-message-formats."+formats.Content[i].Value, formats.Content[i+1]); err != nil {
				return err
			}
		}
	}
	branches := nodeField(flat, "branches")
	if branches == nil {
		return nil
	}
	for i := 0; i < len(branches.Content); i += 2 {
		branch := branches.Content[i+1]
		for _, property := range []string{"source-branches", "is-source-branch-for"} {
			if nodeField(branch, property) == nil {
				putDocumentField(branch, property, listNode())
			}
		}
		if field := nodeField(branch, "prevent-increment"); field == nil {
			putDocumentField(branch, "prevent-increment", documentMap())
		}
		if field := nodeField(branch, "increment"); field == nil {
			putDocumentField(branch, "increment", stringNode("Inherit"))
		}
	}
	for i := 0; i < len(branches.Content); i += 2 {
		name, branch := branches.Content[i].Value, branches.Content[i+1]
		for _, ref := range nodeField(branch, "is-source-branch-for").Content {
			target := nodeField(branches, ref.Value)
			if target == nil {
				return missingBranch(name, "is-source-branch-for", version)
			}
			links := nodeField(target, "source-branches")
			if !slices.ContainsFunc(links.Content, func(item *yaml.Node) bool { return item.Value == name }) {
				links.Content = append(links.Content, stringNode(name))
			}
		}
	}
	for i := 0; i < len(branches.Content); i += 2 {
		name, branch := branches.Content[i].Value, branches.Content[i+1]
		path := "branches." + name + ".regex"
		expression := nodeField(branch, "regex")
		if expression == nil || expression.Tag == "!!null" {
			return validation("BRANCH", publicPath(path, version, false), "Branch configuration requires a regex.")
		}
		if err := checkRegex(path, expression); err != nil {
			return err
		}
		for _, ref := range nodeField(branch, "source-branches").Content {
			if nodeField(branches, ref.Value) == nil {
				return missingBranch(name, "source-branches", version)
			}
		}
	}
	return nil
}

func missingBranch(name, property, version string) error {
	return validation("BRANCH", publicPath("branches."+name+"."+property, version, false), "Referenced branch configuration does not exist.")
}

func (c ResolvedConfiguration) Serialize(version string) ([]byte, error) {
	node := cloneNode(c.flat)
	if version == "v7" {
		var err error
		node, err = nestDocument(node)
		if err != nil {
			return nil, err
		}
	}
	orderConfiguration(node, false)
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	err := encoder.Encode(node)
	return out.Bytes(), err
}

func orderConfiguration(node *yaml.Node, preserve bool) {
	if node.Kind != yaml.MappingNode {
		if node.Kind == yaml.SequenceNode {
			node.Style = 0
		}
		return
	}
	node.Style = 0
	type pair struct{ key, value *yaml.Node }
	var entries []pair
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if value.Tag == "!!null" {
			continue
		}
		key.Style = 0
		orderConfiguration(value, key.Value == "branches" || key.Value == "merge-message-formats")
		entries = append(entries, pair{key, value})
	}
	if !preserve {
		sort.Slice(entries, func(i, j int) bool { return entries[i].key.Value < entries[j].key.Value })
	}
	node.Content = node.Content[:0]
	for _, p := range entries {
		node.Content = append(node.Content, p.key, p.value)
	}
}

func stringScalar(n *yaml.Node, field string) {
	// SharpYaml reads integer next-version values as version numbers.
	if field == "next-version" && n.Tag == "!!int" {
		if number, err := strconv.ParseInt(n.Value, 10, 64); err == nil {
			n.Value = strconv.FormatInt(number, 10) + ".0"
		}
	}
	n.Tag, n.Style = "!!str", 0
}

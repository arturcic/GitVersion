package config

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"strings"
)

// LegacyDocument retains loader values before effective-calculation validation.
type LegacyDocument struct {
	Ignore      IgnoreSettings
	Strategies  map[string]bool
	NextVersion string
	nested      *yaml.Node
}

func ReadLegacyDocument(data []byte) (LegacyDocument, error) {
	var out LegacyDocument
	var doc yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&doc); err != nil {
		if err == io.EOF {
			return ReadLegacyDocument([]byte("{}"))
		}
		return out, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return out, fmt.Errorf("multiple configuration documents are unsupported")
	}
	if len(doc.Content) != 1 {
		return out, fmt.Errorf("configuration requires a mapping")
	}
	flat, err := normalizeDocument(doc.Content[0], "v6", "configuration")
	if err != nil {
		return out, err
	}
	keys := append([]string{"mode", "next-version", "branches", "strategies", "ignore", "tag-pre-release-weight"}, outputSettingNames...)
	fields, err := mapping(flat, "legacy", append([]string{"workflow"}, keys...))
	if err != nil {
		return out, err
	}
	root := documentMap()
	workflow := fields["workflow"]
	if workflow == nil {
		workflow = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "GitFlow/v1"}
	}
	if branches := fields["branches"]; branches != nil {
		// Nesting drops empty branch maps. Validate names before this projection
		// so an unsupported branch cannot bypass effective validation.
		settings := Config{Workflow: workflow.Value}
		if _, err := mapping(branches, "branches", settings.Kinds()); err != nil {
			return out, err
		}
	}
	putDocumentField(root, "workflow", workflow)
	for _, key := range keys {
		value := fields[key]
		if value == nil {
			continue
		}
		if key == "ignore" {
			out.Ignore, err = parseIgnore(value)
			if err != nil {
				return out, err
			}
		}
		if key == "next-version" {
			if value.Kind != yaml.ScalarNode || value.Anchor != "" {
				return out, fmt.Errorf("next-version requires a scalar")
			}
			copy := *value
			copy.Tag = "!!str"
			value = &copy
			out.NextVersion = value.Value
		}
		if key == "strategies" {
			if value.Kind == yaml.ScalarNode {
				s, e := scalar(value, "strategies")
				if e != nil {
					return out, e
				}
				list := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
				for _, part := range strings.Split(s, ",") {
					list.Content = append(list.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(part)})
				}
				value = list
			}
			if value.Kind != yaml.SequenceNode || value.Anchor != "" {
				return out, fmt.Errorf("strategies requires names")
			}
			out.Strategies = map[string]bool{}
			for _, node := range value.Content {
				name, e := scalar(node, "strategies")
				if e != nil {
					return out, e
				}
				switch name {
				case "None", "Fallback", "ConfiguredNextVersion", "MergeMessage", "TaggedCommit", "TrackReleaseBranches", "VersionInBranchName", "Mainline":
					out.Strategies[name] = true
				default:
					return out, fmt.Errorf("unknown strategy %q", name)
				}
			}
		}
		putDocumentField(root, key, value)
	}
	out.nested, err = nestDocument(root)
	if err != nil {
		return out, err
	}
	return out, nil
}
func SelectConfigurationVersion(value *string) (string, error) {
	text := ""
	if value != nil {
		text = strings.ToLower(strings.TrimSpace(*value))
	}
	switch text {
	case "", "v7":
		return "v7", nil
	case "v6":
		return "v6", nil
	default:
		return "", fmt.Errorf("unrecognized GITVERSION_CONFIGURATION_VERSION value %q; valid values are v6 and v7", *value)
	}
}
func IsExplicitV6(value *string) bool {
	return value != nil && strings.EqualFold(strings.TrimSpace(*value), "v6")
}
func ParseSelected(data []byte, version string) (Config, error) {
	selected, err := SelectConfigurationVersion(&version)
	if err != nil {
		return Config{}, err
	}
	switch selected {
	case "", "v7":
		return Parse(data)
	case "v6":
		doc, err := ReadLegacyDocument(data)
		if err != nil {
			return Config{}, err
		}
		raw, err := yaml.Marshal(doc.nested)
		if err != nil {
			return Config{}, err
		}
		return Parse(raw)
	default:
		return Config{}, fmt.Errorf("unsupported configuration version %q", version)
	}
}

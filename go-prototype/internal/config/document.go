package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// These field sets describe the pinned document layout. Effective calculation
// has its own allowlists for the settings the calculator supports.
var outputProperties = strings.Fields(
	"assembly-file-versioning-format assembly-file-versioning-scheme assembly-informational-format " +
		"assembly-versioning-format assembly-versioning-scheme commit-date-format custom-version-format " +
		"pre-release-weight tag-pre-release-weight update-build-number",
)
var outputBranchProperties = strings.Fields("custom-version-format pre-release-weight")
var calculationBranchProperties = strings.Fields(
	"mode label increment prevent-increment track-merge-target track-merge-message " +
		"commit-message-incrementing regex source-branches is-source-branch-for tracks-release-branches " +
		"is-release-branch is-main-branch",
)
var calculationProperties = append(append([]string{}, calculationBranchProperties...), strings.Fields(
	"tag-prefix version-in-branch-pattern next-version major-version-bump-message minor-version-bump-message "+
		"patch-version-bump-message no-bump-message version-bump-reset-message merge-message-formats "+
		"semantic-version-format strategies branches ignore",
)...)

func documentKind(fields map[string]*yaml.Node) string {
	if len(fields) == 0 {
		return "Empty"
	}
	if len(fields) == 1 && fields["workflow"] != nil {
		return "Shared"
	}
	nested := fields["calculation"] != nil || fields["output"] != nil
	for name := range fields {
		if name != "workflow" && name != "calculation" && name != "output" {
			if nested {
				return "Mixed"
			}
			return "V6"
		}
	}
	return "V7"
}

func cloneNode(node *yaml.Node) *yaml.Node {
	copy := *node
	copy.Content = make([]*yaml.Node, len(node.Content))
	for i, child := range node.Content {
		copy.Content[i] = cloneNode(child)
	}
	return &copy
}

func documentMap() *yaml.Node { return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"} }

func putDocumentField(node *yaml.Node, name string, value *yaml.Node) {
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}, value)
}

// normalizeDocument returns a flat document. An empty version selects the
// internal path, which accepts either layout.
func normalizeDocument(node *yaml.Node, version, source string) (*yaml.Node, error) {
	fields, err := mapping(node, source, nil)
	if err != nil {
		return nil, err
	}
	switch documentKind(fields) {
	case "Mixed":
		return nil, fmt.Errorf("%s combines v7 sections with unsupported root-level properties", source)
	case "V6":
		if version == "v7" {
			return nil, fmt.Errorf("%s uses the legacy v6 configuration structure, but v7 is selected", source)
		}
	case "V7":
		if version == "v6" {
			return nil, fmt.Errorf("%s uses the v7 configuration structure, but v6 is selected", source)
		}
		return flattenDocument(fields)
	}
	return cloneNode(node), nil
}

func validateDocumentOwnership(section, name string, branch bool) error {
	path := ""
	calculation, output := calculationProperties, outputProperties
	if branch {
		path = "branches.<branch>."
		calculation, output = calculationBranchProperties, outputBranchProperties
	}
	if name == "workflow" {
		return fmt.Errorf("configuration property '%s.%sworkflow' belongs at the root as 'workflow'", section, path)
	}
	expected := ""
	if containsProperty(output, name) {
		expected = "output"
	} else if containsProperty(calculation, name) {
		expected = "calculation"
	}
	if expected != "" && section != expected {
		return fmt.Errorf("configuration property '%s.%s%s' belongs under '%s.%s%s'", section, path, name, expected, path, name)
	}
	return nil
}

func containsProperty(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

func flattenDocument(fields map[string]*yaml.Node) (*yaml.Node, error) {
	out := documentMap()
	if workflow := fields["workflow"]; workflow != nil {
		putDocumentField(out, "workflow", cloneNode(workflow))
	}
	seen := map[string]bool{}
	branches := documentMap()
	byName := map[string]*yaml.Node{}
	for _, section := range []string{"calculation", "output"} {
		node := fields[section]
		if node == nil || node.Tag == "!!null" {
			continue
		}
		properties, err := mapping(node, section, nil)
		if err != nil {
			return nil, err
		}
		for i := 0; i < len(node.Content); i += 2 {
			name := node.Content[i].Value
			value := properties[name]
			if name == "branches" {
				if err := mergeDocumentBranches(branches, byName, value, section); err != nil {
					return nil, err
				}
				continue
			}
			if err := validateDocumentOwnership(section, name, false); err != nil {
				return nil, err
			}
			if seen[name] {
				return nil, fmt.Errorf("configuration property %q is defined in both sections", name)
			}
			seen[name] = true
			putDocumentField(out, name, cloneNode(value))
		}
	}
	if len(branches.Content) != 0 {
		putDocumentField(out, "branches", branches)
	}
	return out, nil
}

func mergeDocumentBranches(target *yaml.Node, byName map[string]*yaml.Node, source *yaml.Node, section string) error {
	if _, err := mapping(source, section+".branches", nil); err != nil {
		reason := "Expected a mapping."
		if source.Tag == "!!null" {
			reason = "Expected a non-null configuration value."
		}
		return validation("PROPERTY", section+".branches", reason)
	}
	for i := 0; i < len(source.Content); i += 2 {
		name, branch := source.Content[i].Value, source.Content[i+1]
		if _, err := mapping(branch, section+".branches."+name, nil); err != nil {
			return err
		}
		destination := byName[name]
		if destination == nil {
			destination = documentMap()
			byName[name] = destination
			putDocumentField(target, name, destination)
		}
		existing, _ := mapping(destination, "branches."+name, nil)
		for j := 0; j < len(branch.Content); j += 2 {
			property := branch.Content[j].Value
			if err := validateDocumentOwnership(section, property, true); err != nil {
				return err
			}
			if existing[property] != nil {
				return fmt.Errorf("configuration branch property %q is defined in both sections", name+"."+property)
			}
			putDocumentField(destination, property, cloneNode(branch.Content[j+1]))
		}
	}
	return nil
}

func nestDocument(flat *yaml.Node) (*yaml.Node, error) {
	if _, err := mapping(flat, "legacy", nil); err != nil {
		return nil, err
	}
	root, calculation, output := documentMap(), documentMap(), documentMap()
	for i := 0; i < len(flat.Content); i += 2 {
		name, value := flat.Content[i].Value, flat.Content[i+1]
		switch {
		case name == "workflow":
			putDocumentField(root, name, cloneNode(value))
		case name == "branches":
			if err := splitDocumentBranches(calculation, output, value); err != nil {
				return nil, err
			}
		case containsProperty(outputProperties, name):
			putDocumentField(output, name, cloneNode(value))
		default:
			putDocumentField(calculation, name, cloneNode(value))
		}
	}
	putDocumentField(root, "calculation", calculation)
	putDocumentField(root, "output", output)
	return root, nil
}

func splitDocumentBranches(calculation, output, branches *yaml.Node) error {
	if branches.Kind != yaml.MappingNode {
		putDocumentField(calculation, "branches", cloneNode(branches))
		return nil
	}
	if _, err := mapping(branches, "branches", nil); err != nil {
		return err
	}
	calcBranches, outBranches := documentMap(), documentMap()
	for i := 0; i < len(branches.Content); i += 2 {
		name, branch := branches.Content[i].Value, branches.Content[i+1]
		if branch.Kind != yaml.MappingNode {
			putDocumentField(calcBranches, name, cloneNode(branch))
			continue
		}
		if _, err := mapping(branch, "branches."+name, nil); err != nil {
			return err
		}
		calcBranch, outBranch := documentMap(), documentMap()
		for j := 0; j < len(branch.Content); j += 2 {
			property := branch.Content[j].Value
			destination := calcBranch
			if containsProperty(outputBranchProperties, property) {
				destination = outBranch
			}
			putDocumentField(destination, property, cloneNode(branch.Content[j+1]))
		}
		if len(calcBranch.Content) != 0 {
			putDocumentField(calcBranches, name, calcBranch)
		}
		if len(outBranch.Content) != 0 {
			putDocumentField(outBranches, name, outBranch)
		}
	}
	if len(calcBranches.Content) != 0 {
		putDocumentField(calculation, "branches", calcBranches)
	}
	if len(outBranches.Content) != 0 {
		putDocumentField(output, "branches", outBranches)
	}
	return nil
}

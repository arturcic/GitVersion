package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type fieldRule struct {
	kind     string
	enums    []string
	nullable bool
}

func rootField(name string) fieldRule {
	switch name {
	case "workflow":
		return fieldRule{kind: "workflow", nullable: true}
	case "branches", "ignore", "merge-message-formats":
		return fieldRule{kind: name}
	case "semantic-version-format":
		return enumeration("Strict Loose", false)
	case "strategies":
		return fieldRule{kind: name}
	case "update-build-number":
		return fieldRule{kind: "bool"}
	case "tag-pre-release-weight":
		return fieldRule{kind: "int", nullable: true}
	case "assembly-versioning-scheme", "assembly-file-versioning-scheme":
		return enumeration("Major MajorMinor MajorMinorPatch MajorMinorPatchTag None", true)
	}
	if containsProperty(calculationBranchProperties, name) || containsProperty(outputBranchProperties, name) {
		rule := branchField(name)
		if name == "increment" {
			rule.nullable = false
		}
		return rule
	}
	if containsProperty(calculationProperties, name) || containsProperty(outputProperties, name) {
		return fieldRule{kind: "string", nullable: true}
	}
	return fieldRule{}
}

func branchField(name string) fieldRule {
	switch name {
	case "increment":
		return enumeration("Inherit None Patch Minor Major", false)
	case "mode":
		return enumeration("ManualDeployment ContinuousDelivery ContinuousDeployment", true)
	case "commit-message-incrementing":
		return enumeration("Enabled Disabled MergeMessageOnly", true)
	case "source-branches", "is-source-branch-for":
		return fieldRule{kind: "list"}
	case "prevent-increment":
		return fieldRule{kind: name}
	case "track-merge-message", "track-merge-target", "is-main-branch", "is-release-branch", "tracks-release-branches":
		return fieldRule{kind: "bool", nullable: true}
	case "pre-release-weight":
		return fieldRule{kind: "int", nullable: true}
	case "custom-version-format", "regex", "label":
		return fieldRule{kind: "string", nullable: true}
	}
	return fieldRule{}
}

func ignoreField(name string) fieldRule {
	switch name {
	case "sha", "paths", "branches", "tags":
		return fieldRule{kind: "list"}
	case "commits-before":
		return fieldRule{kind: "date", nullable: true}
	}
	return fieldRule{}
}

func preventField(name string) fieldRule {
	switch name {
	case "of-merged-branch", "when-branch-merged", "when-current-commit-tagged":
		return fieldRule{kind: "bool", nullable: true}
	}
	return fieldRule{}
}

func enumeration(names string, nullable bool) fieldRule {
	return fieldRule{kind: "enum", enums: strings.Fields(names), nullable: nullable}
}

func validateFields(node *yaml.Node, rules func(string) fieldRule, parent, version string) error {
	if _, err := mapping(node, parent, nil); err != nil {
		return validation("PROPERTY", parent, "Expected a mapping with unique property names.")
	}
	for i := 0; i < len(node.Content); i += 2 {
		name, value := node.Content[i].Value, node.Content[i+1]
		path := parent + "." + name
		if parent == "" {
			path = publicPath(name, version, containsProperty(outputProperties, name))
		} else if strings.HasPrefix(parent, "calculation.branches.") && containsProperty(outputBranchProperties, name) {
			path = "output." + strings.TrimPrefix(path, "calculation.")
		}
		if err := validateValue(value, rules(name), name, path, version); err != nil {
			return err
		}
	}
	return nil
}

func validateValue(value *yaml.Node, rule fieldRule, name, path, version string) error {
	fail := func(reason string) error { return validation("PROPERTY", path, reason) }
	if rule.kind == "" {
		return fail("Unknown configuration property.")
	}
	if value.Tag == "!!null" && rule.nullable {
		return nil
	}
	if value.Anchor != "" || value.Kind == yaml.AliasNode {
		return fail("Aliases and anchors are unsupported.")
	}
	if value.Tag == "!!null" {
		return fail("Expected a non-null configuration value.")
	}
	switch rule.kind {
	case "branches", "merge-message-formats":
		if _, err := mapping(value, path, nil); err != nil {
			return fail("Expected a mapping.")
		}
		for i := 0; i < len(value.Content); i += 2 {
			key, item := value.Content[i].Value, value.Content[i+1]
			itemPath := path + "." + key
			if item.Tag == "!!null" {
				return validation("PROPERTY", itemPath, "Expected a non-null mapping value.")
			}
			var err error
			if rule.kind == "branches" {
				err = validateFields(item, branchField, itemPath, version)
			} else {
				err = validateValue(item, fieldRule{kind: "string"}, key, itemPath, version)
			}
			if err != nil {
				return err
			}
		}
	case "ignore":
		return validateFields(value, ignoreField, path, version)
	case "prevent-increment":
		return validateFields(value, preventField, path, version)
	case "strategies":
		names := "None Fallback ConfiguredNextVersion MergeMessage TaggedCommit TrackReleaseBranches VersionInBranchName Mainline"
		if value.Kind == yaml.ScalarNode {
			items := listNode()
			for _, item := range strings.Split(value.Value, ",") {
				items.Content = append(items.Content, stringNode(strings.TrimSpace(item)))
			}
			*value = *items
		}
		if value.Kind != yaml.SequenceNode {
			return fail("Expected supported version strategies.")
		}
		for _, item := range value.Content {
			if err := validateValue(item, enumeration(names, false), name, path, version); err != nil {
				return fail("Expected supported version strategies.")
			}
		}
	case "list":
		if value.Kind != yaml.SequenceNode {
			return fail("Expected a sequence.")
		}
		for i, item := range value.Content {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			if item.Tag == "!!null" {
				return validation("PROPERTY", itemPath, "Expected a non-null sequence item.")
			}
			if err := validateValue(item, fieldRule{kind: "string"}, name, itemPath, version); err != nil {
				return err
			}
		}
	case "string", "workflow":
		if rule.kind == "workflow" && value.Tag != "!!str" {
			return fail("Expected a workflow name.")
		}
		if value.Kind != yaml.ScalarNode {
			return fail("Expected a scalar.")
		}
		stringScalar(value, name)
	case "enum":
		for _, allowed := range rule.enums {
			if value.Tag == "!!str" && strings.EqualFold(allowed, value.Value) {
				*value = *stringNode(allowed)
				return nil
			}
		}
		return fail("Expected a supported enum name.")
	case "bool":
		boolean, err := strconv.ParseBool(strings.ToLower(value.Value))
		if value.Kind != yaml.ScalarNode || err != nil || (!strings.EqualFold(value.Value, "true") && !strings.EqualFold(value.Value, "false")) {
			return fail("Expected a boolean.")
		}
		value.Tag, value.Value, value.Style = "!!bool", strconv.FormatBool(boolean), 0
	case "int":
		number, err := strconv.ParseInt(strings.TrimSpace(value.Value), 10, 32)
		if value.Kind != yaml.ScalarNode || err != nil {
			return fail("Expected a 32-bit integer.")
		}
		value.Tag, value.Value, value.Style = "!!int", strconv.FormatInt(number, 10), 0
	case "date":
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
			if parsed, err := time.Parse(layout, value.Value); err == nil && value.Kind == yaml.ScalarNode {
				*value = *stringNode(parsed.Format("2006-01-02T15:04:05Z"))
				return nil
			}
		}
		return fail("Expected a valid commit cutoff date.")
	}
	return nil
}

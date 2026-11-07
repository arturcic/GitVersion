package config

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// MigrateConfiguration converts a user document to v7 without adding effective
// defaults. Values and branch precedence are preserved. Comments are dropped.
func MigrateConfiguration(data []byte) ([]byte, error) {
	document, err := readConfigurationMap(data)
	if err != nil {
		return nil, err
	}
	if _, err := mapping(document, "configuration document", nil); err != nil {
		return nil, &LayoutError{Cause: err}
	}
	if nodeField(nodeField(document, "output"), "workflow") != nil {
		return nil, migrationLayout("Configuration property 'output.workflow' is not supported. Move 'workflow' to the document root before migrating.")
	}
	calculation := nodeField(document, "calculation")
	if workflow := nodeField(calculation, "workflow"); workflow != nil {
		if nodeField(document, "workflow") != nil {
			return nil, migrationLayout("Configuration property 'workflow' is defined at both the document root and 'calculation.workflow'. Keep only one before migrating.")
		}
		for i := 0; i < len(calculation.Content); i += 2 {
			if calculation.Content[i].Value == "workflow" {
				calculation.Content = append(calculation.Content[:i], calculation.Content[i+2:]...)
				break
			}
		}
		putDocumentField(document, "workflow", workflow)
	}
	fields, _ := mapping(document, "configuration document", nil)
	layout := documentKind(fields)
	if layout == "Mixed" {
		return nil, migrationLayout("The configuration document mixes the v6 flat configuration structure with the v7 'calculation'/'output' structure. Use only one structure before migrating.")
	}
	for _, section := range []string{"calculation", "output"} {
		if node := fields[section]; node != nil && node.Tag != "!!null" {
			if node.Kind != yaml.MappingNode {
				return nil, migrationLayout(fmt.Sprintf("Configuration property '%s' must be a mapping.", section))
			}
			if branch := nodeField(node, "branches"); branch != nil && branch.Kind != yaml.MappingNode {
				return nil, migrationLayout(fmt.Sprintf("Configuration property '%s.branches' must be a mapping.", section))
			}
		}
	}
	flat, err := normalizeDocument(document, "", "configuration document")
	if err != nil {
		return nil, &LayoutError{Cause: err}
	}
	if err := validateFields(flat, rootField, "", "v7"); err != nil {
		return nil, &YAMLError{Cause: err}
	}
	if layout != "V7" {
		document, err = nestDocument(document)
		if err != nil {
			return nil, &LayoutError{Cause: err}
		}
	}
	canonicalMigrationNodes(document)
	return serializeConfigurationNode(document, false)
}

func migrationLayout(message string) error { return &LayoutError{Cause: errors.New(message)} }

func canonicalMigrationNodes(node *yaml.Node) {
	node.HeadComment, node.LineComment, node.FootComment = "", "", ""
	node.Style = 0
	if node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
		if node.Value == "" {
			node.Style = yaml.SingleQuotedStyle
		} else if strings.ContainsAny(node.Value, "{}[]") {
			node.Style = yaml.DoubleQuotedStyle
		}
	}
	for _, child := range node.Content {
		canonicalMigrationNodes(child)
	}
}

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCSharpConfigurationMigration(t *testing.T) {
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct {
				Document *string
				Cases    []struct {
					ID        string
					Arguments []json.RawMessage
				}
			}
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-configuration-migration.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	for method, item := range corpus.Suites["ConfigurationMigrationServiceTests"].Methods {
		t.Run(method, func(t *testing.T) {
			for _, c := range item.Cases {
				t.Run(c.ID, func(t *testing.T) { checkMigrationCase(t, method, pathValue(item.Document), c.Arguments) })
			}
		})
	}
}

func checkMigrationCase(t *testing.T, method, input string, args []json.RawMessage) {
	t.Helper()
	switch method {
	case "MigrationPreservesCommitLabelTemplateUnderCalculationBranches":
		input = "branches:\n  feature:\n    label: '" + arg[string](t, args[0]) + "'"
	case "MigratesWorkflowOnlyConfigurationAtRoot":
		input = "workflow: " + arg[string](t, args[0])
	case "RejectsDuplicateRootAndCalculationWorkflowsEvenWhenEqual":
		input = "workflow: GitHubFlow/v1\ncalculation:\n  workflow: " + arg[string](t, args[0])
	case "RejectsOutputWorkflow", "RejectsInvalidSettings", "RejectsInvalidNestedStructure":
		input = arg[string](t, args[0])
	}
	migrated, err := MigrateConfiguration([]byte(input))
	checkLayout := func(wants ...string) {
		t.Helper()
		var layout *LayoutError
		if !errors.As(err, &layout) || migrated != nil {
			t.Fatalf("want layout error, got %s, %v", migrated, err)
		}
		for _, want := range wants {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("diagnostic %q lacks %q", err, want)
			}
		}
	}
	switch method {
	case "RejectsMixedConfiguration":
		checkLayout()
		return
	case "RejectsDuplicateRootAndCalculationWorkflowsEvenWhenEqual":
		checkLayout("both the document root and 'calculation.workflow'")
		return
	case "RejectsOutputWorkflow":
		checkLayout("'output.workflow'", "document root")
		return
	case "RejectsInvalidNestedStructure":
		checkLayout(arg[string](t, args[1]))
		return
	case "RejectsMalformedYaml", "RejectsInvalidSettings":
		var syntax *YAMLError
		if !errors.As(err, &syntax) || migrated != nil {
			t.Fatalf("want YAML error, got %s, %v", migrated, err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	contains := func(wants ...string) {
		t.Helper()
		for _, want := range wants {
			if !bytes.Contains(migrated, []byte(want)) {
				t.Fatalf("missing %q in %s", want, migrated)
			}
		}
	}
	excludes := func(value string) {
		t.Helper()
		if bytes.Contains(migrated, []byte(value)) {
			t.Fatalf("unexpected %q in %s", value, migrated)
		}
	}
	idempotent := func() {
		t.Helper()
		again, err := MigrateConfiguration(migrated)
		if err != nil || !bytes.Equal(again, migrated) {
			t.Fatalf("migration not idempotent\n%s\n%s\n%v", migrated, again, err)
		}
	}
	switch method {
	case "MigrationPreservesCommitLabelTemplateUnderCalculationBranches":
		document := readTestConfiguration(t, string(migrated))
		if nodeField(document, "calculation") == nil || nodeField(document, "branches") != nil {
			t.Fatal(string(migrated))
		}
		flat, err := normalizeDocument(document, "v7", "configuration")
		if err != nil {
			t.Fatal(err)
		}
		checkField(t, branchNode(flat, "feature"), "label", arg[string](t, args[0]))
		idempotent()
	case "MigratesFlatConfigurationToCalculationAndOutputSections":
		contains("calculation:", "workflow: GitFlow/v1", "  tag-prefix: custom-", "      increment: Major", "output:", "  update-build-number: false", "      pre-release-weight: 42")
		excludes("  workflow:")
	case "AcceptsNestedConfigurationAndProducesDeterministicOutput":
		contains("workflow: GitHubFlow/v1")
		excludes("  workflow:")
		idempotent()
	case "MigratesWorkflowOnlyConfigurationAtRoot":
		contains("workflow: " + arg[string](t, args[0]))
		excludes("  workflow:")
		excludes("tag-prefix:")
		idempotent()
	case "MigratesDraftCalculationWorkflowToRootWithoutChangingOverrides":
		contains("workflow: GitHubFlow/v1", "  tag-prefix: custom-", "  update-build-number: false")
		excludes("  workflow:")
		idempotent()
	case "MigratesConfiguredValuesWithoutAddingDefaults":
		contains("workflow: GitHubFlow/v1", "mode: ContinuousDeployment", "update-build-number: false", "increment: Minor", "pre-release-weight: 42")
		excludes("tag-prefix:")
	default:
		t.Fatal(method)
	}
}

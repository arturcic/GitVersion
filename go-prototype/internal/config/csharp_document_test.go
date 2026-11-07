package config

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCSharpConfigurationDocumentMapper(t *testing.T) {
	var corpus struct {
		Methods map[string][]struct {
			ID        string
			Arguments []any
		}
		Schema map[string][]string
	}
	raw, err := os.ReadFile("../../testdata/csharp-document-mapper.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Methods) != 10 {
		t.Fatal("original declaration inventory changed")
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, c := range cases {
				t.Run(c.ID, func(t *testing.T) {
					checkDocumentMapperCase(t, method, c.Arguments, corpus.Schema)
				})
			}
		})
	}
}

func checkDocumentMapperCase(t *testing.T, method string, args []any, schema map[string][]string) {
	t.Helper()
	type document = map[string]any
	encode := func(value any) *yaml.Node {
		t.Helper()
		var node yaml.Node
		if err := node.Encode(value); err != nil {
			t.Fatal(err)
		}
		return &node
	}
	decode := func(node *yaml.Node) document {
		t.Helper()
		var value document
		if err := node.Decode(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	fields := func(value document) map[string]*yaml.Node {
		t.Helper()
		result, err := mapping(encode(value), "test", nil)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	equal := func(got, want any) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
	expectError := func(err error, messages ...string) {
		t.Helper()
		if err == nil {
			t.Fatal("expected configuration error")
		}
		for _, message := range messages {
			if !strings.Contains(err.Error(), message) {
				t.Fatalf("%q lacks %q", err, message)
			}
		}
	}
	version := func(v7 bool) string {
		if v7 {
			return "v7"
		}
		return "v6"
	}

	flat := document{"tag-prefix": "v"}
	nested := document{"calculation": document{}}
	mixed := document{"tag-prefix": "v", "calculation": document{}}
	switch method {
	case "AssignsEverySerializedPropertyToRootOrExactlyOneV7Section":
		sorted := func(values []string) []string {
			values = slices.Clone(values)
			slices.Sort(values)
			return values
		}
		output := append(slices.Clone(outputProperties), "branches")
		equal(sorted(calculationProperties), schema["calculation"])
		equal(sorted(output), schema["output"])
		equal(sorted(calculationBranchProperties), schema["calculation_branch"])
		equal(sorted(outputBranchProperties), schema["output_branch"])
		equal(sorted(append(append(slices.Clone(calculationProperties), outputProperties...), "workflow")), schema["root"])
		equal(sorted(append(slices.Clone(calculationBranchProperties), outputBranchProperties...)), schema["branch"])
		// Exercise ownership through the mapper for every original field.
		root := document{}
		branch := document{}
		for _, name := range schema["root"] {
			root[name] = name
		}
		for _, name := range schema["branch"] {
			branch[name] = name
		}
		root["branches"] = document{"main": branch}
		result, err := nestDocument(encode(root))
		if err != nil {
			t.Fatal(err)
		}
		sections := decode(result)
		for _, section := range []string{"calculation", "output"} {
			sectionFields, ok := sections[section].(document)
			if !ok {
				t.Fatalf("%s is not a mapping", section)
			}
			equal(len(sectionFields), len(schema[section]))
			for _, name := range schema[section] {
				if name != "branches" {
					equal(sectionFields[name], name)
				}
			}
			branchFields := sectionFields["branches"].(document)["main"].(document)
			equal(len(branchFields), len(schema[section+"_branch"]))
			for _, name := range schema[section+"_branch"] {
				equal(branchFields[name], name)
			}
		}
		equal(sections["workflow"], "workflow")
	case "DetectsEmptyFlatNestedAndMixedDocuments":
		equal(documentKind(fields(document{})), "Empty")
		equal(documentKind(fields(flat)), "V6")
		equal(documentKind(fields(nested)), "V7")
		equal(documentKind(fields(mixed)), "Mixed")
	case "FlattensAndMergesCalculationAndOutputBranches":
		value := document{
			"workflow":    "GitHubFlow/v1",
			"calculation": document{"tag-prefix": "v", "branches": document{"main": document{"increment": "Patch"}}},
			"output": document{"update-build-number": false, "branches": document{
				"main": document{"pre-release-weight": 42}, "develop": document{"custom-version-format": "{SemVer}"},
			}},
		}
		result, err := flattenDocument(fields(value))
		if err != nil {
			t.Fatal(err)
		}
		got := decode(result)
		equal(got["workflow"], "GitHubFlow/v1")
		equal(got["tag-prefix"], "v")
		equal(got["update-build-number"], false)
		branches, ok := got["branches"].(document)
		if !ok {
			t.Fatal("branches must be a mapping")
		}
		main, ok := branches["main"].(document)
		if !ok {
			t.Fatal("main must be a mapping")
		}
		equal(main["increment"], "Patch")
		equal(main["pre-release-weight"], 42)
		if _, ok := branches["develop"]; !ok {
			t.Fatal("develop branch was lost")
		}
	case "RejectsMixedAndSelectedVersionMismatches":
		_, err := normalizeDocument(encode(flat), "v7", "test")
		expectError(err)
		_, err = normalizeDocument(encode(nested), "v6", "test")
		expectError(err)
		_, err = normalizeDocument(encode(mixed), "v7", "test")
		expectError(err)
	case "RejectsPropertiesInTheWrongV7SectionWithReplacement":
		_, err := flattenDocument(fields(document{"calculation": document{"update-build-number": false}}))
		expectError(err, "output.update-build-number")
		_, err = flattenDocument(fields(document{"output": document{"branches": document{"main": document{"increment": "Major"}}}}))
		expectError(err, "calculation.branches.<branch>.increment")
	case "AcceptsEmptyAndWorkflowOnlyDocumentsInEitherVersion":
		value := document{}
		kind := "Empty"
		if args[1].(bool) {
			value["workflow"] = "GitHubFlow/v1"
			kind = "Shared"
		}
		equal(documentKind(fields(value)), kind)
		for _, selected := range []string{version(args[0].(bool)), ""} {
			got, err := normalizeDocument(encode(value), selected, "test")
			if err != nil {
				t.Fatal(err)
			}
			equal(decode(got), value)
		}
	case "RootWorkflowDoesNotDetermineDocumentVersion":
		property := args[0].(string)
		value := document{"workflow": "GitHubFlow/v1", property: document{}}
		kind := "V6"
		if property == "tag-prefix" {
			value[property] = "v"
		}
		if args[1].(bool) {
			kind = "V7"
		}
		equal(documentKind(fields(value)), kind)
	case "RootWorkflowDoesNotMakeMixedDocumentValid":
		mixed["workflow"] = "GitHubFlow/v1"
		equal(documentKind(fields(mixed)), "Mixed")
		_, err := normalizeDocument(encode(mixed), version(args[0].(bool)), "test")
		expectError(err, "unsupported root-level properties")
	case "RejectsNestedWorkflowEvenWhenRootWorkflowExists":
		section := args[0].(string)
		value := document{section: document{"workflow": "GitHubFlow/v1"}}
		if args[1].(bool) {
			value["workflow"] = "GitHubFlow/v1"
		}
		_, err := normalizeDocument(encode(value), "v7", "test")
		expectError(err, section+".workflow", "root", "'workflow'")
	case "BothNestingOverloadsKeepWorkflowAtRoot":
		value := document{"workflow": "GitHubFlow/v1", "tag-prefix": "v", "update-build-number": false}
		typed, err := nestDocument(encode(value))
		if err != nil {
			t.Fatal(err)
		}
		got := decode(typed)
		equal(got["workflow"], "GitHubFlow/v1")
		calculation, ok := got["calculation"].(document)
		if !ok {
			t.Fatal("calculation must be a mapping")
		}
		if _, ok := calculation["workflow"]; ok {
			t.Fatal("workflow must remain at the root")
		}
		objectKeys := map[any]any{}
		for k, v := range value {
			objectKeys[k] = v
		}
		untyped, err := nestDocument(encode(objectKeys))
		if err != nil {
			t.Fatal(err)
		}
		equal(decode(untyped)["workflow"], "GitHubFlow/v1")
		roundTrip, err := normalizeDocument(untyped, "v7", "test")
		if err != nil {
			t.Fatal(err)
		}
		equal(decode(roundTrip), value)
	default:
		t.Fatalf("unhandled original method %q", method)
	}
}

func TestLegacyBranchOutputWeightUsesDocumentMapper(t *testing.T) {
	cfg, err := ParseSelected([]byte("workflow: GitHubFlow/v1\nbranches:\n  main: {increment: Minor, pre-release-weight: 42}\n"), "v6")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Main.Weight != 42 || cfg.MainIncrement.String() != "Minor" {
		t.Fatalf("branch settings were lost: %+v", cfg.Main)
	}
}

func TestLegacyUnknownEmptyBranchIsRejected(t *testing.T) {
	_, err := ParseSelected([]byte("workflow: GitHubFlow/v1\nbranches: {unrecognized: {}}"), "v6")
	if err == nil || !strings.Contains(err.Error(), "branches.unrecognized") {
		t.Fatalf("unsupported branch escaped validation: %v", err)
	}
}

package config

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/formatting"

	"gopkg.in/yaml.v3"
)

func TestCSharpConfigurationQuery(t *testing.T) {
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct {
				Cases []struct {
					ID        string
					Arguments []json.RawMessage
				}
			}
		}
		Approval struct{ YAML string }
	}
	raw, err := os.ReadFile("../../testdata/csharp-configuration-query.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	for suite, data := range corpus.Suites {
		if suite == "ConfigurationQueryIntegrationTests" {
			continue
		}
		t.Run(suite, func(t *testing.T) {
			for method, item := range data.Methods {
				t.Run(method, func(t *testing.T) {
					for _, c := range item.Cases {
						t.Run(c.ID, func(t *testing.T) {
							checkQueryCase(t, method, c.Arguments, corpus.Approval.YAML)
						})
					}
				})
			}
		})
	}
}

func queryProperty(t *testing.T, cfg ResolvedConfiguration, path string) []byte {
	t.Helper()
	got, err := cfg.SerializeProperty(path, "v7")
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func checkQueryError(t *testing.T, cfg ResolvedConfiguration, path, version, want string) {
	t.Helper()
	got, err := cfg.SerializeProperty(path, version)
	if err == nil || got != nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("query %q: got %s, %v; want %q", path, got, err, want)
	}
}

func checkQueryCase(t *testing.T, method string, args []json.RawMessage, approved string) {
	t.Helper()
	empty := ResolvedConfiguration{flat: documentMap()}
	switch method {
	case "PreservesStringTypeAndContents":
		value := arg[string](t, args[0])
		putDocumentField(empty.flat, "custom-version-format", stringNode(value))
		result := queryProperty(t, empty, "output.custom-version-format")
		var got any
		if err := json.Unmarshal(result, &got); err != nil || got != value || bytes.Contains(result, []byte{'\n'}) {
			t.Fatalf("string was changed: %s, %v", result, err)
		}
	case "UsesPublicScalarRepresentationAndInvariantNumbers":
		t.Setenv("LC_ALL", "fr_FR.UTF-8")
		t.Setenv("LANG", "fr_FR.UTF-8")
		cfg := mustResolve(t, `output:
  update-build-number: false
  pre-release-weight: 0
  tag-pre-release-weight: 60000
  assembly-versioning-scheme: MajorMinor
calculation:
  prevent-increment: {of-merged-branch: true}
  ignore: {commits-before: '2026-01-02T03:04:05Z'}
`, "", "v7")
		path, want := arg[string](t, args[0]), arg[string](t, args[1])
		if got := string(queryProperty(t, cfg, path)); got != want {
			t.Fatalf("%s = %s, want %s", path, got, want)
		}
	case "KnownNullIsQueryableEvenWhenOmittedFromFullDisplay":
		path := arg[string](t, args[0])
		if got := string(queryProperty(t, empty, path)); got != "null" {
			t.Fatal(got)
		}
		parts := strings.Split(path, ".")
		if display := serializeResolved(t, empty, "v7"); strings.Contains(display, parts[len(parts)-1]+":") {
			t.Fatal(display)
		}
		checkQueryError(t, empty, path+"-unknown", "v7", "Unknown configuration property")
	case "RejectsUnknownPublicPaths":
		checkQueryError(t, empty, arg[string](t, args[0]), "v7", "Unknown configuration property")
	case "RejectsMalformedPaths":
		checkQueryError(t, empty, arg[string](t, args[0]), "v7", "Invalid configuration property path")
	case "RejectsStructuredValuesAndDynamicTraversal":
		checkQueryError(t, empty, arg[string](t, args[0]), "v7", "only scalar properties of fixed objects")
	case "RejectsV6EvenForAValidV7Path":
		checkQueryError(t, empty, "calculation.tag-prefix", "v6", "require v7 configuration")
	case "NonNullScalarLeavesMatchFullSerializedConfiguration":
		cfg := mustResolve(t, "", "", "v7")
		document := readTestConfiguration(t, serializeResolved(t, cfg, "v7"))
		if checked := checkQueryLeaves(t, cfg, document, ""); checked < 20 {
			t.Fatalf("only checked %d scalar leaves", checked)
		}
	case "SerializesV7CalculationAndOutputSectionsAndRoundTrips":
		cfg := ResolvedConfiguration{flat: readTestConfiguration(t, "tag-prefix: custom-\nupdate-build-number: false\nbranches:\n  main: {increment: Major, pre-release-weight: 42}")}
		display := serializeResolved(t, cfg, "v7")
		for _, want := range []string{"calculation:", "output:", "  tag-prefix: custom-", "    main:"} {
			if !strings.Contains(display, want) {
				t.Fatalf("missing %s in %s", want, display)
			}
		}
		roundTrip, err := normalizeDocument(readTestConfiguration(t, display), "v7", "configuration")
		if err != nil || roundTrip == nil {
			t.Fatalf("round trip: %v", err)
		}
		checkField(t, roundTrip, "tag-prefix", "custom-")
		checkField(t, roundTrip, "update-build-number", false)
		checkField(t, branchNode(roundTrip, "main"), "increment", "Major")
		checkField(t, branchNode(roundTrip, "main"), "pre-release-weight", 42)
	case "SerializesFlatConfigurationWhenV6IsSelected":
		putDocumentField(empty.flat, "tag-prefix", stringNode("custom-"))
		display := serializeResolved(t, empty, "v6")
		if !strings.Contains(display, "tag-prefix: custom-") || strings.Contains(display, "calculation:") || strings.Contains(display, "output:") {
			t.Fatal(display)
		}
	case "Serialize_OrdersConfigurationPropertiesAndPreservesBranchNames":
		cfg := ResolvedConfiguration{flat: readTestConfiguration(t, "update-build-number: true\ntag-prefix: x\nbranches:\n  z-last: {label: z, increment: Minor}\n  a-first: {label: a, increment: Major}")}
		display := serializeResolved(t, cfg, "v7")
		root := readTestConfiguration(t, display)
		if keys := mappingKeys(root); !sort.StringsAreSorted(keys) {
			t.Fatal(keys)
		}
		branches := nodeField(nodeField(root, "calculation"), "branches")
		if keys := mappingKeys(branches); !slices.Equal(keys, []string{"z-last", "a-first"}) {
			t.Fatal(keys)
		}
		props := mappingKeys(nodeField(branches, "z-last"))
		if len(props) == 0 || !sort.StringsAreSorted(props) {
			t.Fatal(props)
		}
	case "CanWriteOutEffectiveConfiguration":
		provider := Provider{Version: "v6"}
		directory := t.TempDir()
		cfg, err := provider.Provide(&directory, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := serializeResolved(t, cfg, "v6"); got != approved {
			t.Fatalf("pinned approval mismatch\nwant:\n%s\ngot:\n%s", approved, got)
		}
	default:
		t.Fatal(method)
	}
}

func readTestConfiguration(t *testing.T, content string) *yaml.Node {
	t.Helper()
	node, err := readConfigurationMap([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	return node
}

func mappingKeys(node *yaml.Node) []string {
	if node == nil {
		return nil
	}
	var keys []string
	for i := 0; i < len(node.Content); i += 2 {
		keys = append(keys, node.Content[i].Value)
	}
	return keys
}

func checkQueryLeaves(t *testing.T, cfg ResolvedConfiguration, document *yaml.Node, prefix string) int {
	t.Helper()
	count := 0
	for i := 0; i < len(document.Content); i += 2 {
		key, node := document.Content[i].Value, document.Content[i+1]
		if key == "branches" || key == "merge-message-formats" {
			continue
		}
		path := prefix + key
		if node.Kind == yaml.MappingNode {
			count += checkQueryLeaves(t, cfg, node, path+".")
		} else if node.Kind == yaml.ScalarNode {
			var want any
			if err := node.Decode(&want); err != nil {
				t.Fatal(err)
			}
			raw := queryProperty(t, cfg, path)
			if text, ok := want.(string); ok {
				if !bytes.Equal(raw, formatting.JSONString(text)) {
					t.Fatalf("%s: %s != %q", path, raw, text)
				}
			} else {
				expected, err := json.Marshal(want)
				if err != nil || !bytes.Equal(raw, expected) {
					t.Fatalf("%s: %s != %s, %v", path, raw, expected, err)
				}
			}
			count++
		}
	}
	return count
}

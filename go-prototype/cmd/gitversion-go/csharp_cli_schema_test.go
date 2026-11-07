package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/output"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

type cliSchemaCase struct {
	ID        string
	Arguments []any
}

type cliSchemaCorpus struct {
	Schema map[string]any
	Suites map[string]struct {
		Methods map[string]struct{ Cases []cliSchemaCase }
	}
}

func loadCLISchemaCorpus(t *testing.T) cliSchemaCorpus {
	t.Helper()
	var corpus cliSchemaCorpus
	if err := parity.ReadJSON("../../testdata/csharp-cli-schema.json", &corpus); err != nil {
		t.Fatal(err)
	}
	return corpus
}

func TestCSharpCLISchema(t *testing.T) {
	corpus := loadCLISchemaCorpus(t)
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "invalid-if-resolved")
	t.Setenv("GITVERSION_GIT_BACKEND", "managed")
	t.Chdir(t.TempDir())
	for _, suiteName := range slices.Sorted(maps.Keys(corpus.Suites)) {
		suite := corpus.Suites[suiteName]
		for _, method := range slices.Sorted(maps.Keys(suite.Methods)) {
			if strings.HasPrefix(method, "Process") {
				continue
			}
			t.Run(method, func(t *testing.T) {
				for _, tc := range suite.Methods[method].Cases {
					t.Run(tc.ID, func(t *testing.T) {
						var stdout, diagnostics bytes.Buffer
						root := compatibilityCLITree()
						switch method {
						case "ExportsBeforeMappingOrAuthentication":
							args := strings.Fields(tc.Arguments[0].(string))
							requested, err := cliSchemaInvocation(args, root)
							if err != nil || !requested {
								t.Fatalf("schema routing = %v, %v", requested, err)
							}
							if err := execute(args, &stdout, &diagnostics); err != nil {
								t.Fatal(err)
							}
							if !strings.HasPrefix(stdout.String(), "{") || !strings.HasSuffix(stdout.String(), "}\n") {
								t.Fatal("expected one schema document")
							}
							parseAndValidateCLISchema(t, corpus, stdout.Bytes())
							assertSchemaNoSideEffects(t, diagnostics.String())
						case "PreservesUnmatchedAndPositionalDiagnostics":
							err := execute(strings.Fields(tc.Arguments[0].(string)), &stdout, &diagnostics)
							if err == nil || !strings.Contains(err.Error(), "Could not parse command line parameter") ||
								stdout.Len() != 0 || diagnostics.Len() != 0 {
								t.Fatalf("expected only a syntax error, got %v and %q / %q", err, &stdout, &diagnostics)
							}
						case "TokenConsumedAsStringValueDoesNotRequestSchema":
							args := []string{"config", "migrate", "--output", "--cli-schema"}
							requested, err := cliSchemaInvocation(args, root)
							if err != nil || requested {
								t.Fatalf("schema routing = %v, %v", requested, err)
							}
							options, migration, err := migrationInvocation(args)
							if err != nil || !migration || options.output != "--cli-schema" {
								t.Fatalf("migration routing = %v, %q, %v", migration, options.output, err)
							}
						case "ReportsExpectedOutputFailure":
							err := execute([]string{"--cli-schema"}, closedSchemaOutput{}, &diagnostics)
							if err == nil || err.Error() != "Could not export the CLI schema: output closed" {
								t.Fatalf("unexpected write error %v", err)
							}
						case "HostSkipsExecutorsAndFeatureLogging":
							if err := execute([]string{"--cli-schema"}, &stdout, &diagnostics); err != nil {
								t.Fatal(err)
							}
							parseAndValidateCLISchema(t, corpus, stdout.Bytes())
							assertSchemaNoSideEffects(t, diagnostics.String())
						case "ExportsLiveTreeIncludingHiddenSymbolsAndLocalOptions":
							root = schemaCommand("ignored", "Root description")
							root.Arguments = []cliSchemaArgument{schemaArgument("path", "Path description", 1, schemaUnbounded, true, false)}
							format := schemaArgument("format", "", 0, schemaUnbounded, false, false).
								withChoices([]string{"BuildServer", "Json", "File", "DotEnv"}, true, true)
							root.Options = []cliSchemaOption{schemaOption("--output", "Output description", []string{"-z", "-o"}, &format, true)}
							root.Options[0].Required, root.Options[0].Recursive, root.Options[0].Hidden = true, true, true
							child := schemaCommand("child", "Child description")
							child.Aliases, child.Hidden = []string{"alias"}, true
							child.Options = []cliSchemaOption{schemaOption("--output", "", nil, schemaArgumentPointer(schemaArgument("output", "", 1, 1, false, false)), false)}
							root.Commands = []cliSchemaCommand{child}
							encoded := exportSchemaForTest(t, corpus, root)
							c := encoded.Command
							arg, option, sub := c.Arguments[0], c.Options[0], c.Commands[0]
							if c.Name != "gitversion" || c.Description != "Root description" || !arg.Hidden || !arg.Required ||
								arg.Arity.Minimum != 1 || arg.Arity.Maximum != nil || !slices.Equal(option.Aliases, []string{"-o", "-z"}) ||
								!option.Required || !option.Recursive || !option.Hidden {
								t.Fatalf("incorrect root metadata: %+v", c)
							}
							actual := option.Arguments[0]
							if actual.Name != "format" || !slices.Equal(actual.AcceptedValues, []string{"BuildServer", "Json", "File", "DotEnv"}) ||
								!reflect.DeepEqual(actual.Metadata, []cliSchemaMetadata{{"gitversion.caseInsensitiveValues", true},
									{"gitversion.acceptedValuesExhaustive", false}, {"gitversion.acceptsNumericValues", true}}) {
								t.Fatalf("incorrect output choice metadata: %+v", actual)
							}
							if sub.Name != "child" || !slices.Equal(sub.Aliases, []string{"alias"}) || !sub.Hidden || len(sub.Options) != 1 {
								t.Fatalf("incorrect child command: %+v", sub)
							}
							childArg := sub.Options[0].Arguments[0]
							if sub.Options[0].Name != "--output" || childArg.Arity.Maximum == nil || *childArg.Arity.Maximum != 1 || childArg.AcceptedValues != nil {
								t.Fatalf("incorrect child output argument: %+v", childArg)
							}
						case "DoesNotRunDefaultsValidatorsParsersOrCompletionSources":
							root := schemaCommand("", "")
							root.Arguments = []cliSchemaArgument{schemaArgument("path", "", 1, 1, false, true)}
							if err := writeCLISchema(&stdout, root); err != nil {
								t.Fatal(err)
							}
							d := parseAndValidateCLISchema(t, corpus, stdout.Bytes())
							if d.Command.Arguments[0].Required || strings.Contains(stdout.String(), "default invoked") {
								t.Fatal("export evaluated the positional default")
							}
						case "OutputIsStableAcrossCulturesAndRepeatedExports":
							root := schemaCommand("", "")
							root.Options = []cliSchemaOption{
								schemaOption("--toggle", "", nil, booleanSchemaArgument("toggle"), false),
								schemaOption("--flag", "", nil, nil, false),
							}
							t.Setenv("LC_ALL", "en_US.UTF-8")
							if err := writeCLISchema(&stdout, root); err != nil {
								t.Fatal(err)
							}
							t.Setenv("LC_ALL", "tr_TR.UTF-8")
							for range 2 {
								var actual bytes.Buffer
								if err := writeCLISchema(&actual, root); err != nil || !bytes.Equal(actual.Bytes(), stdout.Bytes()) {
									t.Fatalf("unstable export: %v", err)
								}
							}
							d := parseAndValidateCLISchema(t, corpus, stdout.Bytes())
							toggle := d.Command.Options[0].Arguments[0]
							if len(d.Command.Options[1].Arguments) != 0 || toggle.Arity.Minimum != 0 || toggle.Arity.Maximum == nil ||
								*toggle.Arity.Maximum != 1 || !slices.Equal(toggle.AcceptedValues, []string{"true", "false"}) ||
								d.Info.Version != cliInformationalVersion() {
								t.Fatalf("incorrect boolean or version metadata: %+v %+v", toggle, d.Info)
							}
						case "PreservesFiniteAndUnboundedArgumentArity":
							minimum, maximum := int(tc.Arguments[0].(float64)), int(tc.Arguments[1].(float64))
							root := schemaCommand("", "")
							root.Arguments = []cliSchemaArgument{schemaArgument("values", "", minimum, maximum, false, false)}
							a := exportSchemaForTest(t, corpus, root).Command.Arguments[0]
							if a.Required != (minimum > 0) || a.Arity.Minimum != minimum {
								t.Fatalf("incorrect required/minimum: %+v", a)
							}
							if maximum == schemaUnbounded {
								if a.Arity.Maximum != nil {
									t.Fatal("unbounded maximum should be absent")
								}
							} else if a.Arity.Maximum == nil || *a.Arity.Maximum != maximum {
								t.Fatalf("incorrect finite maximum: %+v", a.Arity)
							}
						default:
							t.Fatalf("missing assertion body for %s", method)
						}
					})
				}
			})
		}
	}
}

type closedSchemaOutput struct{}

func (closedSchemaOutput) Write([]byte) (int, error) { return 0, errors.New("output closed") }

func assertSchemaNoSideEffects(t *testing.T, diagnostics string) {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil || len(entries) != 0 || diagnostics != "" {
		t.Fatalf("unexpected files or diagnostics: %v, %v, %q", entries, err, diagnostics)
	}
}

func exportSchemaForTest(t *testing.T, corpus cliSchemaCorpus, root cliSchemaCommand) cliSchemaDocument {
	t.Helper()
	var stdout bytes.Buffer
	if err := writeCLISchema(&stdout, root); err != nil {
		t.Fatal(err)
	}
	return parseAndValidateCLISchema(t, corpus, stdout.Bytes())
}

func parseAndValidateCLISchema(t *testing.T, corpus cliSchemaCorpus, data []byte) cliSchemaDocument {
	t.Helper()
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	validateSchemaNode(t, corpus.Schema, corpus.Schema, value, "$")
	var document cliSchemaDocument
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

// The pinned schema uses structural assertions, local references, unique items,
// and numeric bounds on arities. Validate them against the serialized document.
func validateSchemaNode(t *testing.T, root, node map[string]any, value any, path string) {
	t.Helper()
	if ref, ok := node["$ref"].(string); ok {
		name, local := strings.CutPrefix(ref, "#/$defs/")
		if !local {
			t.Fatalf("unsupported schema reference %q", ref)
		}
		validateSchemaNode(t, root, root["$defs"].(map[string]any)[name].(map[string]any), value, path)
	}
	switch node["type"] {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("%s: expected object, got %T", path, value)
		}
		required, _ := node["required"].([]any)
		for _, key := range required {
			if _, ok := object[key.(string)]; !ok {
				t.Fatalf("%s: missing %s", path, key)
			}
		}
		properties, _ := node["properties"].(map[string]any)
		for key, child := range properties {
			if actual, present := object[key]; present {
				validateSchemaNode(t, root, child.(map[string]any), actual, path+"."+key)
			}
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			t.Fatalf("%s: expected array, got %T", path, value)
		}
		for index, item := range items {
			if child, exists := node["items"].(map[string]any); exists {
				validateSchemaNode(t, root, child, item, path+"["+strconv.Itoa(index)+"]")
			}
			if node["uniqueItems"] == true && slices.ContainsFunc(items[:index], func(prior any) bool { return reflect.DeepEqual(prior, item) }) {
				t.Fatalf("%s: duplicate array item %v", path, item)
			}
		}
	case "integer":
		number, ok := value.(float64)
		if !ok || number != float64(int(number)) {
			t.Fatalf("%s: expected integer, got %v", path, value)
		}
		if min, ok := node["minimum"].(float64); ok && number < min {
			t.Fatalf("%s: %v below minimum %v", path, number, min)
		}
		if max, ok := node["maximum"].(float64); ok && number > max {
			t.Fatalf("%s: %v above maximum %v", path, number, max)
		}
	case "string":
		if _, ok := value.(string); !ok {
			t.Fatalf("%s: expected string, got %T", path, value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			t.Fatalf("%s: expected boolean, got %T", path, value)
		}
	case nil:
	default:
		t.Fatalf("%s: unsupported schema type %v", path, node["type"])
	}
}

func assertCLISchemaCommand(t *testing.T, d cliSchemaDocument) {
	t.Helper()
	find := func(c cliSchemaCommand, name string) cliSchemaOption {
		t.Helper()
		i := slices.IndexFunc(c.Options, func(o cliSchemaOption) bool { return o.Name == name })
		if i < 0 {
			t.Fatalf("%s: missing option %s", c.Name, name)
		}
		return c.Options[i]
	}
	c := d.Command
	schema, formats := find(c, "--cli-schema"), find(c, "--output").Arguments[0]
	if d.Opencli != "0.1" || !schema.Recursive || len(schema.Arguments) != 0 ||
		!slices.Equal(formats.AcceptedValues, []string{"BuildServer", "Json", "File", "DotEnv"}) || formats.Arity.Maximum != nil {
		t.Fatalf("unexpected root schema: %+v / %+v / %+v", d.Info, schema, formats)
	}
	config := c.Commands[0]
	migrate := config.Commands[0]
	validationIndex := slices.IndexFunc(config.Commands, func(command cliSchemaCommand) bool { return command.Name == "validate" })
	if validationIndex < 0 {
		t.Fatal("missing config validate command")
	}
	validate := config.Commands[validationIndex]
	if migrate.Name != "migrate" || !slices.Equal(find(validate, "--format").Arguments[0].AcceptedValues, []string{"json"}) ||
		find(validate, "--config").Name != "--config" || find(migrate, "--output").Arguments[0].AcceptedValues != nil ||
		!slices.Equal(find(c, "--show-variable").Arguments[0].AcceptedValues, output.VariableNames) ||
		!slices.Equal(find(c, "--verbosity").Arguments[0].AcceptedValues, []string{"Quiet", "Minimal", "Normal", "Verbose", "Diagnostic"}) {
		t.Fatal("unexpected config or root option metadata")
	}
}

func requireSchemaDirectory(t *testing.T, directory string, expected ...string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var actual []string
	for _, entry := range entries {
		actual = append(actual, entry.Name())
	}
	if !slices.Equal(actual, expected) {
		t.Fatalf("%s contains %v, want %v", filepath.Base(directory), actual, expected)
	}
}

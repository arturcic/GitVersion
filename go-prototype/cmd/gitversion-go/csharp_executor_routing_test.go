package main

import (
	"bytes"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

type executorRoutingCase struct {
	ID        string
	Arguments []any
}

type executorRoutingCorpus struct {
	VersionVariables map[string]any `json:"version_variables"`
	Suites           map[string]struct {
		Methods map[string]struct{ Cases []executorRoutingCase }
	}
}

func loadExecutorRoutingCorpus(t *testing.T) executorRoutingCorpus {
	t.Helper()
	var corpus executorRoutingCorpus
	if err := parity.ReadJSON("../../testdata/csharp-executor-routing.json", &corpus); err != nil {
		t.Fatal(err)
	}
	return corpus
}

func TestCSharpExecutorRouting(t *testing.T) {
	corpus := loadExecutorRoutingCorpus(t)
	methods := corpus.Suites["GitVersionExecutorRoutingTests"].Methods
	for _, method := range slices.Sorted(maps.Keys(methods)) {
		t.Run(method, func(t *testing.T) {
			for _, tc := range methods[method].Cases {
				t.Run(tc.ID, func(t *testing.T) {
					switch method {
					case "DisplayUsesVersionSpecificOverridesWithoutCalculating",
						"PropertyQueryUsesScalarSerializerAndSkipsDynamicRepositoryVerification",
						"LegacyPropertyQueryStillThrowsBeforeLoadingConfiguration":
						testConfigurationRoute(t, method, tc)
					case "NormalExecutionDiscoversCalculatesAndWritesOutputsInOrder", "ExpectedCalculationFailureReturnsOneAndWritesStderr":
						testCalculationRoute(t, method, tc, corpus.VersionVariables)
					default:
						t.Fatal("missing routing assertions for", method)
					}
				})
			}
		})
	}
}

type recordingConfigurationView struct {
	version, property         string
	displayCalls, scalarCalls int
}

func (v *recordingConfigurationView) Serialize(version string) ([]byte, error) {
	v.displayCalls++
	v.version = version
	return []byte("serialized configuration"), nil
}

func (v *recordingConfigurationView) SerializeProperty(property, version string) ([]byte, error) {
	v.scalarCalls++
	v.property, v.version = property, version
	return []byte("true"), nil
}

type recordingRoutingOutput struct {
	bytes.Buffer
	calls int
}

func (w *recordingRoutingOutput) Write(data []byte) (int, error) {
	w.calls++
	return w.Buffer.Write(data)
}

func testConfigurationRoute(t *testing.T, method string, tc executorRoutingCase) {
	t.Helper()
	const directory, project = "working", "project"
	property := "output.update-build-number"
	var stdout recordingRoutingOutput
	view := &recordingConfigurationView{}
	projectCalls, providerCalls := 0, 0
	version, remoteURL := "v7", ""
	var selectedProperty *string
	var overrides []byte
	switch method {
	case "DisplayUsesVersionSpecificOverridesWithoutCalculating":
		version = tc.Arguments[0].(string)
		overrides = []byte("tag-prefix: custom-")
	case "PropertyQueryUsesScalarSerializerAndSkipsDynamicRepositoryVerification":
		selectedProperty, remoteURL = &property, "https://example.invalid/repository"
	case "LegacyPropertyQueryStillThrowsBeforeLoadingConfiguration":
		version, selectedProperty = "v6", &property
	}
	findProject := func(working string) *string {
		projectCalls++
		if working != directory {
			t.Fatalf("project lookup from %q, want %q", working, directory)
		}
		root := project
		return &root
	}
	provide := func(working, root *string, runtime []byte) (configurationView, error) {
		providerCalls++
		if working == nil || *working != directory {
			t.Fatalf("provider did not receive working directory: %v", working)
		}
		if remoteURL != "" {
			if root != nil {
				t.Fatal("remote query attempted repository config verification")
			}
		} else if root == nil || *root != project {
			t.Fatalf("provider did not receive project root for verification: %v", root)
		}
		want := overrides
		if version == "v6" {
			want = nil
		}
		if !bytes.Equal(runtime, want) {
			t.Fatalf("%s provider overrides = %q, want %q", version, runtime, want)
		}
		return view, nil
	}
	err := writeConfigurationQuery(version, directory, remoteURL, selectedProperty, overrides, findProject, provide, &stdout)
	switch method {
	case "DisplayUsesVersionSpecificOverridesWithoutCalculating":
		if err != nil || projectCalls != 1 || providerCalls != 1 || stdout.calls != 1 ||
			stdout.String() != "serialized configuration\n" || view.displayCalls != 1 ||
			view.scalarCalls != 0 || view.property != "" || view.version != version {
			t.Fatalf("display route got %v, %d project lookups, %d provider calls, %d writes %q, serializer %+v",
				err, projectCalls, providerCalls, stdout.calls, stdout.String(), view)
		}
	case "PropertyQueryUsesScalarSerializerAndSkipsDynamicRepositoryVerification":
		if err != nil || projectCalls != 0 || providerCalls != 1 || stdout.calls != 1 ||
			stdout.String() != "true\n" || view.displayCalls != 0 || view.scalarCalls != 1 ||
			view.property != property || view.version != version {
			t.Fatalf("scalar route got %v, %d project lookups, %d provider calls, %d writes %q, serializer %+v",
				err, projectCalls, providerCalls, stdout.calls, stdout.String(), view)
		}
	case "LegacyPropertyQueryStillThrowsBeforeLoadingConfiguration":
		const expected = "Configuration property queries require v7 configuration. Set GITVERSION_CONFIGURATION_VERSION=v7."
		if err == nil || err.Error() != expected || projectCalls != 0 || providerCalls != 0 ||
			stdout.calls != 0 || view.displayCalls != 0 || view.scalarCalls != 0 {
			t.Fatalf("legacy rejection got %v, %d project lookups, %d provider calls, %d writes, serializer %+v",
				err, projectCalls, providerCalls, stdout.calls, view)
		}
	}
}

func testCalculationRoute(t *testing.T, method string, tc executorRoutingCase, values map[string]any) {
	t.Helper()
	const directory, project = "repository", "project"
	var calls []string
	var failure error
	if method == "ExpectedCalculationFailureReturnsOneAndWritesStderr" {
		message := "invalid repository"
		if tc.Arguments[0].(bool) {
			message = "invalid configuration"
		}
		failure = errors.New(message)
	}
	resolved := false
	checkValues := func(got map[string]any) {
		t.Helper()
		if reflect.ValueOf(got).Pointer() != reflect.ValueOf(values).Pointer() {
			t.Fatal("output stage received a different set of version variables")
		}
	}
	err := runVersionStages(directory, versionStages{
		discover: func(path string) (string, error) {
			calls = append(calls, "discover")
			if path != directory {
				t.Fatalf("discovery from %q, want %q", path, directory)
			}
			return project, nil
		},
		calculate: func(root string) (calculationExecution, error) {
			calls = append(calls, "calculate")
			if root != project {
				t.Fatalf("calculation root = %q, want %q", root, project)
			}
			if failure != nil {
				return calculationExecution{}, failure
			}
			resolved = true
			return calculationExecution{values: values, updateBuildNumber: tc.Arguments[0].(bool)}, nil
		},
		output: func(result calculationExecution) error {
			calls = append(calls, "output")
			checkValues(result.values)
			if !resolved || result.updateBuildNumber != tc.Arguments[0].(bool) {
				t.Fatalf("output resolved = %v, update build number = %v", resolved, result.updateBuildNumber)
			}
			return nil
		},
		assembly: func(got map[string]any) error {
			calls = append(calls, "assembly")
			checkValues(got)
			return nil
		},
		wix: func(got map[string]any) error {
			calls = append(calls, "wix")
			checkValues(got)
			return nil
		},
	})
	if failure != nil {
		if !errors.Is(err, failure) || resolved || !slices.Equal(calls, []string{"discover", "calculate"}) {
			t.Fatalf("failed calculation got %v, resolved %v, calls %v", err, resolved, calls)
		}
		var stderr bytes.Buffer
		writeCommandDiagnostic(&stderr, err)
		if stderr.String() != "An error occurred:\n"+failure.Error()+"\n" {
			t.Fatalf("unexpected calculation diagnostic %q", stderr.String())
		}
		return
	}
	if err != nil || !resolved || !slices.Equal(calls, []string{"discover", "calculate", "output", "assembly", "wix"}) {
		t.Fatalf("normal calculation got %v, resolved %v, calls %v", err, resolved, calls)
	}
}

func TestConfigGetRejectsAnEmptyProperty(t *testing.T) {
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	var stdout, stderr bytes.Buffer
	err := execute([]string{t.TempDir(), "config", "get", ""}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "property path") || stdout.Len() != 0 {
		t.Fatalf("empty property returned %v, stdout %q, stderr %q", err, &stdout, &stderr)
	}
}

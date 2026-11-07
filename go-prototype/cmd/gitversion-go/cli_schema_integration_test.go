//go:build platform

package main

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformCSharpCLISchema(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	corpus := loadCLISchemaCorpus(t)
	baseDir := filepath.Join(root, ".local/baseline")
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	files, err := parity.FileSet(baseDir)
	if err != nil || !reflect.DeepEqual(files, baseline.Files) {
		t.Fatal("baseline file set mismatch", err)
	}
	specHash, err := parity.FileHash(filepath.Join(root, "baseline.json"))
	if err != nil || specHash != baseline.SpecSHA256 {
		t.Fatal("baseline specification mismatch", err)
	}
	candidate := os.Getenv("PARITY_CANDIDATE")
	candidateHash, err := parity.FileHash(candidate)
	if err != nil {
		t.Fatal(err)
	}
	report, err := os.MkdirTemp(os.Getenv("PARITY_EVIDENCE_DIR"), "cli-schema-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", report)
	type result struct {
		ExitCode       int
		Stdout, Stderr string
	}
	methods := corpus.Suites["CliSchemaTests"].Methods
	comparisons := 0
	for _, method := range slices.Sorted(maps.Keys(methods)) {
		if !strings.HasPrefix(method, "Process") {
			continue
		}
		t.Run(method, func(t *testing.T) {
			for _, test := range methods[method].Cases {
				if method == "ProcessRejectsInvalidSyntaxAndLegacyParser" && test.Arguments[1] == "v6" {
					continue
				}
				t.Run(test.ID, func(t *testing.T) {
					parser, configuration, backend := "v7", "v7", "managed"
					switch method {
					case "ProcessExportsSameDocumentWithoutRepositoryOrFiles":
						configuration, backend = test.Arguments[0].(string), test.Arguments[1].(string)
					case "ProcessRejectsInvalidSyntaxAndLegacyParser":
						parser = test.Arguments[1].(string)
					}
					caseDir := filepath.Join(report, method, test.ID)
					schemaByImplementation := map[string][]byte{}
					for _, implementation := range []string{"csharp", "go"} {
						runDir := filepath.Join(caseDir, implementation)
						work := filepath.Join(runDir, "work")
						if err := os.MkdirAll(work, 0755); err != nil {
							t.Fatal(err)
						}
						env := maps.Clone(baseline.Spec.Environment)
						env["GITVERSION_ARGUMENT_PARSER_VERSION"] = parser
						env["GITVERSION_CONFIGURATION_VERSION"] = configuration
						env["GITVERSION_GIT_BACKEND"] = backend
						run := func(label, arguments string) result {
							t.Helper()
							exe := candidate
							args := strings.Fields(arguments)
							if implementation == "csharp" {
								exe = os.Getenv("PARITY_DOTNET")
								args = append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, args...)
							}
							ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
							defer cancel()
							cmd := exec.CommandContext(ctx, exe, args...)
							cmd.Dir, cmd.Env = work, parity.Environment(runDir, env)
							var stdout, stderr bytes.Buffer
							cmd.Stdout, cmd.Stderr = &stdout, &stderr
							exit := 0
							if err := cmd.Run(); err != nil {
								var status *exec.ExitError
								if !errors.As(err, &status) || ctx.Err() != nil {
									t.Fatal(err)
								}
								exit = status.ExitCode()
							}
							got := result{exit, stdout.String(), stderr.String()}
							if err := parity.WriteJSON(filepath.Join(runDir, label+".json"), got); err != nil {
								t.Fatal(err)
							}
							return got
						}
						if method == "ProcessRejectsInvalidSyntaxAndLegacyParser" {
							got := run("invalid", test.Arguments[0].(string))
							if got.ExitCode == 0 || got.Stdout != "" || !strings.Contains(got.Stderr, "Could not parse command line parameter") {
								t.Fatalf("%s accepted invalid schema invocation: %+v", implementation, got)
							}
							requireSchemaDirectory(t, work)
							continue
						}
						expected := run("root", "--cli-schema")
						if expected.ExitCode != 0 || expected.Stderr != "" || !strings.HasSuffix(expected.Stdout, "\n") {
							t.Fatalf("%s root schema failed: %d, %q", implementation, expected.ExitCode, expected.Stderr)
						}
						schemaByImplementation[implementation] = []byte(expected.Stdout)
						document := parseAndValidateCLISchema(t, corpus, []byte(expected.Stdout))
						assertCLISchemaCommand(t, document)
						compareOutput := func(label, arguments string) {
							t.Helper()
							got := run(label, arguments)
							if got != expected {
								t.Fatalf("%s %s changed output: exit %d, stderr %q, stdout equal %v",
									implementation, label, got.ExitCode, got.Stderr, got.Stdout == expected.Stdout)
							}
						}
						switch method {
						case "ProcessExportsSameDocumentWithoutRepositoryOrFiles":
							compareOutput("config", "config --cli-schema")
							compareOutput("migrate", "config migrate --config missing.yml --in-place --output sentinel.yml --force --cli-schema")
							compareOutput("output", "--config missing.yml --output file --output-file sentinel.json --log-file sentinel.log --cli-schema")
							requireSchemaDirectory(t, work)
						case "ProcessEmitsOnlySchemaWhenCombinedWithHelpOrVersion":
							compareOutput("help-or-version", test.Arguments[0].(string))
							requireSchemaDirectory(t, work)
						case "ProcessIgnoresBuildServerOutputAndPreservesExistingConfiguration":
							const config = "mode: ContinuousDelivery\n"
							configPath := filepath.Join(work, "GitVersion.yml")
							if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
								t.Fatal(err)
							}
							env["GITHUB_ACTIONS"] = "true"
							env["GITHUB_OUTPUT"] = filepath.Join(work, "github-output")
							env["GITHUB_ENV"] = filepath.Join(work, "github-env")
							compareOutput("buildserver", "--config GitVersion.yml --output buildserver --log-file console --cli-schema")
							compareOutput("migrate", "config migrate --config GitVersion.yml --in-place --cli-schema")
							contents, err := os.ReadFile(configPath)
							if err != nil || string(contents) != config {
								t.Fatalf("%s changed configuration: %q, %v", implementation, contents, err)
							}
							requireSchemaDirectory(t, work, "GitVersion.yml")
						default:
							t.Fatal("missing process assertions for", method)
						}
					}
					if method != "ProcessRejectsInvalidSyntaxAndLegacyParser" {
						ref, err := parity.Object(schemaByImplementation["csharp"])
						if err != nil {
							t.Fatal(err)
						}
						actual, err := parity.Object(schemaByImplementation["go"])
						if err != nil {
							t.Fatal(err)
						}
						goInfo, csharpInfo := actual["info"].(map[string]any), ref["info"].(map[string]any)
						if goInfo["version"] != informationalVersion || !strings.HasSuffix(csharpInfo["version"].(string), baseline.Spec.Revision) {
							t.Fatalf("incorrect binary version metadata: %v / %v", goInfo, csharpInfo)
						}
						goInfo["version"] = csharpInfo["version"]
						if !reflect.DeepEqual(actual, ref) {
							t.Fatalf("OpenCLI structure differs. Inspect %s", caseDir)
						}
					}
					comparisons++
				})
			}
		})
	}
	if !t.Failed() {
		if err := parity.WriteJSON(filepath.Join(report, "report.json"), map[string]any{
			"comparisons": comparisons, "baseline_revision": baseline.Spec.Revision,
			"candidate_sha256": candidateHash, "baseline_files": files, "spec_sha256": specHash,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

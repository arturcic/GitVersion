package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

type cliExecutionCase struct {
	ID, Contains, Approved string
	Arguments              []any
}

func cliExecutionCases(t *testing.T, suite string) map[string]struct{ Cases []cliExecutionCase } {
	t.Helper()
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct{ Cases []cliExecutionCase }
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-cli-execution.json", &corpus); err != nil {
		t.Fatal(err)
	}
	return corpus.Suites[suite].Methods
}

func makeCLIHistory(t *testing.T, dir, tag string, two bool) []fixtures.Checkpoint {
	t.Helper()
	steps := []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: tag}, {Op: "commit", ID: "after"},
		{Op: "checkpoint", ID: "requested"}}
	if two {
		steps = append(steps, fixtures.Step{Op: "commit", ID: "latest"}, fixtures.Step{Op: "checkpoint", ID: "head"})
	}
	cps, err := fixtures.Generate([]fixtures.Recipe{{ID: "cli-execution", Steps: steps}}, filepath.Join(dir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	return cps
}

func TestCSharpCLIExecution(t *testing.T) {
	for method, item := range cliExecutionCases(t, "ExecCmdLineArgumentTest") {
		t.Run(method, func(t *testing.T) {
			for _, tc := range item.Cases {
				if (method == "CheckBuildServerVerbosityConsole" || method == "VerboseVerbosityIncludesDebugOutput" || method == "CommitArgumentUsesRequestedCommitWhenCacheExists") && tc.Arguments[0].(bool) {
					continue
				}
				t.Run(tc.ID, func(t *testing.T) {
					t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
					t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
					working := t.TempDir()
					var args []string
					wantFailure, forbidden := false, ""
					expected := tc.Contains
					switch method {
					case "InvalidArgumentsExitCodeShouldNotBeZero":
						if _, err := git.PlainInit(working, false); err != nil {
							t.Fatal(err)
						}
						args, wantFailure = []string{working, "--invalid-argument"}, true
					case "WorkingDirectoryWithoutGitFolderFailsWithInformativeMessage":
						t.Chdir(working)
						wantFailure = true
					case "WorkingDirectoryWithoutCommitsFailsWithInformativeMessage":
						if _, err := git.PlainInit(working, false); err != nil {
							t.Fatal(err)
						}
						args, wantFailure = []string{working, "--log-file", "console"}, true
					case "WorkingDirectoryDoesNotExistFailsWithInformativeMessage":
						target := filepath.Join(working, "missing")
						args, wantFailure = []string{"--target-path", target}, true
						expected = strings.ReplaceAll(expected, "{workingDirectory}", target)
					case "WorkingDirectoryWithoutGitFolderDoesNotFailForVersionAndHelp":
						t.Chdir(working)
						args = strings.Fields(tc.Arguments[0].(string))
					case "LogPathContainsForwardSlash":
						repo := makeCLIHistory(t, working, "1.2.3", false)[0].Path
						args = []string{repo, "--log-file", filepath.Join(working, "gitversion.log")}
						expected = `"MajorMinorPatch": "1.2.4"`
					case "CheckBuildServerVerbosityConsole", "VerboseVerbosityIncludesDebugOutput":
						repo := makeCLIHistory(t, working, "1.2.3", false)[0].Path
						t.Chdir(repo)
						args = []string{"--output", "buildserver", "--log-file", "console"}
						if method == "CheckBuildServerVerbosityConsole" {
							args = append(args, strings.Fields(tc.Arguments[1].(string))...)
							if !tc.Arguments[2].(bool) {
								forbidden, expected = expected, ""
							}
						} else {
							args = append(args, "--verbosity", "verbose", "--no-cache")
						}
					case "CommitArgumentUsesRequestedCommitWhenCacheExists":
						testCLICommitSelection(t, working)
						return
					default:
						t.Fatalf("unknown pinned method %q", method)
					}
					var stdout, stderr bytes.Buffer
					err := execute(args, &stdout, &stderr)
					if (err != nil) != wantFailure {
						t.Fatalf("CLI error = %v, want failure %v", err, wantFailure)
					}
					message := stdout.String() + stderr.String()
					if err != nil {
						message += err.Error()
					}
					if message == "" || !strings.Contains(message, expected) || (forbidden != "" && strings.Contains(message, forbidden)) {
						t.Fatalf("CLI output %q, expected %q, forbidden %q", message, expected, forbidden)
					}
					if method == "LogPathContainsForwardSlash" {
						log, err := os.ReadFile(filepath.Join(working, "gitversion.log"))
						if err != nil || !bytes.Contains(log, []byte("INFO")) {
							t.Fatalf("calculation log %q, %v", log, err)
						}
					}
				})
			}
		})
	}
}

func testCLICommitSelection(t *testing.T, dir string) {
	t.Helper()
	checkpoints := makeCLIHistory(t, dir, "1.0.0", true)
	requested, head := checkpoints[0], checkpoints[1]
	show, commit := "--show-variable", "--commit"
	t.Chdir(head.Path)
	for _, item := range []struct {
		args []string
		want string
	}{
		{[]string{show, "Sha"}, head.Head},
		{[]string{commit, requested.Head, show, "Sha"}, requested.Head},
	} {
		var stdout, stderr bytes.Buffer
		args := item.args
		if err := execute(args, &stdout, &stderr); err != nil || strings.TrimSpace(stdout.String()) != item.want {
			t.Fatalf("CLI %v returned %q, want %s, error %v", args, &stdout, item.want, err)
		}
	}
}

func TestCSharpValidationJSON(t *testing.T) {
	for method, item := range cliExecutionCases(t, "ConfigurationValidationJsonTests") {
		t.Run(method, func(t *testing.T) {
			for _, tc := range item.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					result := validationEnvelope{SchemaVersion: 1, Diagnostics: []validationDiagnostic{}}
					v7, path := "v7", "/repo/candidate.yml"
					line, column := 12, 34
					switch method {
					case "ValidationEnvelopePreservesFieldNamesNullsAndDiagnosticDetails":
						result.Diagnostics = []validationDiagnostic{{Code: "GVCONFIG_FILE", Severity: "error", Message: "file \"missing\"\n"}}
					case "SuccessfulValidationPreservesEmptyDiagnostics":
						result.Valid, result.Version, result.File = true, &v7, &path
					case "YamlDiagnosticPreservesNumericLocationAndEscapedUnicodePath":
						path = "/repo/config é \"quoted\".yml"
						result.Version, result.File = &v7, &path
						result.Diagnostics = []validationDiagnostic{{Code: "GVCONFIG_YAML", Severity: "error", Path: "calculation", Message: "Invalid YAML.", Line: &line, Column: &column}}
					default:
						t.Fatalf("unknown pinned method %q", method)
					}
					actual, err := marshalValidationEnvelope(result)
					if err != nil || string(actual) != tc.Approved {
						t.Fatalf("validation JSON %s\nwant %s\nerror %v", actual, tc.Approved, err)
					}
				})
			}
		})
	}
}

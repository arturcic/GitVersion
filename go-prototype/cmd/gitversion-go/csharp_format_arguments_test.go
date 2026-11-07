package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

type formatArgumentCase struct {
	ID        string
	Arguments []string
}

func formatArgumentCases(t *testing.T) map[string][]formatArgumentCase {
	t.Helper()
	var corpus struct {
		Methods map[string][]formatArgumentCase
	}
	if err := parity.ReadJSON("../../testdata/csharp-format-arguments.json", &corpus); err != nil {
		t.Fatal(err)
	}
	return corpus.Methods
}

func formatArgumentRepositories(t *testing.T, dest string) map[bool]string {
	t.Helper()
	repos, err := fixtures.Generate([]fixtures.Recipe{
		{ID: "minimal", Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "head"}}},
		{ID: "feature", Steps: []fixtures.Step{
			{Op: "commit", ID: "root"},
			{Op: "branch", Name: "develop"},
			{Op: "checkout", Name: "develop"},
			{Op: "commit", ID: "second"},
			{Op: "tag", Name: "1.0.0"},
			{Op: "branch", Name: "feature/foo"},
			{Op: "checkout", Name: "feature/foo"},
			{Op: "commit", ID: "feature"},
			{Op: "checkpoint", ID: "head"},
		}},
	}, dest)
	if err != nil {
		t.Fatal(err)
	}
	return map[bool]string{true: repos[0].Path, false: repos[1].Path}
}

func TestCSharpFormatArguments(t *testing.T) {
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
	t.Setenv("CustomVar", "foo")
	repos := formatArgumentRepositories(t, filepath.Join(t.TempDir(), "fixtures"))
	for method, cases := range formatArgumentCases(t) {
		t.Run(method, func(t *testing.T) {
			repo := repos[strings.HasSuffix(method, "EvenForMinimalRepositories")]
			run := func(args ...string) string {
				t.Helper()
				var out, diagnostics bytes.Buffer
				if err := execute(append([]string{repo}, args...), &out, &diagnostics); err != nil || diagnostics.Len() != 0 {
					t.Fatalf("CLI %v: %v, stderr %q", args, err, diagnostics.String())
				}
				return out.String()
			}
			for _, tc := range cases {
				t.Run(tc.ID, func(t *testing.T) {
					switch method {
					case "ShouldOutputFormatTests", "ShouldOutputFormatWithEnvironmentVariablesTests":
						got := run("--output", "json", "--format", tc.Arguments[0])
						got = strings.NewReplacer("\r", "", "\n", "").Replace(got)
						if got != tc.Arguments[1] {
							t.Fatalf("formatted value = %q, want %q", got, tc.Arguments[1])
						}
					case "ShouldOutputDotEnvEntries", "ShouldOutputAllDotEnvEntriesEvenForMinimalRepositories":
						got := run("--output", "dotenv")
						want := "GitVersion_" + tc.Arguments[0] + "=" + tc.Arguments[1] + "\n"
						if !strings.Contains(got, want) {
							t.Fatalf("missing %q in %q", want, got)
						}
					case "ShouldOutputAllCalculatedVariablesAsDotEnvEntries", "ShouldOutputAllCalculatedVariablesAsDotEnvEntriesEvenForMinimalRepositories":
						var variables map[string]json.RawMessage
						if err := json.Unmarshal([]byte(run("--output", "json")), &variables); err != nil {
							t.Fatal(err)
						}
						got := strings.Count(run("--output", "dotenv"), "\n")
						if got != len(variables) {
							t.Fatalf("dotenv has %d lines, calculated %d variables", got, len(variables))
						}
					default:
						t.Fatalf("unknown source method %q", method)
					}
				})
			}
		})
	}
}

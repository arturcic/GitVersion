//go:build platform

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformArgumentParserParity(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(filepath.Join(root, ".local"), "argument-parser-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	baseDir := filepath.Join(root, ".local/baseline")
	files, err := parity.FileSet(baseDir)
	if err != nil || !reflect.DeepEqual(files, baseline.Files) {
		t.Fatal("baseline file set mismatch", err)
	}
	spec, err := parity.FileHash(filepath.Join(root, "baseline.json"))
	if err != nil || spec != baseline.SpecSHA256 {
		t.Fatal("baseline specification mismatch", err)
	}
	candidate := os.Getenv("PARITY_CANDIDATE")
	hash, err := parity.FileHash(candidate)
	if err != nil {
		t.Fatal(err)
	}
	cps, err := fixtures.Generate([]fixtures.Recipe{{
		ID: "argument-parser", Config: "workflow: GitHubFlow/v1\n",
		Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}, {Op: "commit", ID: "after"}, {Op: "checkpoint", ID: "head"}},
	}}, filepath.Join(dir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	type testCase struct {
		name, parser, format, outputFile, errorText string
		arguments                                   []string
	}
	cases := []testCase{
		{name: "v7-no-args", format: "json"},
		{name: "v7-positional", format: "json", arguments: []string{"{repo}", "--no-fetch", "--no-cache", "--no-normalize"}},
		{name: "v7-target", format: "json", arguments: []string{"--target-path", "{repo}"}},
		{name: "v7-output-alias", format: "json", arguments: []string{"-o", "json", "-o", "json"}},
		{name: "v7-scalar", format: "text", arguments: []string{"-v", "FullSemVer"}},
		{name: "v7-format", format: "text", arguments: []string{"-f", "{Major:00}.{Minor}.{Patch}"}},
		{name: "v7-output-file", outputFile: "GitVersion.json", arguments: []string{"--output", "file"}},
		{name: "v7-json-and-file", format: "json", outputFile: "version.json", arguments: []string{"--output", "json", "file", "--output-file", "version.json"}},
		{name: "v7-override", format: "json", arguments: []string{"--override-config", "calculation.next-version=2.0.0"}},
		{name: "v7-relative-config", format: "json", arguments: []string{"{repo}", "-c", "GitVersion.yml"}},
		{name: "v7-bad-output", errorText: "invalid_value", arguments: []string{"--output", "invalid_value"}},
		{name: "v7-missing-config", errorText: "Could not find config file", arguments: []string{"--config", "missing.yml"}},
		{name: "v6-positional", parser: "v6", format: "json", arguments: []string{"{repo}", "-nofetch", "-nocache", "-nonormalize"}},
		{name: "v6-scalar", parser: "v6", format: "text", arguments: []string{"-showvariable", "FullSemVer"}},
		{name: "v6-format", parser: "v6", format: "text", arguments: []string{"-format", "{Major}.{Minor}.{Patch}"}},
		{name: "v6-output-file", parser: "v6", outputFile: "version.json", arguments: []string{"-output", "file", "-outputfile", "version.json"}},
		{name: "v6-override", parser: "v6", format: "json", arguments: []string{"/overrideconfig", "calculation.next-version=2.0.0"}},
		{name: "v6-bad-output", parser: "v6", errorText: "Value 'invalid_value' cannot be parsed as output type", arguments: []string{"-output", "invalid_value"}},
	}
	passed := 0
	for _, tc := range cases {
		if t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{}
			for key, value := range baseline.Spec.Environment {
				env[key] = value
			}
			if tc.parser != "" {
				env["GITVERSION_ARGUMENT_PARSER_VERSION"] = tc.parser
			}
			commands := []parity.Command{
				{Executable: os.Getenv("PARITY_DOTNET"), Arguments: append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, tc.arguments...)},
				{Executable: candidate, Arguments: slices.Clone(tc.arguments)},
			}
			run := func(command parity.Command, suffix string) (parity.Result, []byte) {
				t.Helper()
				path := filepath.Join(dir, tc.name+"-"+suffix)
				got, err := parity.RunInWorkingDirectory(t.Context(), command, cps[0].Path, path, env, 30*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				if err := saveResult(path, got); err != nil {
					t.Fatal(err)
				}
				var file []byte
				if tc.outputFile != "" {
					file, err = os.ReadFile(filepath.Join(path, "repo", tc.outputFile))
					if err != nil {
						t.Fatalf("missing output: %v (%s)", err, got.Stderr)
					}
				}
				return got, file
			}
			ref, refFile := run(commands[0], "csharp")
			got, gotFile := run(commands[1], "go")
			if tc.errorText != "" {
				for _, r := range []parity.Result{ref, got} {
					if r.ExitCode != 1 || len(r.Stdout) != 0 || !strings.Contains(string(r.Stderr), tc.errorText) {
						t.Fatalf("expected %q, got %d %q %q", tc.errorText, r.ExitCode, r.Stdout, r.Stderr)
					}
				}
				return
			}
			if ref.ExitCode != 0 || got.ExitCode != 0 {
				t.Fatalf("reference %d %s, Go %d %s", ref.ExitCode, ref.Stderr, got.ExitCode, got.Stderr)
			}
			checkJSON := func(expected, actual []byte) {
				t.Helper()
				diff, err := parity.Compare(expected, actual)
				if err != nil || len(diff) != 0 {
					t.Fatalf("JSON differs: %v %+v", err, diff)
				}
			}
			if tc.format == "json" {
				checkJSON(ref.Stdout, got.Stdout)
			} else if !bytes.Equal(ref.Stdout, got.Stdout) {
				t.Fatalf("stdout differs: %q != %q", ref.Stdout, got.Stdout)
			}
			if tc.outputFile != "" {
				checkJSON(refFile, gotFile)
			}
		}) {
			passed++
		}
	}
	if err := parity.WriteJSON(filepath.Join(dir, "report.json"), map[string]any{
		"passed": passed, "failed": len(cases) - passed, "candidate_sha256": hash, "baseline_revision": baseline.Spec.Revision, "spec_sha256": spec,
	}); err != nil {
		t.Fatal(err)
	}
}

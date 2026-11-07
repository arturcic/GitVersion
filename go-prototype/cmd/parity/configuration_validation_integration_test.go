//go:build platform

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/commandline"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformConfigurationValidationParity(t *testing.T) {
	testPlatformConfigurationValidation(t, false)
}

func TestPlatformCSharpValidationInvocationParity(t *testing.T) {
	testPlatformConfigurationValidation(t, true)
}

type validationParityCase struct {
	name, document, version, override string
	code                              int
	missing                           bool
}

func testPlatformConfigurationValidation(t *testing.T, originalInvocations bool) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(filepath.Join(root, ".local"), "configuration-validation-")
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
	cases := []validationParityCase{
		{"empty", "", "v7", "", 0, false},
		{"nested", "calculation: {tag-prefix: 'release-'}\noutput: {update-build-number: false}", "v7", "", 0, false},
		{"trunk", "workflow: TrunkBased/preview1", "v7", "", 0, false},
		{"legacy", "next-version: 2\nsemantic-version-format: Loose", "v6", "", 0, false},
		{"layout", "calculation: {next-version: 2.0.0}", "v6", "", 1, false},
		{"unknown-property", "calculation: {unknown-key: private-value}", "v7", "", 1, false},
		{"output-property", "output: {branches: {main: {pre-release-weight: wrong}}}", "v7", "", 1, false},
		{"invalid-regex", "calculation: {tag-prefix: '['}", "v7", "", 1, false},
		{"invalid-version", "calculation: {next-version: 2.1}", "v7", "", 1, false},
		{"unknown-branch", "calculation: {branches: {main: {source-branches: [missing]}}}", "v7", "", 1, false},
		{"unknown-workflow", "workflow: private-value", "v7", "", 1, false},
		{"missing", "", "v7", "", 1, true},
		{"override-workflow", "{}", "v7", "workflow=GitHubFlow/v1", 0, false},
		{"override-strategies", "{}", "v7", "calculation.strategies=Mainline", 0, false},
		{"override-invalid-regex", "{}", "v7", "calculation.tag-prefix=[", 1, false},
		{"override-invalid-value", "{}", "v7", "calculation.increment=bad", 1, false},
		{"override-invalid-path", "{}", "v7", "unknown=private-value", 1, false},
	}
	prefix := map[string][]string{}
	localFormat := map[string]string{}
	if originalInvocations {
		var corpus struct {
			Methods map[string]json.RawMessage
		}
		if err := parity.ReadJSON(filepath.Join(root, "testdata/csharp-configuration-validation.json"), &corpus); err != nil {
			t.Fatal(err)
		}
		var invocations []struct {
			ID        string
			Arguments []string
		}
		if err := json.Unmarshal(corpus.Methods["IncompatibleRootOptionsAreReportedWithoutSideEffects"], &invocations); err != nil {
			t.Fatal(err)
		}
		cases = nil
		for _, tc := range invocations {
			name := "root-option-" + tc.ID
			cases = append(cases, validationParityCase{name: name, document: "{}", version: "v7", code: 1})
			prefix[name] = commandline.SplitUnquoted(tc.Arguments[0], ' ')
		}
		if len(cases) != 12 {
			t.Fatal("original root option inventory changed")
		}
		cases = append(cases, validationParityCase{name: "invalid-local-format", document: "{}", version: "v7", code: 1})
		localFormat["invalid-local-format"] = "xml"
	}
	type observation struct {
		Stdout, Stderr string
		ExitCode       int
	}
	passed := []string{}
	for _, tc := range cases {
		if t.Run(tc.name, func(t *testing.T) {
			workdir := t.TempDir()
			file := filepath.Join(workdir, "config café.yml")
			if !tc.missing {
				if err := os.WriteFile(file, []byte(tc.document), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := validationSnapshot(workdir)
			if err != nil {
				t.Fatal(err)
			}
			format := "json"
			if value := localFormat[tc.name]; value != "" {
				format = value
			}
			args := append(prefix[tc.name], "config", "validate", "--config", file, "--format", format)
			if tc.override != "" {
				args = append(args, "--override-config", tc.override)
			}
			run := func(executable string, argv []string) observation {
				t.Helper()
				command := exec.CommandContext(t.Context(), executable, argv...)
				command.Dir = workdir
				command.Env = command.Environ()
				for key, value := range baseline.Spec.Environment {
					command.Env = append(command.Env, key+"="+value)
				}
				command.Env = append(command.Env, "GITVERSION_CONFIGURATION_VERSION="+tc.version)
				var stdout, stderr bytes.Buffer
				command.Stdout, command.Stderr = &stdout, &stderr
				err := command.Run()
				if err != nil && command.ProcessState == nil {
					t.Fatal(err)
				}
				return observation{stdout.String(), stderr.String(), command.ProcessState.ExitCode()}
			}
			reference := run(os.Getenv("PARITY_DOTNET"), append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, args...))
			actual := run(candidate, args)
			if err := parity.WriteJSON(filepath.Join(dir, tc.name+".json"), map[string]observation{"reference": reference, "go": actual}); err != nil {
				t.Fatal(err)
			}
			if reference.ExitCode != tc.code || actual.ExitCode != reference.ExitCode {
				t.Fatalf("exit mismatch: reference %+v, Go %+v", reference, actual)
			}
			if format == "json" {
				diff, err := parity.Compare([]byte(reference.Stdout), []byte(actual.Stdout))
				if err != nil || len(diff) > 0 {
					t.Fatalf("validation JSON differs: %v %+v", err, diff)
				}
			} else if reference.Stdout != "" || actual.Stdout != "" {
				t.Fatalf("expected empty stdout: reference %+v, Go %+v", reference, actual)
			}
			if reference.Stderr != actual.Stderr {
				t.Fatalf("diagnostics differ:\n%s\n%s", reference.Stderr, actual.Stderr)
			}
			after, err := validationSnapshot(workdir)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("validation modified inputs", err)
			}
		}) {
			passed = append(passed, tc.name)
		}
	}
	report := map[string]any{"passed": len(passed), "failed": len(cases) - len(passed), "cases": passed, "candidate_sha256": hash, "baseline_revision": baseline.Spec.Revision, "spec_sha256": spec}
	if err := parity.WriteJSON(filepath.Join(dir, "report.json"), report); err != nil {
		t.Fatal(fmt.Errorf("save validation report: %w", err))
	}
}

func validationSnapshot(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return map[string]string{}, nil
	}
	files, err := parity.FileSet(dir)
	if err != nil {
		return nil, err
	}
	snapshot := make(map[string]string, len(files)+len(entries))
	for _, entry := range entries {
		snapshot["entry:"+entry.Name()] = entry.Type().String()
	}
	for name, digest := range files {
		snapshot["file:"+name] = digest
	}
	return snapshot, nil
}

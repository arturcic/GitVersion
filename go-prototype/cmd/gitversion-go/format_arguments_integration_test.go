//go:build platform

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformFormatArgumentParity(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	baseDir := filepath.Join(root, ".local/baseline")
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
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
	dir, err := os.MkdirTemp(os.Getenv("PARITY_EVIDENCE_DIR"), "format-arguments-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)
	repos := formatArgumentRepositories(t, filepath.Join(dir, "fixtures"))
	env := map[string]string{"CustomVar": "foo"}
	for key, value := range baseline.Spec.Environment {
		env[key] = value
	}

	compare := func(t *testing.T, repo, output, format string) {
		t.Helper()
		args := []string{"{repo}", "--output", output, "--no-cache", "--no-fetch", "--no-normalize"}
		if format != "" {
			args = append(args, "--format", format)
		}
		run := func(name, executable string, arguments []string) []byte {
			t.Helper()
			path := filepath.Join(dir, filepath.FromSlash(t.Name()), name)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			result, err := parity.Run(t.Context(), parity.Command{Executable: executable, Arguments: arguments}, repo, path, env, 30*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if err := parity.WriteJSON(filepath.Join(path, "process.json"), result); err != nil {
				t.Fatal(err)
			}
			for name, data := range map[string][]byte{"stdout": result.Stdout, "stderr": result.Stderr} {
				if err := os.WriteFile(filepath.Join(path, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if result.Error != "" || result.ExitCode != 0 || len(result.Stderr) != 0 {
				t.Fatalf("%s exit %d: %s, stderr %q", name, result.ExitCode, result.Error, result.Stderr)
			}
			return result.Stdout
		}
		ref := run("csharp", os.Getenv("PARITY_DOTNET"), append([]string{
			"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll"),
		}, args...))
		actual := run("go", candidate, args)
		if output == "json" && format == "" {
			differences, err := parity.Compare(ref, actual)
			if err != nil || len(differences) != 0 {
				t.Fatalf("JSON difference: %v, %v", differences, err)
			}
		} else if !bytes.Equal(ref, actual) {
			t.Fatalf("C# stdout %q differs from Go stdout %q", ref, actual)
		}
	}

	corpus := formatArgumentCases(t)
	for _, method := range []string{"ShouldOutputFormatTests", "ShouldOutputFormatWithEnvironmentVariablesTests"} {
		t.Run(method, func(t *testing.T) {
			for _, tc := range corpus[method] {
				t.Run(tc.ID, func(t *testing.T) { compare(t, repos[false], "json", tc.Arguments[0]) })
			}
		})
	}
	for name, minimal := range map[string]bool{"feature": false, "minimal": true} {
		t.Run(name, func(t *testing.T) {
			for _, mode := range []string{"json", "dotenv"} {
				t.Run(mode, func(t *testing.T) { compare(t, repos[minimal], mode, "") })
			}
		})
	}
	if !t.Failed() {
		if err := parity.WriteJSON(filepath.Join(dir, "report.json"), map[string]any{
			"comparisons": 11, "baseline_revision": baseline.Spec.Revision, "candidate_sha256": hash,
			"baseline_files": files, "spec_sha256": spec,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

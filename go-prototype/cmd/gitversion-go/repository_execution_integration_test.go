//go:build platform

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformRepositoryExecutionParity(t *testing.T) {
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
	dir, err := os.MkdirTemp(os.Getenv("PARITY_EVIDENCE_DIR"), "repository-execution-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)

	cases := []string{"worktree-tagged-head", "teamcity-json-only", "teamcity-json-buildserver-and-file"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			type result struct {
				Stdout, Stderr []byte
				File           []byte
			}
			run := func(implementation, executable string, prefix []string) result {
				t.Helper()
				path := filepath.Join(dir, name, implementation)
				repository := filepath.Join(path, "repository")
				if err := os.MkdirAll(repository, 0755); err != nil {
					t.Fatal(err)
				}
				var cwd string
				env := map[string]string{}
				for k, v := range baseline.Spec.Environment {
					env[k] = v
				}
				args := []string{"--no-fetch", "--no-cache", "--no-normalize", "--output", "json", "--verbosity", "quiet"}
				if name == "worktree-tagged-head" {
					cwd, _ = supportWorktreeFixture(t, repository)
				} else {
					cwd = teamCityRepository(t, repository)
					env["TEAMCITY_VERSION"] = "8.0.0"
					if name == "teamcity-json-buildserver-and-file" {
						args = append(args, "--output", "buildserver", "--output", "file", "--output-file", "version.json")
					}
				}
				home := filepath.Join(path, "home")
				if err := os.Mkdir(home, 0700); err != nil {
					t.Fatal(err)
				}
				before, err := fixtures.Digest(repository)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				command := exec.CommandContext(ctx, executable, append(prefix, args...)...)
				command.Dir, command.Env = cwd, parity.Environment(home, env)
				var stdout, stderr bytes.Buffer
				command.Stdout, command.Stderr = &stdout, &stderr
				if err := command.Run(); err != nil || stderr.Len() != 0 {
					t.Fatalf("%s: %v, stderr %q", implementation, err, &stderr)
				}
				res := result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
				if name == "teamcity-json-buildserver-and-file" {
					file := filepath.Join(cwd, "version.json")
					res.File, err = os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.Remove(file); err != nil {
						t.Fatal(err)
					}
				}
				after, err := fixtures.Digest(repository)
				if err != nil || after != before {
					t.Fatalf("%s changed the fixture beyond its expected output: %v", implementation, err)
				}
				for filename, data := range map[string][]byte{"stdout": res.Stdout, "stderr": res.Stderr, "version.json": res.File} {
					if err := os.WriteFile(filepath.Join(path, filename), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				return res
			}
			ref := run("csharp", os.Getenv("PARITY_DOTNET"), []string{
				"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll"),
			})
			got := run("go", candidate, nil)
			if name == "teamcity-json-buildserver-and-file" {
				refAt, gotAt := bytes.Index(ref.Stdout, []byte("{\n")), bytes.Index(got.Stdout, []byte("{\n"))
				if refAt < 0 || gotAt < 0 || !bytes.Equal(ref.Stdout[:refAt], got.Stdout[:gotAt]) {
					t.Fatalf("TeamCity protocol differs\nC# %s\nGo %s", ref.Stdout, got.Stdout)
				}
				ref.Stdout, got.Stdout = ref.Stdout[refAt:], got.Stdout[gotAt:]
				compareRepositoryExecutionJSON(t, ref.File, got.File)
			}
			compareRepositoryExecutionJSON(t, ref.Stdout, got.Stdout)
			if name == "worktree-tagged-head" {
				checkVersionJSON(t, got.Stdout, "1.0.0")
			} else {
				checkVersionJSON(t, got.Stdout, "0.0.1-5")
			}
		})
	}
	if !t.Failed() {
		if err := parity.WriteJSON(filepath.Join(dir, "report.json"), map[string]any{
			"comparisons": len(cases), "baseline_revision": baseline.Spec.Revision, "candidate_sha256": hash,
			"baseline_files": files, "spec_sha256": spec,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func compareRepositoryExecutionJSON(t *testing.T, expected, actual []byte) {
	t.Helper()
	diff, err := parity.Compare(expected, actual)
	if err != nil || len(diff) != 0 {
		t.Fatalf("version JSON difference: %v, error %v", diff, err)
	}
}

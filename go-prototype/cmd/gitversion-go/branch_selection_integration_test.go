//go:build platform

package main

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformBranchSelectionParity(t *testing.T) {
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
	dir, err := os.MkdirTemp(os.Getenv("PARITY_EVIDENCE_DIR"), "branch-selection-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)
	source := branchSelectionHistory(t, dir)
	detached := filepath.Join(dir, "detached")
	if err := fixtures.Copy(source.path, detached); err != nil {
		t.Fatal(err)
	}
	repository, err := git.PlainOpen(detached)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, plumbing.NewHash(source.head))); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, upper, alias, target, commit, branch, sha string
		detached, invalid                               bool
	}{
		{name: "explicit", upper: "bad name", target: "feature/work", branch: "feature/work", sha: source.target},
		{name: "remote-tracking", target: "refs/remotes/origin/feature/tracking", branch: "origin/feature/tracking", sha: source.target},
		{name: "missing-target", target: "feature/missing", branch: "main", sha: source.head},
		{name: "divergent-ref-context", upper: "feature/work", alias: "feature/ignored", branch: "feature/work", sha: source.head},
		{name: "detached-alias", upper: " \t", alias: "refs/remotes/origin/feature/alias", branch: "feature/alias", sha: source.head, detached: true},
		{name: "detached-explicit", target: "feature/work", branch: "feature/work", sha: source.target, detached: true},
		{name: "context-explicit-commit", upper: "refs/heads/feature/work", commit: source.base, branch: "feature/work", sha: source.base},
		{name: "invalid-context", upper: "refs/tags/1.0.0", invalid: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			caseDir := filepath.Join(dir, test.name)
			if err := os.Mkdir(caseDir, 0755); err != nil {
				t.Fatal(err)
			}
			env := maps.Clone(baseline.Spec.Environment)
			env["GIT_BRANCH"], env["Git_Branch"] = test.upper, test.alias
			path := source.path
			if test.detached {
				path = detached
			}
			args := []string{"--no-cache", "--no-fetch", "--no-normalize", "--output", "json", "--verbosity", "quiet"}
			targetFlag := "--branch"
			if test.target != "" {
				args = append(args, targetFlag, test.target)
			}
			if test.commit != "" {
				args = append(args, "--commit", test.commit)
			}
			run := func(name, exe string, argv []string) parity.Result {
				t.Helper()
				workspace := filepath.Join(caseDir, name)
				result, err := parity.RunInWorkingDirectory(t.Context(), parity.Command{Executable: exe, Arguments: argv}, path, workspace, env, 30*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				if err := parity.WriteJSON(filepath.Join(workspace, "process.json"), result); err != nil {
					t.Fatal(err)
				}
				for name, data := range map[string][]byte{"stdout": result.Stdout, "stderr": result.Stderr} {
					if err := os.WriteFile(filepath.Join(workspace, name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if result.Before != result.After {
					t.Fatalf("%s changed its repository", name)
				}
				if test.invalid {
					if result.ExitCode != 1 || len(result.Stdout) != 0 || !strings.Contains(string(result.Stderr), "Invalid branch name") {
						t.Fatalf("%s accepted invalid context: %d / %s / %s", name, result.ExitCode, result.Stdout, result.Stderr)
					}
				} else {
					values, err := parity.Object(result.Stdout)
					if result.ExitCode != 0 || err != nil || values["Sha"] != test.sha || values["BranchName"] != test.branch {
						t.Fatalf("%s expected %s at %s, exit %d, output %s, stderr %s", name, test.branch, test.sha, result.ExitCode, result.Stdout, result.Stderr)
					}
				}
				return result
			}
			ref := run("csharp", os.Getenv("PARITY_DOTNET"), append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, args...))
			got := run("go", candidate, args)
			if !test.invalid {
				if diff, err := parity.Compare(ref.Stdout, got.Stdout); err != nil || len(diff) != 0 {
					t.Fatalf("JSON differs: %v, %v", diff, err)
				}
			}
		})
	}
	if !t.Failed() {
		if err := parity.WriteJSON(filepath.Join(dir, "report.json"), map[string]any{"comparisons": len(cases), "baseline_revision": baseline.Spec.Revision, "candidate_sha256": hash, "baseline_files": files, "spec_sha256": spec}); err != nil {
			t.Fatal(err)
		}
	}
}

//go:build platform

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformRepositoryCompatibilityParity(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	baseDir := filepath.Join(root, ".local/baseline")
	files, err := parity.FileSet(baseDir)
	if err != nil || !reflect.DeepEqual(files, baseline.Files) {
		t.Fatal("baseline changed", err)
	}
	spec, err := parity.FileHash(filepath.Join(root, "baseline.json"))
	if err != nil || spec != baseline.SpecSHA256 {
		t.Fatal("baseline specification changed", err)
	}
	candidate := os.Getenv("PARITY_CANDIDATE")
	candidateHash, err := parity.FileHash(candidate)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(filepath.Join(root, ".local"), "repository-compatibility-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)
	var recipes []fixtures.Recipe
	for _, workflow := range []string{"GitHubFlow/v1", "GitFlow/v1"} {
		for _, branch := range []string{"main", "codex/topic"} {
			for _, tag := range []string{"1.2.3+build.1025", "1.2.3-alpha.4+1106.build.1025"} {
				steps := []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: tag}}
				if branch != "main" {
					steps = append(steps, fixtures.Step{Op: "branch", Name: branch}, fixtures.Step{Op: "checkout", Name: branch})
				}
				steps = append(steps, fixtures.Step{Op: "checkpoint", ID: "tagged"}, fixtures.Step{Op: "commit", ID: "after"}, fixtures.Step{Op: "checkpoint", ID: "after"})
				recipes = append(recipes, fixtures.Recipe{ID: fmt.Sprintf("repository-%02d", len(recipes)), Description: workflow + " " + branch + " " + tag, Config: "workflow: " + workflow + "\ncalculation:\n  mode: ManualDeployment\n  branches:\n    main: {label: alpha}\n", Steps: steps})
			}
		}
	}
	recipes = append(recipes, fixtures.Recipe{ID: "issue-number-is-not-version", Description: "Historical hotfix issue number must not become major version", Config: "workflow: GitFlow/v1\ncalculation:\n  mode: ManualDeployment\n  next-version: 7.0.0\n  branches:\n    main: {label: alpha}\n", Steps: []fixtures.Step{
		{Op: "commit", ID: "root"}, {Op: "tag", Name: "6.0.0"}, {Op: "branch", Name: "hotfix/4795-reads-tag-from-other-branch"}, {Op: "checkout", Name: "hotfix/4795-reads-tag-from-other-branch"}, {Op: "commit", ID: "work"}, {Op: "checkout", Name: "main"}, {Op: "merge", ID: "merge", From: "hotfix/4795-reads-tag-from-other-branch", Message: "Merge pull request #4796 from HHobeck/hotfix/4795-reads-tag-from-other-branch"}, {Op: "checkpoint", ID: "merged"},
	}})

	if err := parity.WriteJSON(filepath.Join(dir, "recipes.json"), recipes); err != nil {
		t.Fatal(err)
	}
	cps, err := fixtures.Generate(recipes, filepath.Join(dir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	referenceArgs := []string{}
	for i := 0; i < len(baseline.Spec.Arguments); i++ {
		if baseline.Spec.Arguments[i] == "--config" {
			i++
			continue
		}
		referenceArgs = append(referenceArgs, baseline.Spec.Arguments[i])
	}
	commands := []parity.Command{{Executable: os.Getenv("PARITY_DOTNET"), Arguments: append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, referenceArgs...)}, {Executable: candidate, Arguments: []string{"calculate", "--repository", "{repo}"}}}
	passed := 0
	for i, cp := range cps {
		t.Run(cp.ID, func(t *testing.T) {
			write := func(rel, content string) {
				t.Helper()
				p := filepath.Join(cp.Path, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(filepath.Join(cp.Path, ".git/config"))
			if err != nil {
				t.Fatal(err)
			}
			enabled := "true"
			if i%4 == 3 {
				enabled = "false"
			}
			write(".git/config", string(raw)+"\n[extensions]\n worktreeConfig = "+enabled+"\n")
			if i%4 == 1 {
				write(".git/config.worktree", "[core]\n bare = true\n")
			}
			if i%4 == 2 || i%4 == 3 {
				write(".git/config.worktree", "invalid content ignored by pinned common-config reader\n")
			}
			write(".git/info/exclude", "[Oo]bj\nartifacts\n!build/artifacts\n")
			write("build/artifacts/obj/output.bin", "ignored build output\n")
			write(".git/refs/codex/private", cp.Head+"\n")
			write(".git/refs/remotes/origin/main", cp.Head+"\n")
			write(".git/refs/remotes/origin/HEAD", "ref: refs/remotes/origin/main\n")
			var reference []byte
			for j, command := range commands {
				output := filepath.Join(dir, fmt.Sprintf("%s-%d", cp.ID, j))
				result, err := parity.Run(t.Context(), command, cp.Path, output, baseline.Spec.Environment, 60*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				if err := saveResult(output, result); err != nil {
					t.Fatal(err)
				}
				if err := result.Validate(); err != nil {
					t.Fatalf("command %d: %v", j, err)
				}
				if j == 0 {
					reference = result.Stdout
				} else {
					diff, err := parity.Compare(reference, result.Stdout)
					if err != nil || len(diff) > 0 {
						t.Fatalf("JSON differs: %v %+v", err, diff)
					}
				}
			}
			passed++
		})
	}
	if err := parity.WriteJSON(filepath.Join(dir, "report.json"), map[string]any{"passed": passed, "failed": len(cps) - passed, "candidate_sha256": candidateHash, "baseline_revision": baseline.Spec.Revision, "spec_sha256": spec}); err != nil {
		t.Fatal(err)
	}
	after, err := parity.FileSet(baseDir)
	if err != nil || !reflect.DeepEqual(files, after) {
		t.Fatal("baseline modified", err)
	}
}

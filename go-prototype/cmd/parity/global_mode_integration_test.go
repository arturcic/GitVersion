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

// Verify the real executable boundary: configuration precedence must produce
// identical values AND JSON types for every output field, not merely SemVer.
func TestPlatformGlobalDeploymentModeParity(t *testing.T) {
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
		t.Fatal("baseline assemblies differ", err)
	}
	spec, err := parity.FileHash(filepath.Join(root, "baseline.json"))
	if err != nil || spec != baseline.SpecSHA256 {
		t.Fatal("baseline spec differs", err)
	}
	dotnet, candidate := os.Getenv("PARITY_DOTNET"), os.Getenv("PARITY_CANDIDATE")
	candidateHash, err := parity.FileHash(candidate)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(filepath.Join(root, ".local"), "global-mode-parity-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)
	var recipes []fixtures.Recipe
	add := func(workflow, branch, raw string) {
		steps := []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}}
		if workflow == "GitFlow/v1" && (branch == "develop" || branch == "feature/topic" || branch == "release/1.3.0" || branch == "pull/42/merge") {
			steps = append(steps, fixtures.Step{Op: "branch", Name: "develop"}, fixtures.Step{Op: "checkout", Name: "develop"}, fixtures.Step{Op: "commit", ID: "development"})
		}
		if branch != "main" && branch != "develop" {
			steps = append(steps, fixtures.Step{Op: "branch", Name: branch}, fixtures.Step{Op: "checkout", Name: branch})
		}
		steps = append(steps, fixtures.Step{Op: "commit", ID: "first"}, fixtures.Step{Op: "checkpoint", ID: "first"}, fixtures.Step{Op: "commit", ID: "second"}, fixtures.Step{Op: "checkpoint", ID: "second"})
		recipes = append(recipes, fixtures.Recipe{ID: fmt.Sprintf("global-mode-%02d", len(recipes)), Description: workflow + " " + branch, Config: raw, Steps: steps})
	}
	for _, workflow := range []string{"GitHubFlow/v1", "GitFlow/v1"} {
		branches := []string{"main", "feature/topic", "release/1.3.0", "pull/42/merge"}
		if workflow == "GitFlow/v1" {
			branches = append(branches, "develop", "hotfix/1.2.4", "support/1.x")
		}
		for _, mode := range []string{"ManualDeployment", "ContinuousDelivery", "ContinuousDeployment"} {
			for _, branch := range branches {
				add(workflow, branch, fmt.Sprintf("workflow: %s\ncalculation:\n  mode: %s\n  branches:\n    main: {label: alpha}\n", workflow, mode))
			}
			// Explicit branch settings must win even if the global value follows them.
			for _, branch := range []string{"main", "support/1.x"} {
				if branch != "main" && workflow != "GitFlow/v1" {
					continue
				}
				kind := "main"
				if branch != "main" {
					kind = "support"
				}
				add(workflow, branch, fmt.Sprintf("workflow: %s\ncalculation:\n  branches:\n    %s: {mode: ContinuousDelivery, label: beta}\n  mode: %s\n", workflow, kind, mode))
			}
		}
	}
	// Exact reported configuration, tested on clean deterministic histories.
	raw := "workflow: GitFlow/v1\ncalculation:\n  mode: ManualDeployment\n  next-version: 7.0.0\n  branches:\n    main:\n      label: alpha\n    support:\n      label: beta\noutput: {}\n"
	add("GitFlow/v1", "main", raw)
	add("GitFlow/v1", "support/1.x", raw)
	if len(recipes) != 44 {
		t.Fatalf("recipe count %d", len(recipes))
	}
	if err := parity.WriteJSON(filepath.Join(dir, "recipes.json"), recipes); err != nil {
		t.Fatal(err)
	}
	cps, err := fixtures.Generate(recipes, filepath.Join(dir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateConfigurations(t.Context(), cps, baseline, dotnet, baseDir, dir, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	commands := []parity.Command{
		{Executable: dotnet, Arguments: append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, baseline.Spec.Arguments...)},
		{Executable: candidate, Arguments: []string{"calculate", "--repository", "{repo}"}},
	}
	passed := 0
	for _, cp := range cps {
		t.Run(cp.ID, func(t *testing.T) {
			var reference []byte
			for i, command := range commands {
				output := filepath.Join(dir, fmt.Sprintf("%s-%d", cp.ID, i))
				result, err := parity.Run(t.Context(), command, cp.Path, output, baseline.Spec.Environment, 30*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				if err := saveResult(output, result); err != nil {
					t.Fatal(err)
				}
				if err := result.Validate(); err != nil {
					t.Fatalf("command %d: %v", i, err)
				}
				if i == 0 {
					reference = result.Stdout
				} else {
					diff, err := parity.Compare(reference, result.Stdout)
					if err != nil || len(diff) > 0 {
						t.Fatalf("JSON parity: %v %+v", err, diff)
					}
				}
			}
			passed++
		})
	}
	after, err := parity.FileSet(baseDir)
	if err != nil || !reflect.DeepEqual(files, after) {
		t.Fatal("baseline modified", err)
	}
	if err := parity.WriteJSON(filepath.Join(dir, "report.json"), map[string]any{"passed": passed, "failed": len(cps) - passed, "baseline_revision": baseline.Spec.Revision, "spec_sha256": spec, "candidate_sha256": candidateHash, "checkpoints": cps}); err != nil {
		t.Fatal(err)
	}
}

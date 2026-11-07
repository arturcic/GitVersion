//go:build platform

package main

import (
	"encoding/json"
	"fmt"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

	for n, message := range []string{
		"Merge tag 'v2.0.0' into main",
		"Finish release/2.0.0 into main",
		"Merge pull request #1234 from project from release/2.0.0 to main",
		"Pull request #68: Release/2.0.0\n\nMerge in aaa/777 from release/2.0.0 to main",
		"Merged in release/2.0.0 (pull request #1789)",
		"Merge pull request #1234 in organization/release/2.0.0 into main",
		"Merge remote-tracking branch 'origin/release/2.0.0' into main",
		"Merge pull request 1234 from release/2.0.0 into main",
	} {
		recipes = append(recipes, fixtures.Recipe{ID: fmt.Sprintf("merge-format-%02d", n), Description: message, Config: "workflow: GitFlow/v1\ncalculation:\n  mode: ManualDeployment\n  branches:\n    main: {label: alpha}\n", Steps: []fixtures.Step{
			{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.0"}, {Op: "branch", Name: "release/2.0.0"}, {Op: "checkout", Name: "release/2.0.0"}, {Op: "commit", ID: "work"}, {Op: "checkout", Name: "main"}, {Op: "merge", ID: "merge", From: "release/2.0.0", Message: message}, {Op: "checkpoint", ID: "merged"},
		}})
	}

	for _, workflow := range []string{"GitHubFlow/v1", "GitFlow/v1"} {
		for _, mainline := range []bool{false, true} {
			for _, setting := range []string{"sha-middle", "sha-prefix", "sha-head", "date-middle", "paths-middle", "paths-head", "paths-config", "empty"} {
				doc := map[string]any{"workflow": workflow, "calculation": map[string]any{"mode": "ManualDeployment", "branches": map[string]any{"main": map[string]any{"label": "alpha"}}}}
				if mainline {
					doc["calculation"].(map[string]any)["strategies"] = []string{"Mainline"}
				}
				raw, _ := json.Marshal(doc)
				recipes = append(recipes, fixtures.Recipe{ID: fmt.Sprintf("ignore-%02d", len(recipes)), Description: setting, Config: string(raw), Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.0"}, {Op: "commit", ID: "middle", Message: "work +semver: major"}, {Op: "commit", ID: "after"}, {Op: "checkpoint", ID: "head"}}})
			}
		}
	}

	for _, workflow := range []string{"GitHubFlow/v1", "GitFlow/v1"} {
		for _, mainline := range []bool{false, true} {
			doc := map[string]any{"workflow": workflow, "calculation": map[string]any{"mode": "ManualDeployment", "ignore": map[string]any{"commits-before": "2025-01-01T00:01:00Z"}, "branches": map[string]any{"main": map[string]any{"label": "alpha"}}}}
			if mainline {
				doc["calculation"].(map[string]any)["strategies"] = []string{"Mainline"}
			}
			raw, _ := json.Marshal(doc)
			recipes = append(recipes, fixtures.Recipe{ID: fmt.Sprintf("date-floor-%02d", len(recipes)), Description: "Mismatched high tag exactly at minimum date", Config: string(raw), Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.0"}, {Op: "commit", ID: "boundary"}, {Op: "tag", Name: "5.0.0-other.1"}, {Op: "commit", ID: "after"}, {Op: "checkpoint", ID: "head"}}})
		}
	}

	for _, workflow := range []string{"GitHubFlow/v1", "GitFlow/v1"} {
		for _, mainline := range []bool{false, true} {
			for _, pattern := range []string{"^5[.]", "OTHER", "(?-i)OTHER", "^missing"} {
				calc := map[string]any{"mode": "ManualDeployment", "ignore": map[string]any{"tags": []string{pattern}}, "branches": map[string]any{"main": map[string]any{"label": "alpha"}}}
				if mainline {
					calc["strategies"] = []string{"Mainline"}
				}
				raw, _ := json.Marshal(map[string]any{"workflow": workflow, "calculation": calc})
				recipes = append(recipes, fixtures.Recipe{ID: fmt.Sprintf("tag-name-%02d", len(recipes)), Description: pattern, Config: string(raw), Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.0"}, {Op: "commit", ID: "high"}, {Op: "tag", Name: "5.0.0-other.1"}, {Op: "commit", ID: "after"}, {Op: "checkpoint", ID: "head"}}})
			}
		}
	}
	releaseSteps := []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.0"}, {Op: "branch", Name: "develop"}, {Op: "checkout", Name: "develop"}, {Op: "commit", ID: "base"}, {Op: "branch", Name: "release/2.0.0"}, {Op: "checkout", Name: "release/2.0.0"}, {Op: "commit", ID: "release"}, {Op: "tag", Name: "2.0.0-beta.1"}, {Op: "checkout", Name: "develop"}, {Op: "commit", ID: "after"}, {Op: "checkpoint", ID: "head"}}
	for _, mainline := range []bool{false, true} {
		for _, pattern := range []string{"^release/", "^RELEASE/", "(?-i)^RELEASE/", "^missing"} {
			calc := map[string]any{"mode": "ManualDeployment", "ignore": map[string]any{"branches": []string{pattern}}}
			if mainline {
				calc["strategies"] = []string{"Mainline"}
			}
			raw, _ := json.Marshal(map[string]any{"workflow": "GitFlow/v1", "calculation": calc})
			recipes = append(recipes, fixtures.Recipe{ID: fmt.Sprintf("branch-name-%02d", len(recipes)), Description: pattern, Config: string(raw), Steps: releaseSteps})
		}
	}
	for _, workflow := range []string{"GitHubFlow/v1", "GitFlow/v1"} {
		for _, variant := range []string{"plain", "tags", "branches"} {
			doc := map[string]any{"workflow": workflow, "mode": "ManualDeployment", "next-version": "2.0.0", "strategies": "ConfiguredNextVersion, TaggedCommit", "branches": map[string]any{"main": map[string]any{"label": "alpha"}}}
			steps := []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.0"}, {Op: "commit", ID: "high"}, {Op: "tag", Name: "5.0.0-other.1"}, {Op: "commit", ID: "after"}, {Op: "checkpoint", ID: "head"}}
			if variant == "tags" {
				doc["ignore"] = map[string]any{"tags": []string{"other"}}
			}
			if variant == "branches" {
				doc["ignore"] = map[string]any{"branches": []string{"^release/"}}
				steps = releaseSteps
			}
			raw, _ := json.Marshal(doc)
			recipes = append(recipes, fixtures.Recipe{ID: fmt.Sprintf("legacy-%02d", len(recipes)), Description: variant, Config: string(raw), Steps: steps})
		}
	}

	for _, setting := range []string{"missing", "empty", "empty-map", "null-workflow", "priority", "relative", "absolute"} {
		recipes = append(recipes, fixtures.Recipe{ID: "locator-" + setting, Description: setting, Config: "workflow: GitHubFlow/v1\n", Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.0"}, {Op: "commit", ID: "after"}, {Op: "checkpoint", ID: "head"}}})
	}

	for _, setting := range []struct{ id, configuration string }{
		{"legacy-mapper-weight", "workflow: GitHubFlow/v1\nbranches:\n  main: {mode: ManualDeployment, label: ci, increment: Minor, pre-release-weight: 42}\n"},
		{"mapper-weight", "workflow: GitHubFlow/v1\ncalculation:\n  branches:\n    main: {mode: ManualDeployment, label: ci, increment: Minor}\noutput:\n  branches:\n    main: {pre-release-weight: 42}\n"},
	} {
		recipes = append(recipes, fixtures.Recipe{
			ID: setting.id, Description: "Calculation and output settings on one branch", Config: setting.configuration,
			Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.0"}, {Op: "commit", ID: "after"}, {Op: "checkpoint", ID: "head"}},
		})
	}

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

			if strings.HasPrefix(cp.Recipe, "ignore-") {
				g, err := git.PlainOpen(cp.Path)
				if err != nil {
					t.Fatal(err)
				}
				head, err := g.CommitObject(plumbing.NewHash(cp.Head))
				if err != nil {
					t.Fatal(err)
				}
				var doc map[string]any
				raw, err := os.ReadFile(filepath.Join(cp.Path, "GitVersion.yml"))
				if err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(raw, &doc); err != nil {
					t.Fatal(err)
				}
				setting := ""
				for _, r := range recipes {
					if r.ID == cp.Recipe {
						setting = r.Description
					}
				}
				ignore := map[string]any{}
				switch setting {
				case "sha-middle":
					ignore["sha"] = []string{head.ParentHashes[0].String()}
				case "sha-prefix":
					ignore["sha"] = []string{strings.ToUpper(head.ParentHashes[0].String()[:8])}
				case "sha-head":
					ignore["sha"] = []string{cp.Head}
				case "date-middle":
					ignore["commits-before"] = "2025-01-01T00:01:00Z"
				case "paths-middle":
					ignore["paths"] = []string{"^middle[.]txt$"}
				case "paths-head":
					ignore["paths"] = []string{"^after[.]txt$"}
				case "paths-config":
					ignore["paths"] = []string{"^GitVersion[.]yml$"}
				case "empty":
					ignore["sha"] = []string{}
					ignore["paths"] = []string{}
				}
				doc["calculation"].(map[string]any)["ignore"] = ignore
				raw, err = json.Marshal(doc)
				if err != nil {
					t.Fatal(err)
				}
				write("GitVersion.yml", string(raw)+"\n")
				wt, err := g.Worktree()
				if err != nil {
					t.Fatal(err)
				}
				if _, err = wt.Add("GitVersion.yml"); err != nil {
					t.Fatal(err)
				}
				hash, err := wt.Commit("Configure ignore rules", &git.CommitOptions{Author: &object.Signature{Name: "Parity Configuration", Email: "parity@example.invalid", When: time.Date(2025, 1, 1, 0, 3, 0, 0, time.UTC)}})
				if err != nil {
					t.Fatal(err)
				}
				cp.Head = hash.String()
			}

			customPath := ""
			if strings.HasPrefix(cp.Recipe, "locator-") {
				setting := strings.TrimPrefix(cp.Recipe, "locator-")
				g, err := git.PlainOpen(cp.Path)
				if err != nil {
					t.Fatal(err)
				}
				wt, err := g.Worktree()
				if err != nil {
					t.Fatal(err)
				}
				switch setting {
				case "missing":
					if _, err = wt.Remove("GitVersion.yml"); err != nil {
						t.Fatal(err)
					}
				case "empty", "empty-map", "null-workflow":
					value := ""
					if setting == "empty-map" {
						value = "{}"
					}
					if setting == "null-workflow" {
						value = "workflow: null"
					}
					write("GitVersion.yml", value)
					if _, err = wt.Add("GitVersion.yml"); err != nil {
						t.Fatal(err)
					}
				case "priority":
					write("GitVersion.yaml", "bad YAML: [")
					if _, err = wt.Add("GitVersion.yaml"); err != nil {
						t.Fatal(err)
					}
				case "relative", "absolute":
					write("Configuration/custom.yaml", "workflow: GitHubFlow/v1\ncalculation: {next-version: 9.0.0, mode: ManualDeployment, branches: {main: {label: custom}}}\n")
					if _, err = wt.Add("Configuration/custom.yaml"); err != nil {
						t.Fatal(err)
					}
					customPath = "Configuration/custom.yaml"
					if setting == "absolute" {
						customPath = "@absolute"
					}
				}
				hash, err := wt.Commit("Configure locator case", &git.CommitOptions{Author: &object.Signature{Name: "Parity Configuration", Email: "parity@example.invalid", When: time.Date(2025, 1, 1, 0, 3, 0, 0, time.UTC)}})
				if err != nil {
					t.Fatal(err)
				}
				cp.Head = hash.String()
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
			environment := map[string]string{}
			for k, v := range baseline.Spec.Environment {
				environment[k] = v
			}
			if strings.HasPrefix(cp.Recipe, "legacy-") {
				environment["GITVERSION_CONFIGURATION_VERSION"] = "v6"
			}
			caseCommands := append([]parity.Command{}, commands...)
			if customPath != "" {
				for j := range caseCommands {
					caseCommands[j].Arguments = append(append([]string{}, caseCommands[j].Arguments...), "--config", customPath)
				}
			}
			var reference []byte
			for j, command := range caseCommands {
				output := filepath.Join(dir, fmt.Sprintf("%s-%d", cp.ID, j))
				if customPath == "@absolute" {
					command.Arguments = append([]string{}, command.Arguments...)
					command.Arguments[len(command.Arguments)-1] = filepath.Join(output, "repo", "Configuration", "custom.yaml")
				}
				result, err := parity.Run(t.Context(), command, cp.Path, output, environment, 60*time.Second)
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

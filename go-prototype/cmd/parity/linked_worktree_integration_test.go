//go:build platform

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformLinkedWorktreeParity(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	published := filepath.Join(root, ".local/baseline")
	files, err := parity.FileSet(published)
	if err != nil || !reflect.DeepEqual(files, baseline.Files) {
		t.Fatal("published baseline changed", err)
	}
	specHash, err := parity.FileHash(filepath.Join(root, "baseline.json"))
	if err != nil || specHash != baseline.SpecSHA256 {
		t.Fatal("baseline specification changed", err)
	}
	dotnet, candidate := os.Getenv("PARITY_DOTNET"), os.Getenv("PARITY_CANDIDATE")
	candidateHash, err := parity.FileHash(candidate)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := os.MkdirTemp(filepath.Join(root, ".local"), "linked-worktree-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", workspace)
	cases, err := fixtures.Generate([]fixtures.Recipe{{
		ID: "linked-base", InitialBranch: "main", Config: "workflow: GitFlow/v1\n",
		Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}, {Op: "commit", ID: "later"}, {Op: "checkpoint", ID: "head"}},
	}}, filepath.Join(workspace, "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	report := Report{Mode: "linked-worktree", Calculator: "external-candidate"}
	for _, tc := range []struct {
		name, branch        string
		relative, extension bool
	}{
		{"absolute-feature", "feature/topic", false, false},
		{"relative-feature", "feature/topic", true, false},
		{"relative-common-config", "feature/topic", true, true},
		{"absolute-develop", "develop", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outputs := map[string][]byte{}
			for _, backend := range []string{"csharp", "go"} {
				runDir := filepath.Join(workspace, tc.name+"-"+backend)
				if err := os.Mkdir(runDir, 0755); err != nil {
					t.Fatal(err)
				}
				repo := filepath.Join(runDir, "main")
				linked := filepath.Join(runDir, "linked")
				if err := fixtures.Copy(cases[0].Path, repo); err != nil {
					t.Fatal(err)
				}
				if err := fixtures.LinkWorktree(repo, linked, tc.branch, tc.relative); err != nil {
					t.Fatal(err)
				}
				if tc.extension {
					config := filepath.Join(repo, ".git", "config")
					raw, err := os.ReadFile(config)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(config, append(raw, []byte("\n[extensions]\n worktreeConfig = true\n")...), 0644); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(repo, ".git", "worktrees", "linked", "config.worktree"), []byte("[core]\n bare = true\n"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				before, err := fixtures.Digest(runDir)
				if err != nil {
					t.Fatal(err)
				}
				executable, args := candidate, []string{"calculate", "--repository", linked}
				if backend == "csharp" {
					executable = dotnet
					args = []string{"--fx-version", baseline.Spec.Runtime, filepath.Join(published, "gitversion.dll")}
					for _, arg := range baseline.Spec.Arguments {
						args = append(args, strings.ReplaceAll(arg, "{repo}", linked))
					}
				}
				home := filepath.Join(runDir, "home")
				if err := os.Mkdir(home, 0700); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				cmd := exec.CommandContext(ctx, executable, args...)
				cmd.Dir = linked
				cmd.Env = parity.Environment(home, baseline.Spec.Environment)
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				stdout, runErr := cmd.Output()
				cancel()
				after, err := fixtures.Digest(runDir)
				if err != nil {
					t.Fatal(err)
				}
				result := parity.Result{Command: parity.Command{Executable: executable, Arguments: args}, Before: before, After: after, Stdout: stdout, Stderr: stderr.Bytes()}
				if runErr != nil {
					result.ExitCode = -1
					result.Error = runErr.Error()
				}
				resultDir := filepath.Join(workspace, tc.name+"-"+backend+"-result")
				if err := os.Mkdir(resultDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := saveResult(resultDir, result); err != nil {
					t.Fatal(err)
				}
				if runErr != nil || stderr.Len() != 0 || before != after {
					t.Fatalf("%s failed or changed the fixture: %v, %s", backend, runErr, stderr.String())
				}
				values, err := parity.Object(stdout)
				if err != nil || len(values) != 33 || values["BranchName"] != tc.branch || values["Sha"] != cases[0].Head {
					t.Fatalf("%s selected the wrong worktree: %v, %s", backend, err, stdout)
				}
				outputs[backend] = stdout
			}
			differences, err := parity.Compare(outputs["csharp"], outputs["go"])
			if err != nil || len(differences) != 0 {
				t.Fatalf("linked worktree parity: %v, %+v", err, differences)
			}
			report.Passed++
			report.Cases = append(report.Cases, CaseResult{ID: tc.name, Status: "pass"})
		})
	}
	report.Failed = 4 - report.Passed
	if err := parity.WriteJSON(filepath.Join(workspace, "report.json"), report); err != nil {
		t.Fatal(err)
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "provenance.json"), map[string]string{"candidate_sha256": candidateHash, "baseline_revision": baseline.Spec.Revision, "baseline_spec_sha256": specHash, "platform": parity.Platform()}); err != nil {
		t.Fatal(err)
	}
}

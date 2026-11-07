//go:build platform

package main

import (
	"bytes"
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformRepositoryPreparationParity(t *testing.T) {
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
	report, err := os.MkdirTemp(os.Getenv("PARITY_EVIDENCE_DIR"), "repository-preparation-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", report)
	scratch := t.TempDir()
	remotePath := filepath.Join(scratch, "upstream")
	prepareExecutorCacheRepo(t, remotePath)
	remote, err := git.PlainOpen(remotePath)
	if err != nil {
		t.Fatal(err)
	}
	head, err := remote.Head()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := remote.CreateTag("1.0.0", head.Hash(), nil); err != nil {
		t.Fatal(err)
	}
	seed := filepath.Join(scratch, "seed")
	if _, err := git.PlainClone(seed, false, &git.CloneOptions{URL: remotePath}); err != nil {
		t.Fatal(err)
	}
	if _, err := remote.CreateTag("2.0.0", head.Hash(), nil); err != nil {
		t.Fatal(err)
	}
	work, err := remote.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	signature := &object.Signature{Name: "Fixture", Email: "fixture@example.invalid", When: time.Unix(1700000100, 0)}
	advanced, err := work.Commit("advance remote main", &git.CommitOptions{AllowEmptyCommits: true, Author: signature, Committer: signature})
	if err != nil {
		t.Fatal(err)
	}
	sourceBefore, err := fixtures.State(remotePath)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name                                                 string
		dynamic, contextual, noFetch, noNormalize, prevented bool
	}{
		{name: "dynamic-explicit", dynamic: true, noFetch: true},
		{name: "dynamic-context", dynamic: true, contextual: true, noFetch: true},
		{name: "ci-fetch", contextual: true},
		{name: "ci-no-fetch", contextual: true, noFetch: true},
		{name: "ci-no-normalize", contextual: true, noNormalize: true},
		{name: "ci-provider-prevents-fetch", contextual: true, prevented: true},
		{name: "ci-detached-normalization", noFetch: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(implementation string) []byte {
				t.Helper()
				area := filepath.Join(report, tc.name, implementation)
				if err := os.MkdirAll(area, 0755); err != nil {
					t.Fatal(err)
				}
				cwd := filepath.Join(scratch, tc.name+"-"+implementation)
				env := maps.Clone(baseline.Spec.Environment)
				args := []string{"--no-cache", "--verbosity", "quiet"}
				if tc.noFetch {
					args = append(args, "--no-fetch")
				}
				if tc.noNormalize {
					args = append(args, "--no-normalize")
				}
				var cloneBase string
				if tc.dynamic {
					if err := os.Mkdir(cwd, 0755); err != nil {
						t.Fatal(err)
					}
					cloneBase = filepath.Join(scratch, tc.name+"-"+implementation+"-clones")
					args = append(args, "--url", remotePath, "--dynamic-repo-location", cloneBase)
					if tc.contextual {
						env["GIT_BRANCH"] = "feature/dynamic"
					} else {
						args = append(args, "--branch", "main")
					}
				} else {
					if err := fixtures.Copy(seed, cwd); err != nil {
						t.Fatal(err)
					}
					env["ContinuaCI.Version"] = "true"
					if tc.prevented {
						delete(env, "ContinuaCI.Version")
						env["TF_BUILD"] = "true"
					}
					if tc.contextual {
						env["GIT_BRANCH"] = "main"
					} else {
						repo, err := git.PlainOpen(cwd)
						if err != nil {
							t.Fatal(err)
						}
						if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, head.Hash())); err != nil {
							t.Fatal(err)
						}
					}
				}
				exe := candidate
				if implementation == "csharp" {
					exe = os.Getenv("PARITY_DOTNET")
					args = append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, args...)
					if backend := os.Getenv("PARITY_PREPARATION_CSHARP_BACKEND"); backend != "" {
						env["GITVERSION_GIT_BACKEND"] = backend
					}
				}
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				command := exec.CommandContext(ctx, exe, args...)
				command.Dir, command.Env = cwd, parity.Environment(area, env)
				var stdout, stderr bytes.Buffer
				command.Stdout, command.Stderr = &stdout, &stderr
				if err := command.Run(); err != nil {
					t.Fatalf("%s: %v, stdout %s, stderr %s", implementation, err, &stdout, &stderr)
				}
				if err := os.WriteFile(filepath.Join(area, "stdout.json"), stdout.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(area, "stderr"), stderr.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				values, err := parity.Object(stdout.Bytes())
				wantBranch, wantHead := "main", head.Hash()
				if tc.dynamic {
					wantHead = advanced
					if tc.contextual {
						wantBranch = "feature/dynamic"
					}
					if entries, err := os.ReadDir(cwd); err != nil || len(entries) != 0 {
						t.Fatalf("caller checkout changed: %v, %v", entries, err)
					}
					if _, err := os.Stat(filepath.Join(cloneBase, "upstream", ".git")); err != nil {
						t.Fatal(err)
					}
				} else {
					repo, err := git.PlainOpen(cwd)
					if err != nil {
						t.Fatal(err)
					}
					ref, err := repo.Reference("refs/remotes/origin/main", true)
					wantRemote := head.Hash()
					if !tc.noFetch && !tc.noNormalize && !tc.prevented {
						wantRemote = advanced
					}
					if err != nil || ref.Hash() != wantRemote {
						t.Fatalf("tracking ref %v, want %v (%v)", ref, wantRemote, err)
					}
					ref, err = repo.Head()
					if err != nil || ref.Hash() != head.Hash() {
						t.Fatalf("checkout moved: %v, %v", ref, err)
					}
				}
				if err != nil || values["BranchName"] != wantBranch || values["Sha"] != wantHead.String() {
					t.Fatalf("unexpected output %s, want %s at %s (%v)", &stdout, wantBranch, wantHead, err)
				}
				return stdout.Bytes()
			}
			ref, actual := run("csharp"), run("go")
			if diff, err := parity.Compare(ref, actual); err != nil || len(diff) != 0 {
				t.Fatalf("JSON differs: %v (%v)", diff, err)
			}
		})
	}
	after, err := fixtures.State(remotePath)
	if err != nil || after != sourceBefore {
		t.Fatalf("source repository changed: %v", err)
	}
	if !t.Failed() {
		if err := parity.WriteJSON(filepath.Join(report, "report.json"), map[string]any{"comparisons": 7, "baseline_revision": baseline.Spec.Revision, "baseline_files": files, "spec_sha256": spec, "candidate_sha256": hash, "backend_override": strings.TrimSpace(os.Getenv("PARITY_PREPARATION_CSHARP_BACKEND"))}); err != nil {
			t.Fatal(err)
		}
	}
}

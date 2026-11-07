//go:build platform

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformShallowCIParity(t *testing.T) {
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
	dir, err := os.MkdirTemp(os.Getenv("PARITY_EVIDENCE_DIR"), "shallow-ci-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)
	selected := map[string]bool{
		"VerifyAzurePipelinesPullRequest": true,
		"VerifyGitLabCIPullRequest":       true,
	}
	comparisons := 0
	for suite, group := range ciCases(t) {
		for method, item := range group.Methods {
			if suite == "PullRequestInBuildAgentTest" && !selected[method] {
				continue
			}
			for _, tc := range item.Cases {
				comparisons++
				t.Run(method+"/"+tc.ID, func(t *testing.T) {
					type result struct {
						Stdout, Stderr string
						Backend        string
						ExitCode       int
					}
					run := func(name, executable string, prefix []string) result {
						t.Helper()
						caseDir := filepath.Join(dir, filepath.FromSlash(method+"/"+tc.ID), name)
						if err := os.MkdirAll(caseDir, 0755); err != nil {
							t.Fatal(err)
						}
						path, _ := ciRepository(t, caseDir, suite, method, tc.Ref)
						before, err := fixtures.State(path)
						if err != nil {
							t.Fatal(err)
						}
						env := map[string]string{}
						for key, value := range baseline.Spec.Environment {
							env[key] = value
						}
						if name == "csharp" && (runtime.GOOS == "linux" || os.Getenv("PARITY_CSHARP_CI_NATIVE") == "1") && !strings.Contains(method, "Shallow") {
							// The offline Ubuntu image has no Git CLI for the C# managed mutator.
							env["GITVERSION_GIT_BACKEND"] = "libgit2"
						}
						for key, value := range tc.Environment {
							env[key] = value
						}
						if suite == "GitVersionExecutorTests" {
							env["TF_BUILD"] = "true"
						}
						args := append(prefix, "--output", "json", "--no-fetch", "--no-cache", "--verbosity", "quiet")
						if strings.Contains(method, "AllowShallow") {
							args = append(args, "--allow-shallow")
						}
						ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
						defer cancel()
						command := exec.CommandContext(ctx, executable, args...)
						command.Dir, command.Env = path, parity.Environment(caseDir, env)
						var out, diagnostics bytes.Buffer
						command.Stdout, command.Stderr = &out, &diagnostics
						runErr := command.Run()
						got := result{Stdout: out.String(), Stderr: diagnostics.String(), Backend: "go-git"}
						if name == "csharp" {
							got.Backend = env["GITVERSION_GIT_BACKEND"]
						}
						if runErr != nil {
							var status *exec.ExitError
							if !errors.As(runErr, &status) || ctx.Err() != nil {
								t.Fatal(runErr)
							}
							got.ExitCode = status.ExitCode()
						}
						if err := parity.WriteJSON(filepath.Join(caseDir, "process.json"), got); err != nil {
							t.Fatal(err)
						}
						if name == "go" {
							after, err := fixtures.State(path)
							if err != nil || after != before {
								t.Fatalf("Go changed the fixture: %v", err)
							}
						}
						return got
					}
					ref := run("csharp", os.Getenv("PARITY_DOTNET"), []string{
						"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll"),
					})
					got := run("go", candidate, nil)
					if strings.HasSuffix(method, "_ThrowException") {
						for _, result := range []result{ref, got} {
							if result.ExitCode != 1 || !strings.Contains(result.Stdout+result.Stderr, tc.Expected[0]) {
								t.Fatalf("expected rejection %q, got %+v", tc.Expected[0], result)
							}
						}
						return
					}
					if ref.ExitCode != 0 || got.ExitCode != 0 {
						t.Fatalf("C# %+v, Go %+v", ref, got)
					}
					compareRepositoryExecutionJSON(t, []byte(ref.Stdout), []byte(got.Stdout))
				})
			}
		}
	}
	if !t.Failed() {
		if err := parity.WriteJSON(filepath.Join(dir, "report.json"), map[string]any{
			"comparisons": comparisons, "baseline_revision": baseline.Spec.Revision, "candidate_sha256": hash,
			"baseline_files": files, "spec_sha256": spec,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

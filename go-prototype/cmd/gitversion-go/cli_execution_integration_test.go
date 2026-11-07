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
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformCLIExecutionParity(t *testing.T) {
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
	dir, err := os.MkdirTemp(os.Getenv("PARITY_EVIDENCE_DIR"), "cli-execution-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)
	cps := makeCLIHistory(t, dir, "1.0.0", true)
	requested, head := cps[0], cps[1]
	empty := filepath.Join(dir, "empty")
	if _, err := git.PlainInit(empty, false); err != nil {
		t.Fatal(err)
	}
	nonRepo := t.TempDir()
	missing := filepath.Join(nonRepo, "missing")
	nested := filepath.Join(head.Path, "nested")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	validation := filepath.Join(dir, "config é 😀 + ' \"quoted\".yml")

	type testCase struct {
		name, cwd, parser, sha, message string
		args                            []string
	}
	cases := []testCase{
		{name: "requested-v7", cwd: head.Path, parser: "v7", sha: requested.Head, args: []string{"--no-fetch", "--no-cache", "--no-normalize", "--commit", requested.Head}},
		{name: "requested-v6", cwd: head.Path, parser: "v6", sha: requested.Head, args: []string{head.Path, "-nofetch", "-nocache", "-nonormalize", "-c", requested.Head}},
		{name: "nested-discovery", cwd: nested, parser: "v7", sha: head.Head, args: []string{"--no-fetch", "--no-cache", "--no-normalize"}},
		{name: "not-a-repository", cwd: nonRepo, parser: "v7", message: "Cannot find the .git directory", args: []string{nonRepo}},
		{name: "unborn-branch", cwd: empty, parser: "v7", message: "No commits found on the current branch.", args: []string{empty, "--log-file", "console", "--no-fetch", "--no-cache", "--no-normalize"}},
		{name: "missing-directory", cwd: nonRepo, parser: "v7", message: "The working directory '" + missing + "' does not exist.", args: []string{"--target-path", missing}},
		{name: "validation-escaping", cwd: dir, parser: "v7", message: "GVCONFIG_FILE", args: []string{"config", "validate", "--config", validation, "--format", "json"}},
	}
	before, err := fixtures.State(head.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			type result struct {
				ExitCode int
				Stdout   string
				Stderr   string
			}
			run := func(implementation, executable string, arguments []string) result {
				t.Helper()
				outputDir := filepath.Join(dir, tc.name, implementation)
				if err := os.MkdirAll(outputDir, 0755); err != nil {
					t.Fatal(err)
				}
				env := map[string]string{}
				for key, value := range baseline.Spec.Environment {
					env[key] = value
				}
				env["GITVERSION_ARGUMENT_PARSER_VERSION"] = tc.parser
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				command := exec.CommandContext(ctx, executable, arguments...)
				command.Dir = tc.cwd
				command.Env = parity.Environment(outputDir, env)
				var stdout, stderr bytes.Buffer
				command.Stdout, command.Stderr = &stdout, &stderr
				err := command.Run()
				got := result{Stdout: stdout.String(), Stderr: stderr.String()}
				if err != nil {
					var status *exec.ExitError
					if !errors.As(err, &status) || ctx.Err() != nil {
						t.Fatal(err)
					}
					got.ExitCode = status.ExitCode()
				}
				if err := parity.WriteJSON(filepath.Join(outputDir, "process.json"), got); err != nil {
					t.Fatal(err)
				}
				return got
			}
			ref := run("csharp", os.Getenv("PARITY_DOTNET"), append([]string{
				"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll"),
			}, tc.args...))
			actual := run("go", candidate, tc.args)
			for _, got := range []result{ref, actual} {
				if tc.message != "" {
					if got.ExitCode != 1 || !strings.Contains(got.Stdout+got.Stderr, tc.message) {
						t.Fatalf("expected failure with %q, got %+v", tc.message, got)
					}
				} else {
					fields, err := parity.Object([]byte(got.Stdout))
					if got.ExitCode != 0 || got.Stderr != "" || err != nil || len(fields) != 33 || fields["Sha"] != tc.sha {
						t.Fatalf("expected 33 JSON fields and SHA %s, got %+v (%v)", tc.sha, got, err)
					}
				}
			}
			switch {
			case tc.name == "validation-escaping":
				if actual.Stdout != ref.Stdout {
					t.Fatalf("validation JSON differs\nC# %q\nGo %q", ref.Stdout, actual.Stdout)
				}
			case tc.message == "":
				differences, err := parity.Compare([]byte(ref.Stdout), []byte(actual.Stdout))
				if err != nil || len(differences) != 0 {
					t.Fatalf("calculation differs: %v, %v", differences, err)
				}
			}
		})
	}
	after, err := fixtures.State(head.Path)
	if err != nil || before != after {
		t.Fatalf("calculation changed the checkout: %v", err)
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

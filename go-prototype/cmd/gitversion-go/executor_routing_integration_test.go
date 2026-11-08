//go:build platform

package main

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"gopkg.in/yaml.v3"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformExecutorRoutingParity(t *testing.T) {
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
	dir, err := os.MkdirTemp(os.Getenv("PARITY_EVIDENCE_DIR"), "executor-routing-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)
	type routingCase struct {
		name, version, mode string
		args                []string
		buildNumber         bool
		failureText         string
	}
	query := []string{"config", "get", "output.update-build-number"}
	fileOutputs := []string{"--no-cache", "--no-fetch", "--no-normalize", "--update-assembly-info", "AssemblyInfo.cs",
		"--ensure-assembly-info", "--update-wix-version-file"}
	cases := []routingCase{
		{name: "display-v6", version: "v6", mode: "display"},
		{name: "display-v7", version: "v7", mode: "display"},
		{name: "scalar-v7", version: "v7", mode: "scalar", args: query},
		{name: "scalar-v6", version: "v6", mode: "scalar", args: query, failureText: config.RequireV7Query},
		{name: "build-number-off", version: "v7", mode: "write"},
		{name: "build-number-on", version: "v7", mode: "write", buildNumber: true},
		{name: "configuration-failure", version: "v7", mode: "invalid-configuration", failureText: "workflow"},
		{name: "repository-failure", version: "v7", mode: "invalid-repository", failureText: "No commits found on the current branch."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caseDir := filepath.Join(dir, tc.name)
			if err := os.Mkdir(caseDir, 0755); err != nil {
				t.Fatal(err)
			}
			repoPath := filepath.Join(caseDir, "source")
			args := tc.args
			switch tc.mode {
			case "display":
				override := "tag-prefix=custom-"
				if tc.version == "v7" {
					override = "calculation." + override
				}
				args = []string{"--show-config", "--override-config", override}
			case "write":
				args = append([]string{"--output", "buildserver", "--verbosity", "quiet"}, fileOutputs...)
			case "invalid-configuration":
				args = fileOutputs
			case "invalid-repository":
				args = append([]string{"--log-file", "console"}, fileOutputs...)
			}
			if tc.mode == "invalid-repository" {
				if _, err := git.PlainInit(repoPath, false); err != nil {
					t.Fatal(err)
				}
			} else {
				prepareExecutorCacheRepo(t, repoPath)
			}
			if tc.mode == "invalid-configuration" {
				if err := os.WriteFile(filepath.Join(repoPath, "GitVersion.yml"), []byte("workflow: invalid-workflow\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			env := maps.Clone(baseline.Spec.Environment)
			env["GITVERSION_CONFIGURATION_VERSION"] = tc.version
			if tc.mode == "write" {
				env["TEAMCITY_VERSION"] = "2024.12"
				contents := "output:\n  update-build-number: false\n"
				if tc.buildNumber {
					contents = "output:\n  update-build-number: true\n"
				}
				if err := os.WriteFile(filepath.Join(repoPath, "GitVersion.yml"), []byte(contents), 0600); err != nil {
					t.Fatal(err)
				}
				repository, err := git.PlainOpen(repoPath)
				if err != nil {
					t.Fatal(err)
				}
				head, err := repository.Head()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := repository.CreateTag("1.2.3", head.Hash(), nil); err != nil {
					t.Fatal(err)
				}
			}
			run := func(name, executable string, arguments []string) parity.Result {
				t.Helper()
				workspace := filepath.Join(caseDir, name)
				result, err := parity.RunInWorkingDirectory(t.Context(), parity.Command{Executable: executable, Arguments: arguments},
					repoPath, workspace, env, 30*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				if err := parity.WriteJSON(filepath.Join(workspace, "process.json"), result); err != nil {
					t.Fatal(err)
				}
				for file, data := range map[string][]byte{"stdout": result.Stdout, "stderr": result.Stderr} {
					if err := os.WriteFile(filepath.Join(workspace, file), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if tc.failureText != "" {
					if result.ExitCode != 1 || len(result.Stdout) != 0 || !strings.Contains(string(result.Stderr), tc.failureText) || result.Before != result.After {
						t.Fatalf("%s failure exit %d (%s), stdout %q, stderr %q", name, result.ExitCode, result.Error, result.Stdout, result.Stderr)
					}
				} else if result.ExitCode != 0 || len(result.Stderr) != 0 {
					t.Fatalf("%s exit %d (%s), stderr %q", name, result.ExitCode, result.Error, result.Stderr)
				} else if tc.mode != "write" && result.Before != result.After {
					t.Fatalf("%s query changed repository", name)
				}
				return result
			}
			ref := run("csharp", os.Getenv("PARITY_DOTNET"), append([]string{
				"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll"),
			}, args...))
			actual := run("go", candidate, args)
			if tc.failureText != "" {
				return
			}
			switch tc.mode {
			case "display":
				for _, result := range []parity.Result{ref, actual} {
					var document map[string]any
					if err := yaml.Unmarshal(result.Stdout, &document); err != nil {
						t.Fatal(err)
					}
					want := "[vV]?"
					if tc.version == "v7" {
						section, ok := document["calculation"].(map[string]any)
						if !ok {
							t.Fatal("missing v7 calculation section")
						}
						document, want = section, "custom-"
					}
					if document["tag-prefix"] != want {
						t.Fatalf("%s tag prefix = %v, want %s", tc.version, document["tag-prefix"], want)
					}
				}
			case "scalar":
				if string(ref.Stdout) != string(nativeText([]byte("true\n"))) || !bytes.Equal(ref.Stdout, actual.Stdout) {
					t.Fatalf("scalar output differs: %q vs %q", ref.Stdout, actual.Stdout)
				}
			case "write":
				if !bytes.Equal(ref.Stdout, actual.Stdout) ||
					strings.Contains(string(actual.Stdout), "##teamcity[buildNumber '1.2.3']") != tc.buildNumber {
					t.Fatalf("TeamCity output differs\nC# %s\nGo %s", ref.Stdout, actual.Stdout)
				}
				for _, filename := range []string{"AssemblyInfo.cs", wixVersionFileName} {
					original, err := os.ReadFile(filepath.Join(caseDir, "csharp", "repo", filename))
					if err != nil {
						t.Fatal(err)
					}
					generated, err := os.ReadFile(filepath.Join(caseDir, "go", "repo", filename))
					if err != nil || !bytes.Equal(original, generated) {
						t.Fatalf("%s differs: %v", filename, err)
					}
				}
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

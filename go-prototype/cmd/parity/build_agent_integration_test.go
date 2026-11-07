//go:build platform

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformBuildAgentParity(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(filepath.Join(root, ".local"), "build-agent-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	baseDir := filepath.Join(root, ".local/baseline")
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
	type testCase struct {
		name             string
		environment      map[string]string
		outputFiles      []string
		noUpdate, dotenv bool
	}
	cases := []testCase{
		{name: "local"},
		{name: "azure", environment: map[string]string{"TF_BUILD": "true", "BUILD_BUILDNUMBER": "build-$(GitVersion.FullSemVer)"}},
		{name: "teamcity", environment: map[string]string{"TEAMCITY_VERSION": "2026.1", "Git_Branch": "main"}},
		{name: "myget", environment: map[string]string{"BuildRunner": "MyGet"}},
		{name: "continua", environment: map[string]string{"ContinuaCI.Version": "1.0"}},
		{name: "drone", environment: map[string]string{"DRONE": "true", "DRONE_BRANCH": "main"}},
		{name: "buildkite", environment: map[string]string{"BUILDKITE": "true", "BUILDKITE_BRANCH": "main"}},
		{name: "space", environment: map[string]string{"JB_SPACE_PROJECT_KEY": "task", "JB_SPACE_GIT_BRANCH": "refs/heads/main"}},
		{name: "travis", environment: map[string]string{"TRAVIS": "true", "CI": "true"}},
		{name: "github", environment: map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF_TYPE": "branch", "GITHUB_REF": "refs/heads/main", "GITHUB_ENV": "github.env"}, outputFiles: []string{"github.env"}},
		{name: "gitlab", environment: map[string]string{"GITLAB_CI": "true", "CI_COMMIT_REF_NAME": "main"}, outputFiles: []string{"gitversion.properties"}},
		{name: "jenkins", environment: map[string]string{"JENKINS_URL": "http://local.invalid", "GIT_LOCAL_BRANCH": "main"}, outputFiles: []string{"gitversion.properties"}},
		{name: "codebuild", environment: map[string]string{"CODEBUILD_SOURCE_VERSION": "main"}, outputFiles: []string{"gitversion.properties"}},
		{name: "bitbucket", environment: map[string]string{"BITBUCKET_WORKSPACE": "task", "BITBUCKET_BRANCH": "refs/heads/main"}, outputFiles: []string{"gitversion.properties", "gitversion.ps1"}},
		{name: "envrun", environment: map[string]string{"ENVRUN_DATABASE": ".git/config"}},
		{name: "azure-no-build-number", environment: map[string]string{"TF_BUILD": "true"}, noUpdate: true},
		{name: "dotenv", dotenv: true},
		{name: "github-without-env-file", environment: map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF_TYPE": "branch", "GITHUB_REF": "refs/heads/main"}},
		{name: "gitlab-no-build-number", environment: map[string]string{"GITLAB_CI": "true", "CI_COMMIT_REF_NAME": "main"}, outputFiles: []string{"gitversion.properties"}, noUpdate: true},
		{name: "selection-precedence", environment: map[string]string{"TF_BUILD": "true", "TEAMCITY_VERSION": "2026.1", "Git_Branch": "main"}},
	}
	passed := 0
	for _, tc := range cases {
		if t.Run(tc.name, func(t *testing.T) {
			config := "workflow: GitHubFlow/v1\n"
			if tc.noUpdate {
				config += "output: {update-build-number: false}\n"
			}
			cps, err := fixtures.Generate([]fixtures.Recipe{{
				ID: "build-agent", Config: config,
				Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}, {Op: "commit", ID: "after"}, {Op: "checkpoint", ID: "head"}},
			}}, filepath.Join(dir, tc.name+"-fixtures"))
			if err != nil {
				t.Fatal(err)
			}
			env := map[string]string{}
			for key, value := range baseline.Spec.Environment {
				env[key] = value
			}
			for key, value := range tc.environment {
				env[key] = value
			}
			mode := "buildserver"
			if tc.dotenv {
				mode = "dotenv"
			}
			args := []string{"{repo}", "--output", mode, "--no-cache", "--no-fetch", "--no-normalize"}
			commands := []parity.Command{
				{Executable: os.Getenv("PARITY_DOTNET"), Arguments: append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, args...)},
				{Executable: candidate, Arguments: slices.Clone(args)},
			}
			type result struct {
				stdout []byte
				files  map[string]string
			}
			run := func(command parity.Command, suffix string) result {
				t.Helper()
				path := filepath.Join(dir, tc.name+"-"+suffix)
				got, err := parity.Run(t.Context(), command, cps[0].Path, path, env, 30*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				if err := saveResult(path, got); err != nil {
					t.Fatal(err)
				}
				if got.ExitCode != 0 {
					t.Fatalf("exit %d, %s", got.ExitCode, got.Stderr)
				}
				files := map[string]string{}
				for _, file := range tc.outputFiles {
					data, err := os.ReadFile(filepath.Join(path, "repo", file))
					if err != nil {
						t.Fatal(err)
					}
					files[file] = string(data)
				}
				return result{got.Stdout, files}
			}
			ref, actual := run(commands[0], "csharp"), run(commands[1], "go")
			if !bytes.Equal(ref.stdout, actual.stdout) || !reflect.DeepEqual(ref.files, actual.files) {
				t.Fatalf("build integration differs\nC# stdout %s\nGo stdout %s\nC# files %v\nGo files %v", ref.stdout, actual.stdout, ref.files, actual.files)
			}
		}) {
			passed++
		}
	}
	if err := parity.WriteJSON(filepath.Join(dir, "report.json"), map[string]any{
		"passed": passed, "failed": len(cases) - passed, "candidate_sha256": hash, "baseline_revision": baseline.Spec.Revision, "spec_sha256": spec,
	}); err != nil {
		t.Fatal(err)
	}
}

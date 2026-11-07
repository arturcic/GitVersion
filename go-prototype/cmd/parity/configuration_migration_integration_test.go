//go:build platform

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformConfigurationMigrationParity(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(filepath.Join(root, ".local"), "configuration-migration-")
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

	const replace = "Comments cannot be preserved during migration."
	type migrationCase struct {
		name, document, diagnostic string
		options                    []string
		exit                       int
		existingOutput             bool
	}
	cases := []migrationCase{
		{name: "empty"},
		{name: "workflow-only", document: "workflow: GitHubFlow/v1"},
		{name: "next-version", document: "next-version: 2.0.0"},
		{name: "branch-split", document: "branches: {main: {increment: Major, pre-release-weight: 42}}"},
		{name: "nested", document: "calculation: {tag-prefix: custom-}\noutput: {update-build-number: false}"},
		{name: "draft-workflow", document: "calculation: {workflow: GitHubFlow/v1, next-version: 2.0.0}"},
		{name: "label", document: "branches: {feature: {label: 'ci.{ShortSha}'}}"},
		{name: "explicit-null", document: "workflow: null\ncalculation: {}"},
		{name: "drop-comments", document: "# heading\nnext-version: 2.0.0 # comment"},
		{name: "in-place", document: "next-version: 2.0.0", options: []string{"--in-place"}, diagnostic: replace},
		{name: "new-output", document: "next-version: 2.0.0", options: []string{"--output", "out.yml"}},
		{name: "existing-output", document: "next-version: 2.0.0", options: []string{"--output", "out.yml"}, exit: 1, existingOutput: true, diagnostic: "already exists. Use --force"},
		{name: "force-output", document: "next-version: 2.0.0", options: []string{"--output", "out.yml", "--force"}, existingOutput: true, diagnostic: replace},
		{name: "invalid-in-place", document: "output: {increment: Major}", options: []string{"--in-place"}, exit: 1, diagnostic: "calculation.increment"},
		{name: "duplicate-workflow", document: "workflow: GitHubFlow/v1\ncalculation: {workflow: GitFlow/v1}", exit: 1, diagnostic: "both the document root and 'calculation.workflow'"},
	}
	type observation struct {
		Stdout, Stderr string
		ExitCode       int
		Files          map[string]string
	}
	passed := 0
	for _, tc := range cases {
		if t.Run(tc.name, func(t *testing.T) {
			run := func(executable string, argv []string) observation {
				t.Helper()
				workdir := t.TempDir()
				if err := os.WriteFile(filepath.Join(workdir, "GitVersion.yml"), []byte(tc.document), 0600); err != nil {
					t.Fatal(err)
				}
				if tc.existingOutput {
					if err := os.WriteFile(filepath.Join(workdir, "out.yml"), []byte("existing"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				args := append(slices.Clone(argv), workdir, "config", "migrate")
				args = append(args, tc.options...)
				command := exec.CommandContext(t.Context(), executable, args...)
				command.Dir = workdir
				command.Env = command.Environ()
				for key, value := range baseline.Spec.Environment {
					command.Env = append(command.Env, key+"="+value)
				}
				var stdout, stderr bytes.Buffer
				command.Stdout, command.Stderr = &stdout, &stderr
				err := command.Run()
				if err != nil && command.ProcessState == nil {
					t.Fatal(err)
				}
				entries, err := os.ReadDir(workdir)
				if err != nil {
					t.Fatal(err)
				}
				after := make(map[string]string, len(entries))
				for _, entry := range entries {
					content, err := os.ReadFile(filepath.Join(workdir, entry.Name()))
					if err != nil {
						t.Fatal(err)
					}
					after[entry.Name()] = string(content)
				}
				return observation{stdout.String(), stderr.String(), command.ProcessState.ExitCode(), after}
			}
			reference := run(os.Getenv("PARITY_DOTNET"), []string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")})
			actual := run(candidate, nil)
			if err := parity.WriteJSON(filepath.Join(dir, tc.name+".json"), map[string]observation{"reference": reference, "go": actual}); err != nil {
				t.Fatal(err)
			}
			if actual.ExitCode != tc.exit || reference.ExitCode != tc.exit || actual.Stdout != reference.Stdout || !reflect.DeepEqual(actual.Files, reference.Files) {
				t.Fatalf("migration differs:\nGo %+v\nC# %+v", actual, reference)
			}
			if tc.diagnostic == "" {
				if actual.Stderr != "" || reference.Stderr != "" {
					t.Fatalf("unexpected stderr: Go %q, C# %q", actual.Stderr, reference.Stderr)
				}
			} else if strings.Count(actual.Stderr, tc.diagnostic) != 1 || !strings.Contains(reference.Stderr, tc.diagnostic) {
				t.Fatalf("diagnostic differs: Go %q, C# %q", actual.Stderr, reference.Stderr)
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

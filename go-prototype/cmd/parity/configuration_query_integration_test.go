//go:build platform

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	git "github.com/go-git/go-git/v5"
)

func TestPlatformConfigurationQueryParity(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(filepath.Join(root, ".local"), "configuration-query-")
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

	type queryCase struct {
		name, document, version, property string
		options                           []string
		exit                              int
	}
	cases := []queryCase{
		{name: "defaults", property: "calculation.tag-prefix"},
		{name: "null-workflow", property: "workflow"},
		{name: "named-workflow", document: "workflow: TrunkBased/preview1", property: "workflow"},
		{name: "boolean", document: "output: {update-build-number: false}", property: "output.update-build-number"},
		{name: "integer-zero", document: "output: {tag-pre-release-weight: 0}", property: "output.tag-pre-release-weight"},
		{name: "string-null", document: "output: {custom-version-format: 'null'}", property: "output.custom-version-format"},
		{name: "quoted-text", document: "output:\n  custom-version-format: \"café \\\"x\\\" <>&+\\x60 \\n 🐙\"", property: "output.custom-version-format"},
		{name: "cutoff-date", document: "calculation: {ignore: {commits-before: '2026-01-02T03:04:05+02:00'}}", property: "calculation.ignore.commits-before"},
		{name: "override", document: "calculation: {tag-prefix: file-}", property: "calculation.tag-prefix", options: []string{"--override-config", "calculation.tag-prefix=runtime-"}},
		{name: "override-zero", document: "output: {tag-pre-release-weight: 15}", property: "output.tag-pre-release-weight", options: []string{"--override-config", "output.tag-pre-release-weight=0"}},
		{name: "unknown", property: "calculation.missing", exit: 1},
		{name: "malformed", property: "calculation..tag-prefix", exit: 1},
		{name: "dynamic", property: "calculation.branches.main.increment", exit: 1},
		{name: "v6-query", document: "invalid: [", version: "v6", property: "workflow", exit: 1},
		{name: "invalid-selector", document: "invalid: [", version: "invalid", property: "workflow", exit: 1},
		{name: "default-legacy-display", version: "v6"},
		{name: "default-display"},
		{name: "v7-display", options: []string{"--override-config", "calculation.tag-prefix=runtime-"}},
		{name: "v6-display", version: "v6", options: []string{"--override-config", "tag-prefix=ignored-"}},
	}
	type observation struct {
		Stdout, Stderr string
		ExitCode       int
	}
	passed := 0
	for _, tc := range cases {
		if t.Run(tc.name, func(t *testing.T) {
			workdir := t.TempDir()
			if _, err := git.PlainInit(workdir, false); err != nil {
				t.Fatal(err)
			}
			if tc.document != "" {
				if err := os.WriteFile(filepath.Join(workdir, "GitVersion.yml"), []byte(tc.document), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := parity.FileSet(workdir)
			if err != nil {
				t.Fatal(err)
			}
			args := []string{workdir, "--show-config"}
			if tc.property != "" {
				args = []string{workdir, "config", "get", tc.property}
			}
			args = append(args, tc.options...)
			run := func(executable string, argv []string) observation {
				t.Helper()
				command := exec.CommandContext(t.Context(), executable, argv...)
				command.Dir = workdir
				command.Env = command.Environ()
				for key, value := range baseline.Spec.Environment {
					command.Env = append(command.Env, key+"="+value)
				}
				version := tc.version
				if version == "" {
					version = "v7"
				}
				command.Env = append(command.Env, "GITVERSION_CONFIGURATION_VERSION="+version)
				var stdout, stderr bytes.Buffer
				command.Stdout, command.Stderr = &stdout, &stderr
				err := command.Run()
				if err != nil && command.ProcessState == nil {
					t.Fatal(err)
				}
				return observation{stdout.String(), stderr.String(), command.ProcessState.ExitCode()}
			}
			reference := run(os.Getenv("PARITY_DOTNET"), append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, args...))
			actual := run(candidate, args)
			if err := parity.WriteJSON(filepath.Join(dir, tc.name+".json"), map[string]observation{"reference": reference, "go": actual}); err != nil {
				t.Fatal(err)
			}
			if reference.ExitCode != tc.exit || actual != reference {
				t.Fatalf("query/display differs: exit %d/%d, stdout equal %t, stderr Go %q C# %q", actual.ExitCode, reference.ExitCode, actual.Stdout == reference.Stdout, actual.Stderr, reference.Stderr)
			}
			after, err := parity.FileSet(workdir)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("query modified inputs", err)
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

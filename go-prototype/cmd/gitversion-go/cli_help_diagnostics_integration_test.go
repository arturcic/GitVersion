//go:build platform

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformCLIHelpAndDiagnosticsParity(t *testing.T) {
	h := newRemoteCIHarness(t)
	type result struct {
		stdout, stderr string
		exit           int
	}
	run := func(t *testing.T, implementation, directory string, args ...string) result {
		t.Helper()
		area := t.TempDir()
		env := maps.Clone(h.baseline.Spec.Environment)
		exe := h.candidate
		if implementation == "csharp" {
			exe = h.exe
			args = append([]string{"--fx-version", h.baseline.Spec.Runtime, h.dll}, args...)
		} else {
			env["PATH"] = area
		}
		cmd := exec.CommandContext(t.Context(), exe, args...)
		cmd.Dir, cmd.Env = directory, parity.Environment(area, env)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		var r result
		if err := cmd.Run(); err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatal(err)
			}
			r.exit = exit.ExitCode()
		}
		r.stdout, r.stderr = stdout.String(), stderr.String()
		if err := parity.WriteJSON(filepath.Join(h.report, filepath.Base(t.Name())+"-"+implementation+"-"+strings.TrimLeft(args[len(args)-1], "-")+".json"), map[string]any{"stdout": r.stdout, "stderr": r.stderr, "exit": r.exit}); err != nil {
			t.Fatal(err)
		}
		return r
	}

	t.Run("help", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			args []string
		}{
			{"root", []string{"--help"}},
			{"config", []string{"config", "--help"}},
			{"get", []string{"config", "get", "--help"}},
			{"migrate", []string{"config", "migrate", "--help"}},
			{"validate", []string{"config", "validate", "--help"}},
			{"short-alias", []string{"config", "get", "-h"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				dir := t.TempDir()
				want := run(t, "csharp", dir, tc.args...)
				got := run(t, "go", dir, tc.args...)
				if want.exit != 0 || want.stderr != "" || want.stdout == "" || got != want {
					t.Fatalf("help did not match C#\nC#: %+v\nGo: %+v", want, got)
				}
				files, err := os.ReadDir(dir)
				if err != nil || len(files) != 0 {
					t.Fatalf("help modified the working directory: %v (%v)", files, err)
				}
			})
		}
	})
	t.Run("version", func(t *testing.T) {
		dir := t.TempDir()
		version := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.+-]+)?\r?\n$`)
		for _, implementation := range []string{"csharp", "go"} {
			got := run(t, implementation, dir, "--version")
			schema := run(t, implementation, dir, "--cli-schema")
			var document cliSchemaDocument
			if err := json.Unmarshal([]byte(schema.stdout), &document); err != nil ||
				schema.exit != 0 || schema.stderr != "" || got.exit != 0 || got.stderr != "" ||
				!version.MatchString(got.stdout) || strings.TrimSpace(got.stdout) != document.Info.Version {
				t.Fatalf("%s: --version did not match executable schema: %+v (%v)", implementation, got, err)
			}
		}
	})
	t.Run("invalid-query-verbosity", func(t *testing.T) {
		dir := t.TempDir()
		args := []string{"config", "get", "calculation.tag-prefix", "--verbosity", "bogus"}
		want, got := run(t, "csharp", dir, args...), run(t, "go", dir, args...)
		if want.exit != 1 || want.stdout != "" || !strings.Contains(want.stderr, "Could not parse Verbosity value 'bogus'") || got != want {
			t.Fatalf("verbosity rejection differed: C# %+v, Go %+v", want, got)
		}
	})
	t.Run("diagnose", func(t *testing.T) {
		dir := t.TempDir()
		prepareExecutorCacheRepo(t, dir)
		args := []string{"--diagnose", "--log-file", "console", "--no-fetch", "--no-normalize", "--show-variable", "FullSemVer"}
		want, got := run(t, "csharp", dir, args...), run(t, "go", dir, args...)
		if want.exit != 0 || got.exit != 0 || want.stdout == "" || want.stdout != got.stdout ||
			!strings.Contains(want.stderr, "Please run `git log --graph") || !strings.Contains(got.stderr, "Please run `git log --graph") {
			t.Fatalf("diagnostic calculation differed: C# %+v, Go %+v", want, got)
		}
		cache, err := os.ReadDir(filepath.Join(dir, ".git", "gitversion_cache"))
		if len(cache) != 0 || (err != nil && !errors.Is(err, os.ErrNotExist)) {
			t.Fatalf("diagnostic calculation wrote cache entries: %v (%v)", cache, err)
		}
	})
}

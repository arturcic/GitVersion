//go:build platform

package main

import (
	"bytes"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformCLILoggingParity(t *testing.T) {
	h := newRemoteCIHarness(t)
	root := t.TempDir()
	repository := filepath.Join(root, "repo")
	if err := os.Mkdir(repository, 0755); err != nil {
		t.Fatal(err)
	}
	prepareExecutorCacheRepo(t, repository)
	tagged := filepath.Join(root, "tagged")
	if err := os.Mkdir(tagged, 0755); err != nil {
		t.Fatal(err)
	}
	prepareExecutorCacheRepo(t, tagged)
	repo, err := git.PlainOpen(tagged)
	if err != nil {
		t.Fatal(err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateTag("1.2.3", head.Hash(), nil); err != nil {
		t.Fatal(err)
	}
	unborn := filepath.Join(root, "unborn")
	if _, err := git.PlainInit(unborn, false); err != nil {
		t.Fatal(err)
	}
	configuration := filepath.Join(root, "valid.yml")
	legacy := filepath.Join(root, "legacy.yml")
	for path, data := range map[string]string{
		configuration: "calculation: {}\noutput: {}\n",
		legacy:        "next-version: 1.2.3\n",
	} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	const logFile = "logs/gitversion.log"
	calc := []string{repository, "--no-fetch", "--no-normalize", "--no-cache", "--show-variable", "FullSemVer"}
	query := []string{"config", "get", "calculation.tag-prefix", "--config", configuration, "--target-path", repository}
	migrate := []string{"config", "migrate", "--config", legacy}
	startup := "INFO Argument parser version: v7; Configuration version: v7; Git backend: managed\n"
	local := "INFO Applicable build agent found: 'LocalBuild'.\n"
	type result struct {
		Stdout, Stderr, File string
		Exit                 int
		FileExists           bool
	}
	timestamp := regexp.MustCompile(`(?m)^([A-Z]{4}) \[\d{2}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}:\d{2}\] `)
	duration := regexp.MustCompile(`\(Took: [0-9.]+ms\)`)
	normalize := func(log string) string {
		log = duration.ReplaceAllString(log, "(Took: elapsed)")
		return timestamp.ReplaceAllString(log, "${1} ")
	}

	for _, tc := range []struct {
		name      string
		args      []string
		exact     bool
		prefix    string
		contains  string
		file      bool
		sameSinks bool
		seed      string
		exit      int
	}{
		{name: "default", args: calc, exact: true},
		{name: "tagged-console", args: []string{tagged, "--no-normalize", "--no-cache", "--show-variable", "FullSemVer", "--verbosity", "diagnostic", "--log-file", "console"}, exact: true, contains: "Returning cached tagged semantic versions."},
		{name: "tagged-file", args: []string{tagged, "--no-normalize", "--no-cache", "--show-variable", "FullSemVer", "--verbosity", "diagnostic", "--log-file", logFile}, exact: true, contains: `and Format: "Strict"`, file: true},
		{name: "show-config", args: []string{repository, "--show-config", "--log-file", "console"}, exact: true},
		{name: "normal-console", args: append(slices.Clone(calc), "--log-file", "CoNsOlE"), prefix: startup + local},
		{name: "verbose-console", args: append(slices.Clone(calc), "--verbosity", "verbose", "--log-file", "console"), prefix: startup + local, contains: "DBUG Trying to find configuration file"},
		{name: "diagnostic-console", args: append(slices.Clone(calc), "--verbosity", "diagnostic", "--log-file", "console"), prefix: startup + local, contains: "DBUG Trying to find configuration file"},
		{name: "minimal-console", args: append(slices.Clone(calc), "--verbosity", "minimal", "--log-file", "console"), exact: true},
		{name: "quiet-console", args: append(slices.Clone(calc), "--verbosity", "quiet", "--log-file", "console"), exact: true},
		{name: "file-relative-to-cwd", args: append(slices.Clone(calc), "--log-file", logFile), prefix: startup + local, file: true},
		{name: "quiet-creates-empty-file", args: append(slices.Clone(calc), "--verbosity", "quiet", "--log-file", logFile), exact: true, file: true},
		{name: "append-to-file", args: append(slices.Clone(calc), "--log-file", logFile), prefix: "previous invocation\n" + startup + local, file: true, seed: "previous invocation\n"},
		{name: "root-argument-error", args: append(slices.Clone(calc), "--invalid-flag", "--log-file", logFile), exact: true, exit: 1},
		{name: "root-verbosity-error", args: append(slices.Clone(calc), "--verbosity", "what", "--log-file", logFile), exact: true, exit: 1},
		{name: "invalid-file-sink", args: append(slices.Clone(calc), "--log-file", repository), exact: true},
		{name: "buildserver-console", args: []string{repository, "--no-cache", "--no-normalize", "--output", "buildserver"}, prefix: startup + local},
		{name: "buildserver-both-sinks", args: []string{repository, "--no-cache", "--no-normalize", "--output", "buildserver", "--log-file", logFile}, prefix: startup + local, file: true, sameSinks: true},
		{name: "unborn-no-sink", args: []string{unborn, "--no-cache"}, exact: true, exit: 1},
		{name: "unborn-console", args: []string{unborn, "--no-cache", "--log-file", "console"}, prefix: startup + local, contains: "EROR An unexpected error occurred:\nNo commits found on the current branch.", exit: 1},
		{name: "invalid-config", args: append(slices.Clone(calc), "--config", legacy, "--log-file", "console"), prefix: startup + local, contains: "\nAn error occurred:\nThe configuration file uses the legacy v6 configuration structure", exit: 1},
		{name: "missing-directory", args: []string{filepath.Join(root, "missing"), "--log-file", "console"}, exact: true, exit: 1},
		{name: "missing-directory-minimal", args: []string{filepath.Join(root, "missing"), "--verbosity", "minimal", "--log-file", logFile}, exact: true, file: true, exit: 1},
		{name: "missing-commit-warning", args: append(slices.Clone(calc), "--commit", "invalid", "--log-file", logFile), file: true, contains: "WARN Commit 'invalid' specified but not found"},
		{name: "missing-commit-quiet", args: append(slices.Clone(calc), "--commit", "invalid", "--verbosity", "quiet"), exact: true},
		{name: "diagnose", args: append(slices.Clone(calc), "--diagnose", "--log-file", "console"), prefix: startup + local + "INFO " + gitGraphHint + "\n"},
		{name: "config-get", args: append(slices.Clone(query), "--verbosity", "diagnostic", "--log-file", "console"), exact: true},
		{name: "config-get-missing-path", args: []string{"config", "get", "calculation.tag-prefix", "--target-path", filepath.Join(root, "missing"), "--log-file", logFile}, exact: true, exit: 1},
		{name: "config-get-quiet", args: append(slices.Clone(query), "--verbosity", "quiet", "--log-file", "console"), exact: true},
		{name: "config-get-failure", args: []string{"config", "get", "missing.property", "--config", configuration, "--target-path", repository, "--log-file", "console"}, exact: true, exit: 1},
		{name: "migration-root-log-option", args: append([]string{"--log-file", "console"}, migrate...), exact: true},
		{name: "migration-ignores-root-verbosity", args: append([]string{"--verbosity", "quiet", "--log-file", "console"}, migrate...), exact: true, prefix: startup},
		{name: "migration-missing-input", args: []string{"--log-file", "console", "config", "migrate", "--config", filepath.Join(root, "missing.yml")}, exact: true, prefix: startup, exit: 1},
		{name: "migration-local-log-option-rejected", args: append(slices.Clone(migrate), "--log-file", "console"), exact: true, exit: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			run := func(csharp bool) result {
				t.Helper()
				area := t.TempDir()
				path := filepath.Join(directory, logFile)
				if tc.seed != "" {
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, nativeText([]byte(tc.seed)), 0600); err != nil {
						t.Fatal(err)
					}
				}
				env := maps.Clone(h.baseline.Spec.Environment)
				exe, args, impl := h.candidate, tc.args, "go"
				if csharp {
					exe, args, impl = h.exe, append([]string{"--fx-version", h.baseline.Spec.Runtime, h.dll}, args...), "csharp"
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
					r.Exit = exit.ExitCode()
				}
				r.Stdout, r.Stderr = stdout.String(), stderr.String()
				log, err := os.ReadFile(path)
				if err == nil {
					r.File, r.FileExists = string(log), true
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				} else if !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
				if err := parity.WriteJSON(filepath.Join(h.report, tc.name+"-"+impl+".json"), r); err != nil {
					t.Fatal(err)
				}
				if tc.sameSinks && strings.ReplaceAll(r.File, `and Format: "Strict"`, "and Format: Strict") != r.Stderr {
					t.Fatalf("%s console and file transcripts differ", impl)
				}
				r.Stderr, r.File = normalize(r.Stderr), normalize(r.File)
				return r
			}

			want, got := run(true), run(false)
			if got.Exit != tc.exit || want.Exit != tc.exit || got.Stdout != want.Stdout ||
				got.FileExists != tc.file || want.FileExists != tc.file {
				t.Fatalf("exit, stdout or file presence differs\nC#: %+v\nGo: %+v", want, got)
			}
			if tc.exact && got != want {
				t.Fatalf("normalized transcript differs\nC#: %+v\nGo: %+v", want, got)
			}
			for _, r := range []result{want, got} {
				stream := r.Stderr
				if tc.file {
					stream = r.File
				}
				contains := tc.contains
				if tc.name != "unborn-console" {
					contains = string(nativeText([]byte(contains)))
				}
				if !strings.HasPrefix(stream, string(nativeText([]byte(tc.prefix)))) || !strings.Contains(stream, contains) {
					t.Fatalf("expected prefix %q and message %q, got %q", tc.prefix, tc.contains, stream)
				}
			}
			if tc.name == "missing-commit-warning" && want.Stderr != got.Stderr {
				t.Fatalf("warning-only stderr differs: C# %q, Go %q", want.Stderr, got.Stderr)
			}
			if tc.file && !tc.sameSinks && tc.exit == 0 && tc.name != "missing-commit-warning" && (want.Stderr != "" || got.Stderr != "") {
				t.Fatal("file logging leaked into stderr")
			}
		})
	}
}

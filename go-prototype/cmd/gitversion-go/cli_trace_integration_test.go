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
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformCLITraceParity(t *testing.T) {
	h := newRemoteCIHarness(t)
	root := t.TempDir()
	source := filepath.Join(root, "source")
	prepareExecutorCacheRepo(t, source)
	tagHistory := makeCLIHistory(t, root, "1.2.3", false)[0].Path
	config := filepath.Join(root, "next.yml")
	if err := os.WriteFile(config, []byte("calculation:\n  next-version: 2.0.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	mainline := filepath.Join(root, "mainline.yml")
	if err := os.WriteFile(mainline, []byte("calculation: {strategies: [Mainline]}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	type transcript struct {
		Stdout, Stderr string
		Exit           int
	}
	ts := regexp.MustCompile(`(?m)^([A-Z]{4}) \[\d{2}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}:\d{2}\] `)
	elapsed := regexp.MustCompile(`\(Took: [0-9.]+ms\)`)
	cacheKey := regexp.MustCompile(`gitversion_cache/(?:[A-F0-9]{40}|go-v1-[a-f0-9]{64})`)
	all := map[string]map[string]transcript{}
	for _, implementation := range []string{"csharp", "go"} {
		dir := t.TempDir()
		checkout := filepath.Join(dir, "repo")
		if err := fixtures.Copy(source, checkout); err != nil {
			t.Fatal(err)
		}
		all[implementation] = map[string]transcript{}
		run := func(label string, environment map[string]string, args ...string) {
			t.Helper()
			env := maps.Clone(h.baseline.Spec.Environment)
			maps.Copy(env, environment)
			exe := h.candidate
			common := []string{checkout, "--show-variable", "FullSemVer"}
			if label != "clone-no-sink-exception" {
				common = append(common, "--log-file", "console")
			}
			args = append(common, args...)
			if implementation == "csharp" {
				exe = h.exe
				args = append([]string{"--fx-version", h.baseline.Spec.Runtime, h.dll}, args...)
			} else {
				env["PATH"] = dir
			}
			cmd := exec.CommandContext(t.Context(), exe, args...)
			cmd.Dir, cmd.Env = checkout, parity.Environment(dir, env)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			r := transcript{}
			if err := cmd.Run(); err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				r.Exit = exit.ExitCode()
			}
			r.Stdout, r.Stderr = stdout.String(), stderr.String()
			if err := parity.WriteJSON(filepath.Join(h.report, implementation+"-"+label+".json"), r); err != nil {
				t.Fatal(err)
			}
			r.Stderr = ts.ReplaceAllString(r.Stderr, "${1} ")
			r.Stderr = elapsed.ReplaceAllString(r.Stderr, "(Took: elapsed)")
			r.Stderr = cacheKey.ReplaceAllString(r.Stderr, "gitversion_cache/<key>")
			r.Stderr = strings.ReplaceAll(r.Stderr, dir, "<fixture>")
			all[implementation][label] = r
		}
		run("fallback", nil, "--no-cache")
		run("configured-next", nil, "--no-cache", "--config", config)
		run("tagged-strategy", nil, "--no-cache", "--target-path", tagHistory)
		run("mainline", nil, "--no-cache", "--config", mainline)
		run("cache-miss", nil)
		run("cache-hit", nil)
		cacheDir := filepath.Join(checkout, ".git", "gitversion_cache")
		entries, err := os.ReadDir(cacheDir)
		if err != nil || len(entries) != 1 {
			t.Fatalf("%s cache entries %v: %v", implementation, entries, err)
		}
		cachePath := filepath.Join(cacheDir, entries[0].Name())
		if err := os.WriteFile(cachePath, []byte("broken"), 0600); err != nil {
			t.Fatal(err)
		}
		run("cache-corrupt", nil)
		if raw, err := os.ReadFile(cachePath); err != nil || !bytes.Contains(raw, []byte("FullSemVer")) {
			t.Fatalf("%s cache was not repaired: %v", implementation, err)
		}
		run("clone", nil, "--no-cache", "--url", source, "--branch", "main", "--dynamic-repo-location", filepath.Join(dir, "clones"))
		run("clone-repeat", nil, "--no-cache", "--url", source, "--branch", "main", "--dynamic-repo-location", filepath.Join(dir, "clones"))
		brokenClone := []string{"--no-cache", "--url", filepath.Join(root, "missing.git"), "--branch", "main", "--dynamic-repo-location", filepath.Join(dir, "clones")}
		run("clone-exception", nil, brokenClone...)
		run("clone-quiet-exception", nil, append(brokenClone, "--verbosity", "quiet")...)
		run("clone-no-sink-exception", nil, brokenClone...)
		repo, err := git.PlainOpen(checkout)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{source}}); err != nil {
			t.Fatal(err)
		}
		run("ci-skip", map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF": "refs/heads/main"}, "--no-cache")
		run("ci-fetch", map[string]string{"APPVEYOR": "True", "APPVEYOR_REPO_BRANCH": "main"}, "--no-cache")
	}

	for _, tc := range []struct {
		name    string
		exact   bool
		failure bool
		records []string
	}{
		{"fallback", true, false, []string{"[Using 'FallbackVersionStrategy' strategy]", "Fallback base version: Version increment"}},
		{"configured-next", true, false, []string{"[Using 'ConfiguredNextVersionVersionStrategy' strategy]", "NextVersion in GitVersion configuration file: Version increment"}},
		{"tagged-strategy", true, false, []string{"[Using 'TaggedCommitVersionStrategy' strategy]", "Git tag '1.2.3': Version increment"}},
		{"mainline", false, false, []string{"Begin: [Using 'MainlineVersionStrategy' strategy]", "+semver 'Patch' with label", "End: [Using 'MainlineVersionStrategy' strategy]", "Commit count source: All reachable history"}},
		{"cache-miss", true, false, []string{"Begin: Loading version variables", "not found.", "Configuration version:", "Begin: Write version variables to cache file"}},
		{"cache-hit", true, false, []string{"Begin: Loading version variables", "End: Loading version variables", "Configuration version:"}},
		{"cache-corrupt", false, false, []string{"Begin: Loading version variables", "EROR │   Unable to read cache file", "End: Loading version variables", "Begin: Write version variables"}},
		{"clone", false, false, []string{"Begin: Creating dynamic repository", "Begin: Cloning repository from url", "Cloned repository", "End: Creating dynamic repository", "Begin: Normalizing git directory for branch 'main'"}},
		{"clone-repeat", false, false, []string{"Begin: Creating dynamic repository", "End: Creating dynamic repository", "Begin: Normalizing git directory for branch 'main'"}},
		{"clone-exception", false, true, []string{"Begin: Creating dynamic repository", "End: Cloning repository from url", "End: Creating dynamic repository", "EROR An unexpected error occurred:", "Please run `git log --graph"}},
		{"clone-quiet-exception", false, true, []string{"EROR An unexpected error occurred:"}},
		{"clone-no-sink-exception", true, true, nil},
		{"ci-skip", false, false, []string{"Begin: Normalizing git directory for branch 'refs/heads/main'", "One remote found (origin", "Skipping fetching", "HEAD points at branch 'refs/heads/main'.", "End: Normalizing git directory"}},
		{"ci-fetch", false, false, []string{"Begin: Normalizing git directory for branch 'main'", "One remote found (origin", "Fetching from remote 'origin' using the following refspecs:", "HEAD points at branch 'refs/heads/main'.", "End: Normalizing git directory"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want, got := all["csharp"][tc.name], all["go"][tc.name]
			exit := 0
			if tc.failure {
				exit = 1
			}
			if want.Exit != exit || got.Exit != exit || got.Stdout != want.Stdout {
				t.Fatalf("process parity differs\nC#: %+v\nGo: %+v", want, got)
			}
			if tc.name == "clone-quiet-exception" {
				for _, r := range []transcript{want, got} {
					if !strings.HasPrefix(r.Stderr, "EROR An unexpected error occurred:") || strings.Contains(r.Stderr, "INFO ") {
						t.Fatalf("quiet exception leaked informational logs: %q", r.Stderr)
					}
				}
			}
			if tc.exact && got.Stderr != want.Stderr {
				t.Fatalf("normalized transcript differs\nC#:\n%s\nGo:\n%s", want.Stderr, got.Stderr)
			}
			for _, pair := range []struct{ implementation, log string }{{"csharp", want.Stderr}, {"go", got.Stderr}} {
				rest := pair.log
				for _, record := range tc.records {
					_, after, found := strings.Cut(rest, record)
					if !found {
						t.Fatalf("%s missing ordered record %q\n%s", pair.implementation, record, pair.log)
					}
					rest = after
				}
			}
		})
	}
}

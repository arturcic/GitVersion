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
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformCLITraceParity(t *testing.T) {
	h := newRemoteCIHarness(t)
	root := t.TempDir()
	source := filepath.Join(root, "source")
	prepareExecutorCacheRepo(t, source)
	tagCheckpoint := makeCLIHistory(t, root, "1.2.3", false)[0]
	tagHistory := tagCheckpoint.Path
	complex, err := fixtures.Generate([]fixtures.Recipe{
		{ID: "merged-release", Config: "workflow: GitFlow/v1\n", Steps: []fixtures.Step{
			{Op: "commit", ID: "root"}, {Op: "branch", Name: "release/2.0.0"},
			{Op: "checkout", Name: "release/2.0.0"}, {Op: "commit", ID: "release"},
			{Op: "checkout", Name: "main"}, {Op: "commit", ID: "work"},
			{Op: "merge", ID: "merged", From: "release/2.0.0", Message: "Merge branch 'release/2.0.0'"},
			{Op: "checkpoint", ID: "head"},
		}},
		{ID: "named-release", Config: "workflow: GitFlow/v1\n", Steps: []fixtures.Step{
			{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"},
			{Op: "branch", Name: "develop"}, {Op: "checkout", Name: "develop"}, {Op: "commit", ID: "work"},
			{Op: "branch", Name: "release/1.3.0"}, {Op: "checkpoint", ID: "head"},
		}},
		{ID: "tagged-release", Config: "workflow: GitFlow/v1\n", Steps: []fixtures.Step{
			{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"},
			{Op: "branch", Name: "develop"}, {Op: "checkout", Name: "develop"}, {Op: "commit", ID: "work"},
			{Op: "branch", Name: "release/current"}, {Op: "checkout", Name: "release/current"},
			{Op: "commit", ID: "release"}, {Op: "tag", Name: "2.0.0-beta.1"},
			{Op: "checkout", Name: "develop"}, {Op: "checkpoint", ID: "head"},
		}},
		{ID: "mainline-merge", Config: "workflow: GitHubFlow/v1\ncalculation: {strategies: [Mainline]}\n", Steps: []fixtures.Step{
			{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.0"},
			{Op: "branch", Name: "feature/topic"}, {Op: "checkout", Name: "feature/topic"},
			{Op: "commit", ID: "feature", Message: "feature +semver: minor"},
			{Op: "checkout", Name: "main"}, {Op: "commit", ID: "work"},
			{Op: "merge", ID: "merged", From: "feature/topic", Message: "Merge branch 'feature/topic'"},
			{Op: "checkpoint", ID: "head"},
		}},
		{ID: "pruned-tags", Steps: []fixtures.Step{
			{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.0"},
			{Op: "commit", ID: "second"}, {Op: "tag", Name: "3.0.0"},
			{Op: "commit", ID: "head"}, {Op: "checkpoint", ID: "head"},
		}},
		{ID: "tied-tags", Steps: []fixtures.Step{
			{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"},
			{Op: "commit", ID: "second"}, {Op: "tag", Name: "v1.2.3"},
			{Op: "commit", ID: "head"}, {Op: "checkpoint", ID: "head"},
		}},
	}, filepath.Join(root, "complex"))
	if err != nil {
		t.Fatal(err)
	}
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
		for _, cp := range complex {
			run(cp.Recipe, nil, "--no-cache", "--target-path", cp.Path)
		}
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

		ci := filepath.Join(dir, "ci-refs")
		if err := fixtures.Copy(tagHistory, ci); err != nil {
			t.Fatal(err)
		}
		ciRepo, err := git.PlainOpen(ci)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ciRepo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{source}}); err != nil {
			t.Fatal(err)
		}
		head, err := ciRepo.Head()
		if err != nil {
			t.Fatal(err)
		}
		commit, err := ciRepo.CommitObject(head.Hash())
		if err != nil {
			t.Fatal(err)
		}
		old := commit.ParentHashes[0]
		refs := map[string]plumbing.Hash{
			"refs/remotes/origin/a-new": head.Hash(),
			"refs/heads/b-stale":        old, "refs/remotes/origin/b-stale": head.Hash(),
			"refs/heads/c-same": head.Hash(), "refs/remotes/origin/c-same": head.Hash(),
			"refs/remotes/origin/d-ignored": head.Hash(), "refs/remotes/origin/main": old,
		}
		for name, sha := range refs {
			if err := ciRepo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(name), sha)); err != nil {
				t.Fatal(err)
			}
		}
		ciConfig := filepath.Join(ci, "GitVersion.yml")
		if err := os.WriteFile(ciConfig, []byte("calculation:\n  ignore:\n    branches: ['^d-ignored$']\n"), 0600); err != nil {
			t.Fatal(err)
		}
		before, err := fixtures.State(ci)
		if err != nil {
			t.Fatal(err)
		}
		run("ci-ref-decisions", map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF": "refs/heads/main"}, "--no-cache", "--target-path", ci)
		after, err := fixtures.State(ci)
		if err != nil {
			t.Fatal(err)
		}
		if (before == after) != (implementation == "go") {
			t.Fatalf("%s unexpected normalization effects: before %s, after %s", implementation, before, after)
		}
		checkRef := func(name string, expected plumbing.Hash) {
			t.Helper()
			ref, err := ciRepo.Storer.Reference(plumbing.NewBranchReferenceName(name))
			if expected.IsZero() {
				if !errors.Is(err, plumbing.ErrReferenceNotFound) {
					t.Fatalf("%s should leave branch %s absent, got %v (%v)", implementation, name, ref, err)
				}
			} else if err != nil || ref.Hash() != expected {
				t.Fatalf("%s %s should point at %s, got %v (%v)", implementation, name, expected, ref, err)
			}
		}
		checkRef("main", head.Hash())
		checkRef("c-same", head.Hash())
		checkRef("d-ignored", plumbing.ZeroHash)
		if implementation == "go" {
			checkRef("a-new", plumbing.ZeroHash)
			checkRef("b-stale", old)
		} else {
			checkRef("a-new", head.Hash())
			checkRef("b-stale", head.Hash())
		}
		run("ci-context-skips-projection", map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF": "refs/heads/main", "GIT_BRANCH": "main"}, "--no-cache", "--target-path", ci)
		contextState, err := fixtures.State(ci)
		if err != nil || after != contextState {
			t.Fatalf("%s contextual calculation changed refs: %v", implementation, err)
		}
		if err := ciRepo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, head.Hash())); err != nil {
			t.Fatal(err)
		}
		run("ci-detached-selection", map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF": "refs/heads/main"}, "--no-cache", "--target-path", ci)
		actualHead, err := ciRepo.Storer.Reference(plumbing.HEAD)
		if err != nil {
			t.Fatal(err)
		}
		if implementation == "go" {
			if actualHead.Type() != plumbing.HashReference || actualHead.Hash() != head.Hash() {
				t.Fatalf("Go should preserve detached HEAD: %v", actualHead)
			}
		} else if actualHead.Type() != plumbing.SymbolicReference || actualHead.Target() != plumbing.NewBranchReferenceName("main") {
			t.Fatalf("C# should attach HEAD to main: %v", actualHead)
		}
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
		{"mainline", true, false, []string{"Begin: [Using 'MainlineVersionStrategy' strategy]", "Getting tagged semantic versions on branch 'refs/heads/main'", "Finding branches source of 'main'", "CommitOnTrunk: Force version increment '0.0.0' +semver 'Patch' with label", "End: [Using 'MainlineVersionStrategy' strategy]", "Commit count source: All reachable history"}},
		{"merged-release", true, false, []string{"Begin: [Using 'MergeMessageVersionStrategy' strategy]", "matching merge message format: Default", "Merge message 'Merge branch 'release/2.0.0'': Version increment", "Semantic version source: Merge message 'Merge branch 'release/2.0.0''"}},
		{"named-release", true, false, []string{"Begin: [Using 'TrackReleaseBranchesVersionStrategy' strategy]", "Release branch exists -> Version in branch name: Version increment", "Semantic version source: Version in branch name"}},
		{"tagged-release", true, false, []string{"Release branch exists -> Version in release branch tag '2.0.0-beta.1': Version increment", "Semantic version source: Git tag '2.0.0-beta.1', version 2.0.0,"}},
		{"mainline-merge", true, false, []string{"Begin: [Using 'MainlineVersionStrategy' strategy]", "Getting tagged semantic versions on branch 'refs/heads/main'", "Finding branches source of 'main'", "Finding merge base between 'feature/topic' and 'main'.", "LastMergeCommitOnTrunk: Force version increment", "End: [Using 'MainlineVersionStrategy' strategy]"}},
		{"pruned-tags", true, false, []string{"Git tag '3.0.0': Version increment", "The tag '1.0.0' is skipped because it provides a lower base version than other tags.", "Semantic version source: Git tag '3.0.0'"}},
		{"tied-tags", true, false, []string{"Git tag 'v1.2.3': Version increment", "Found multiple base versions which will produce the same SemVer", "taking latest source for commit counting (Git tag 'v1.2.3')"}},
		{"cache-miss", true, false, []string{"Begin: Loading version variables", "not found.", "Configuration version:", "Begin: Write version variables to cache file"}},
		{"cache-hit", true, false, []string{"Begin: Loading version variables", "End: Loading version variables", "Configuration version:"}},
		{"cache-corrupt", false, false, []string{"Begin: Loading version variables", "EROR │   Unable to read cache file", "End: Loading version variables", "Begin: Write version variables"}},
		{"clone", false, false, []string{"Begin: Creating dynamic repository", "Begin: Cloning repository from url", "Cloned repository", "End: Creating dynamic repository", "Begin: Normalizing git directory for branch 'main'"}},
		{"clone-repeat", false, false, []string{"Begin: Creating dynamic repository", "End: Creating dynamic repository", "Begin: Normalizing git directory for branch 'main'"}},
		{"clone-exception", false, true, []string{"Begin: Creating dynamic repository", "End: Cloning repository from url", "End: Creating dynamic repository", "EROR An unexpected error occurred:", "Please run `git log --graph"}},
		{"clone-quiet-exception", false, true, []string{"EROR An unexpected error occurred:"}},
		{"clone-no-sink-exception", true, true, nil},
		{"ci-skip", false, false, []string{"Begin: Normalizing git directory for branch 'refs/heads/main'", "One remote found (origin", "Skipping fetching", "HEAD points at branch 'refs/heads/main'.", "End: Normalizing git directory"}},
		{"ci-ref-decisions", false, false, []string{"Begin: Normalizing git directory", "Creating local branch from remote tracking 'refs/remotes/origin/a-new'", "Updating local ref 'refs/heads/b-stale' to point at", "Skipping update of 'refs/remotes/origin/c-same' as it already matches the remote ref.", "Skipping ignored remote tracking branch 'refs/remotes/origin/d-ignored'.", "HEAD points at branch 'refs/heads/main'.", "End: Normalizing git directory"}},
		{"ci-context-skips-projection", true, false, []string{"Branch input: null; contextual override: main", "Begin: Normalizing git directory for branch ''", "Skipping fetching", "End: Normalizing git directory"}},
		{"ci-detached-selection", false, false, []string{"Begin: Normalizing git directory for branch 'refs/heads/main'", "End: Normalizing git directory", "Running against branch: main"}},
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
			if tc.name == "ci-detached-selection" {
				if !strings.Contains(want.Stderr, "Updating local branch main to point at "+tagCheckpoint.Head[:7]) ||
					!strings.Contains(want.Stderr, "HEAD points at branch 'refs/heads/main'.") ||
					!strings.Contains(got.Stderr, "HEAD is detached and points at commit '"+tagCheckpoint.Head+"'.") ||
					!strings.Contains(got.Stderr, "Selecting local branch 'refs/heads/main' at "+tagCheckpoint.Head[:7]+" (calculation view).") {
					t.Fatalf("missing selected branch trace\nC#: %s\nGo: %s", want.Stderr, got.Stderr)
				}
			}
			if tc.name == "ci-context-skips-projection" {
				for _, r := range []transcript{want, got} {
					if strings.Contains(r.Stderr, "Creating local branch") || strings.Contains(r.Stderr, "Updating local ref") || strings.Contains(r.Stderr, "calculation view") {
						t.Fatalf("contextual calculation projected remote refs: %q", r.Stderr)
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

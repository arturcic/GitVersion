package main

import (
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestCSharpBranchContextResolution(t *testing.T) {
	for _, test := range []struct{ upper, alias, branch string }{
		{"feature/upper", "feature/lower", "feature/upper"},
		{"", "feature/lower", "feature/lower"},
		{" \t", "feature/lower", "feature/lower"},
		{"refs/heads/feature/work", "", "feature/work"},
		{"refs/remotes/origin/feature/work", "", "feature/work"},
		{"origin/feature/work", "", "origin/feature/work"},
		{"feature/é", "", "feature/é"}, {"@", "", "@"},
	} {
		env := map[string]string{"GIT_BRANCH": test.upper, "Git_Branch": test.alias}
		target, branch, err := branchSelection("", func(key string) string { return env[key] })
		if err != nil || target != "" || branch != "refs/heads/"+test.branch {
			t.Fatalf("%q / %q resolved to %q, %q, %v", test.upper, test.alias, target, branch, err)
		}
	}
	for _, name := range []string{
		"HEAD", "-branch", "a..b", "a@{1}", "@{-1}", "a b", "a\tb", "a\x7fb", "a~b", "a^b", "a:b",
		"a?b", "a*b", "a[b", "a\\b", "a/", "/a", "a//b", ".a", "a/.b", "a.", "a.lock/b", "a/b.lock",
		"refs/heads/", "refs/remotes//feature/work", "refs/remotes/origin", "refs/tags/1.2.3", "refs/pull/42/merge",
	} {
		env := func(string) string { return name }
		if _, _, err := branchSelection("", env); err == nil || !strings.Contains(err.Error(), "Invalid branch name") {
			t.Fatalf("%q must be rejected, got %v", name, err)
		}
		if target, branch, err := branchSelection("feature/explicit", env); err != nil || branch != "" || target != "feature/explicit" {
			t.Fatalf("explicit branch lost precedence over %q: %q / %q / %v", name, target, branch, err)
		}
	}
	if target, branch, err := branchSelection("", func(string) string { return " \t" }); err != nil || target != "" || branch != "" {
		t.Fatalf("blank aliases resolved to %q / %q / %v", target, branch, err)
	}
}

func TestBranchContextCacheSeparatesTargetFromCheckout(t *testing.T) {
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
	t.Setenv("Git_Branch", "")
	source := branchSelectionHistory(t, t.TempDir())
	t.Setenv("GIT_BRANCH", "feature/work")
	check := func(sha string, args ...string) {
		t.Helper()
		raw, _ := executeCacheCLI(t, append([]string{source.path, "--no-normalize"}, args...)...)
		values, err := parity.Object([]byte(raw))
		if err != nil || values["Sha"] != sha || values["BranchName"] != "feature/work" {
			t.Fatalf("got branch %v, SHA %v, want feature/work at %s (%v)", values["BranchName"], values["Sha"], sha, err)
		}
	}
	check(source.head)
	check(source.target, "--branch", "feature/work")
	check(source.head)
	if entries := cacheEntries(t, source.path); len(entries) != 2 {
		t.Fatalf("expected two independent cache entries, got %v", entries)
	}
}

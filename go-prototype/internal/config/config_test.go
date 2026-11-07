package config

import (
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"os"
	"path/filepath"
	"testing"
)

func TestFrozenConfigurationDefaultsAndOverrides(t *testing.T) {
	c, err := Parse([]byte("# comment\nworkflow: 'GitHubFlow/v1'\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Next != nil || c.MainIncrement != semver.Patch {
		t.Fatalf("defaults %+v", c)
	}
	c, err = Parse([]byte("workflow: GitHubFlow/v1\ncalculation:\n  next-version: 7.8.9\n  branches:\n    main:\n      increment: Minor\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Next == nil || c.Next.String() != "7.8.9" || c.MainIncrement != semver.Minor {
		t.Fatalf("overrides %+v", c)
	}
	for _, name := range []string{"None", "Patch", "Minor", "Major"} {
		c, err = Parse([]byte("workflow: GitHubFlow/v1\ncalculation: {branches: {main: {increment: " + name + "}}}\n"))
		if err != nil || c.MainIncrement.String() != name {
			t.Fatalf("increment %s: %+v %v", name, c, err)
		}
	}
}
func TestFrozenConfigurationRejectsUnsupportedInput(t *testing.T) {
	for _, tc := range []struct{ name, yaml string }{
		{"missing-workflow", "{}"}, {"wrong-workflow", "workflow: UnknownFlow/v1"}, {"duplicate", "workflow: GitHubFlow/v1\nworkflow: GitHubFlow/v1"}, {"unknown-root", "workflow: GitHubFlow/v1\nunknown: {}"},
		{"mode", "workflow: GitHubFlow/v1\ncalculation: {mode: Mainline}"}, {"unknown-branch", "workflow: GitHubFlow/v1\ncalculation: {branches: {unrecognized-config-key: {}}}"}, {"regex", "workflow: GitHubFlow/v1\ncalculation: {branches: {main: {regex: x}}}"},
		{"null-increment", "workflow: GitHubFlow/v1\ncalculation: {branches: {main: {increment: null}}}"}, {"duplicate-increment", "workflow: GitHubFlow/v1\ncalculation: {branches: {main: {increment: Minor, increment: Patch}}}"},
		{"sequence", "workflow: GitHubFlow/v1\ncalculation: []"}, {"anchor", "workflow: &flow GitHubFlow/v1"}, {"alias", "workflow: *flow"}, {"multiple-docs", "workflow: GitHubFlow/v1\n---\n{}"}, {"bad-yaml", "workflow: ["},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.yaml)); err == nil {
				t.Fatal("unsupported config accepted")
			}
		})
	}
}
func TestConfigurationFileDiscovery(t *testing.T) {
	for _, name := range []string{"GitVersion.yml", "GitVersion.yaml", ".GitVersion.yml", ".GitVersion.yaml", ".gitversion.yml", "gitVERSION.YAML"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, name), []byte("workflow: GitHubFlow/v1\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(dir); err != nil {
				t.Fatal(err)
			}
		})
	}
	dir := t.TempDir()
	if c, err := Load(dir); err != nil || c.Workflow != "GitFlow/v1" {
		t.Fatal("missing config did not use defaults", err)
	}
	for _, n := range []string{"GitVersion.yml", "GitVersion.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("workflow: GitHubFlow/v1"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "GitVersion.yaml"), []byte("invalid YAML: ["), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err != nil {
		t.Fatal("preferred .yml was not selected", err)
	}
}
func TestBranchLabelsAndUTF16Sanitization(t *testing.T) {
	for _, tc := range []struct {
		branch, label string
		main          bool
	}{{"main", "", true}, {"master", "", true}, {"feature/foo", "foo", false}, {"features/team/fix", "team-fix", false}, {"feature/café", "caf-", false}, {"feature/a😀b", "a--b", false}, {"feature/a.b", "a-b", false}} {
		main, label, err := Branch(tc.branch)
		if err != nil || main != tc.main || label != tc.label {
			t.Fatalf("branch %q: %t %q %v", tc.branch, main, label, err)
		}
	}
	for _, b := range []string{"develop", "release/1.2.3", "feature/", "unknown"} {
		if _, _, err := Branch(b); err == nil {
			t.Errorf("unsupported branch accepted %q", b)
		}
	}
	if Sanitize("feature/a.b", false) != "feature-a-b" {
		t.Fatal("escaped branch name permits dot")
	}
}

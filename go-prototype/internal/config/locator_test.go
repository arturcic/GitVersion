package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeConfigurationLocatorBoundaries(t *testing.T) {
	dir := t.TempDir()
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "GitVersion.yml"), "workflow: GitHubFlow/v1\n")
	t.Run("missing-custom-falls-back-in-locator", func(t *testing.T) {
		name := "missing.yml"
		path, err := (FileLocator{Custom: &name}).Get(&dir)
		if err != nil || path == nil || *path != filepath.Join(dir, "GitVersion.yml") {
			t.Fatal(path, err)
		}
	})
	t.Run("explicit-cli-missing-fails", func(t *testing.T) {
		_, err := LoadNamed(dir, "missing.yml")
		if err == nil || !strings.Contains(err.Error(), "missing.yml") {
			t.Fatal(err)
		}
	})
	t.Run("absolute-outside-repository", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "custom.yaml")
		write(path, "workflow: GitHubFlow/v1\ncalculation: {next-version: 8.0.0}")
		cfg, err := LoadNamed(dir, path)
		if err != nil || cfg.Next == nil || cfg.Next.String() != "8.0.0" {
			t.Fatal(cfg, err)
		}
	})
	t.Run("absolute-verify-does-not-probe", func(t *testing.T) {
		name := filepath.Join(dir, "not-present.yaml")
		if err := (FileLocator{Custom: &name}).Verify(filepath.Join(dir, "working"), dir); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("missing-directory-provider-default", func(t *testing.T) {
		missing := filepath.Join(dir, "missing")
		cfg, err := (FileLocator{}).ProvideForDirectory(&missing, "v7")
		if err != nil || cfg.Workflow != "GitFlow/v1" {
			t.Fatal(cfg, err)
		}
	})
	for _, c := range []struct {
		name, text string
		bad        bool
	}{{"empty", "", false}, {"comments", "# comment", false}, {"empty-map", "{}", false}, {"null", "null", false}, {"duplicate-workflow", "workflow: GitFlow/v1\nworkflow: GitHubFlow/v1", true}, {"multiple-documents", "{}\n---\n{}", true}, {"unknown-root", "bogus: true", true}} {
		t.Run("provider-"+c.name, func(t *testing.T) {
			_, err := ParseProvided([]byte(c.text), "v7")
			if (err != nil) != c.bad {
				t.Fatal(err)
			}
		})
	}
}

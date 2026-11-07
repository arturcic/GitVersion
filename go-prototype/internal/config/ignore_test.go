package config

import (
	"strings"
	"testing"
	"time"
)

func TestIgnoreConfigurationParsing(t *testing.T) {
	for _, c := range []struct {
		name, yaml string
		bad        bool
	}{
		{"empty", "{}", false}, {"values", "{sha: [AbC, AbC], paths: ['^docs/', '^docs/'], commits-before: '2025-01-01T02:00:00+02:00'}", false},
		{"timestamp", "{commits-before: 2025-01-01T00:00:00Z}", false},
		{"bad-date", "{commits-before: tomorrow}", true}, {"bad-pattern", "{paths: ['[secret-pattern']}", true},
		{"scalar-sha", "{sha: abc}", true}, {"typed-sha", "{sha: [123]}", true}, {"scalar-path", "{paths: docs}", true}, {"duplicate", "{sha: [], sha: []}", true}, {"unknown", "{anything: []}", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := Parse([]byte("workflow: GitHubFlow/v1\ncalculation:\n  ignore: " + c.yaml + "\n"))
			if (err != nil) != c.bad {
				t.Fatal(err)
			}
			if c.bad {
				if strings.Contains(err.Error(), "secret-pattern") {
					t.Fatal("pattern leaked in diagnostic")
				}
				return
			}
			if c.name == "empty" && !cfg.Ignore.Empty() {
				t.Fatal("empty settings activated a filter")
			}
			if c.name == "values" {
				if len(cfg.Ignore.SHAs) != 1 || len(cfg.Ignore.Paths) != 1 || cfg.Ignore.Before == nil || !cfg.Ignore.Before.Equal(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)) {
					t.Fatal(cfg.Ignore)
				}
			}
		})
	}
}

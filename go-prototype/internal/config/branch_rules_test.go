package config

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

func TestBranchRulesResolveFirstMatchAndNamedCaptures(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/branch-rule-recipes.json")
	if err != nil {
		t.Fatal(err)
	}
	var inputs []struct {
		ID, Config    string
		InitialBranch string `json:"initial_branch"`
		Steps         []struct{ Op, Name string }
	}
	if err := json.Unmarshal(raw, &inputs); err != nil {
		t.Fatal(err)
	}
	expected := map[string]struct {
		kind, label string
		increment   semver.Increment
	}{
		"case-insensitive":      {"custom", "Foo", semver.None},
		"main-regex":            {"main", "alpha", semver.Patch},
		"specific-first":        {"specific", "stable", semver.None},
		"broad-first":           {"broad", "broad", semver.Minor},
		"named-captures":        {"custom", "preview.Foo---", semver.None},
		"sha-capture":           {"custom", "topic", semver.None},
		"source-inherit":        {"custom", "topic", semver.Patch},
		"inline-case-sensitive": {"unknown", "CUSTOM-topic", semver.Patch},
	}
	if len(inputs) != len(expected)+11 {
		t.Fatal("edge inventory differs")
	}
	for _, recipe := range inputs {
		if strings.HasPrefix(recipe.ID, "branch-rule-trunk-global-") || strings.HasPrefix(recipe.ID, "branch-rule-inherited-") {
			continue
		}
		name := strings.TrimPrefix(recipe.ID, "branch-rule-")
		t.Run(name, func(t *testing.T) {
			cfg, err := Parse([]byte(recipe.Config))
			if err != nil {
				t.Fatal(err)
			}
			branch := recipe.InitialBranch
			if branch == "" {
				branch = "main"
			}
			for _, step := range recipe.Steps {
				if step.Op == "checkout" {
					branch = step.Name
				}
			}
			got, err := cfg.EffectiveAt(branch, "main", "1234567890abcdef")
			if err != nil {
				t.Fatal(err)
			}
			want, ok := expected[name]
			if !ok {
				t.Fatal("missing expected behavior")
			}
			if got.Kind != want.kind || got.Label != want.label || got.Increment != want.increment {
				t.Fatalf("got %+v, want %+v", got, want)
			}
			if name == "source-inherit" && !slices.Equal(cfg.SourceKinds("custom"), []string{"main"}) {
				t.Fatal("custom source rule lost")
			}
		})
	}
}

func TestBranchRulesRejectInvalidDefinitions(t *testing.T) {
	for _, tc := range []struct{ name, branches, diagnostic string }{
		{"missing-regex", `{custom: {label: preview}}`, "requires a regex"},
		{"invalid-regex", `{custom: {regex: '('}}`, "regex"},
		{"source-scalar", `{custom: {regex: '^custom/', source-branches: main}}`, "requires a list"},
		{"source-mapping", `{custom: {regex: '^custom/', source-branches: [{}]}}`, "requires a plain string"},
		{"source-missing", `{custom: {regex: '^custom/', source-branches: [missing]}}`, "referenced branch configuration"},
		{"undefined-capture", `{custom: {regex: '^custom/', label: '{Missing}'}}`, "unsupported label template"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte("workflow: GitFlow/v1\ncalculation: {branches: " + tc.branches + "}\n")); err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatal("invalid branch definition diagnostic differs", err)
			}
		})
	}
}

func TestTrunkBasedWorkflowUsesPinnedDefaults(t *testing.T) {
	for _, global := range []Mode{"", Manual, Delivery, Deployment} {
		name := string(global)
		if name == "" {
			name = "default"
		}
		t.Run(name, func(t *testing.T) {
			data := "workflow: TrunkBased/preview1\n"
			if global != "" {
				data += "calculation: {mode: " + string(global) + "}\n"
			}
			cfg, err := Parse([]byte(data))
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Strategies) != 2 || !cfg.Strategies["ConfiguredNextVersion"] || !cfg.Strategies["Mainline"] {
				t.Fatal("TrunkBased strategy defaults differ")
			}
			for _, tc := range []struct {
				branch, label string
				mode          Mode
				increment     semver.Increment
				weight        int64
				main          bool
			}{
				{"main", "", Deployment, semver.Patch, 55000, true},
				{"feature/topic", "topic", Delivery, semver.Minor, 30000, false},
				{"hotfix/topic", "topic", Delivery, semver.Patch, 30000, false},
				{"pull/42", "PullRequest42", Delivery, semver.Patch, 30000, true},
				{"topic", "topic", Delivery, semver.Patch, 30000, false},
			} {
				if tc.branch == "topic" && global != "" {
					tc.mode = global
				}
				e, err := cfg.EffectiveFrom(tc.branch, "main")
				if err != nil {
					t.Fatal(err)
				}
				if e.Mode != tc.mode || e.Label != tc.label || e.Increment != tc.increment || e.Weight != tc.weight || e.IsMain != tc.main {
					t.Fatalf("branch %s has %+v", tc.branch, e)
				}
			}
		})
	}
}

func TestCustomBranchesInheritNullableDeploymentAndTagSetting(t *testing.T) {
	for _, tc := range []struct {
		name, parent, custom string
		mode                 Mode
		preventTagged        bool
	}{
		{"inherited-main", "main", "", Manual, false},
		{"inherited-develop", "develop", "", Manual, false},
		{"explicit-child", "develop", ", mode: ContinuousDeployment, prevent-increment: {when-current-commit-tagged: true}", Deployment, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := "workflow: GitFlow/v1\ncalculation:\n  branches:\n    " + tc.parent + ": {mode: ManualDeployment, increment: Minor, prevent-increment: {when-current-commit-tagged: false}}\n    custom: {regex: '^custom/(?<BranchName>.+)', label: '{BranchName}', increment: Inherit, source-branches: [" + tc.parent + "]" + tc.custom + "}\n"
			cfg, err := Parse([]byte(data))
			if err != nil {
				t.Fatal(err)
			}
			e, err := cfg.EffectiveFrom("custom/topic", tc.parent)
			if err != nil {
				t.Fatal(err)
			}
			if e.Mode != tc.mode || e.PreventTagged != tc.preventTagged || e.Increment != semver.Minor || e.Label != "topic" {
				t.Fatalf("nullable inheritance differs: %+v", e)
			}
		})
	}
}

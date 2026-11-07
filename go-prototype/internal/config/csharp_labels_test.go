package config

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestCSharpConfigurationLabels(t *testing.T) {
	var corpus struct {
		Revision string
		Methods  map[string][]struct {
			ID                                       string
			Label, Regex, Branch, Override, Expected *string
			SHA                                      string `json:"sha"`
			ShortSHA                                 string `json:"short_sha"`
			Environment                              map[string]string
			MissingCommit                            bool `json:"missing_commit"`
			Error                                    bool
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-configuration-labels.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	names := []string{}
	total := 0
	for name, cases := range corpus.Methods {
		names = append(names, name)
		total += len(cases)
	}
	sort.Strings(names)
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(names) != 16 || total != 39 {
		t.Fatal("source inventory changed")
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			for _, tc := range corpus.Methods[name] {
				t.Run(tc.ID, func(t *testing.T) {
					c := LabelConfiguration{Label: tc.Label, Regex: tc.Regex}
					commit := &LabelCommit{tc.SHA, tc.ShortSHA}
					if tc.MissingCommit {
						commit = nil
					}
					env := func(name string) (string, bool) { v, ok := tc.Environment[name]; return v, ok }
					got, err := c.Resolve(tc.Branch, tc.Override, env, commit)
					if tc.Error {
						if err == nil {
							t.Fatal("invalid label context accepted")
						}
						return
					}
					if err != nil || !reflect.DeepEqual(got, tc.Expected) {
						t.Fatalf("want %v got %v (%v)", tc.Expected, got, err)
					}
				})
			}
		})
	}
}
func TestLabelResolutionBoundaries(t *testing.T) {
	ptr := func(s string) *string { return &s }
	commit := &LabelCommit{"abcdef0123456789", "abcdef0"}
	env := func(string) (string, bool) { return "", false }
	if _, err := (LabelConfiguration{}).Resolve(nil, nil, nil, commit); err == nil {
		t.Fatal("nil environment accepted")
	}
	if got, err := (LabelConfiguration{}).Resolve(nil, nil, env, commit); err != nil || got != nil {
		t.Fatal("null label changed", got, err)
	}
	cases := []struct {
		name, label, pattern, branch, want string
		fail                               bool
	}{
		{"capture-sanitization", "{BranchName}", `^feature/(?<BranchName>.+)$`, "feature/a.b/😀", "a-b---", false},
		{"unnamed-not-placeholder", "{1}", `^feature/(.+)$`, "feature/topic", "1", false},
		{"empty-named-capture", "{Sha}.{ShortSha}", `^feature/(?<Sha>.*)$`, "feature/", ".abcdef0", false},
		{"invalid-pattern", "x", `[`, "feature/topic", "", true},
		{"invalid-numeric-format-fallback", `{env:MISSING ?? Sha}`, `^feature/`, "feature/topic", "abcdef0123456789", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (LabelConfiguration{ptr(tc.label), ptr(tc.pattern)}).Resolve(ptr(tc.branch), nil, env, commit)
			if tc.fail {
				if err == nil {
					t.Fatal("expected failure")
				}
				return
			}
			if err != nil || got == nil || *got != tc.want {
				t.Fatal("label differs", got, err)
			}
		})
	}
	got, err := (LabelConfiguration{ptr("{ShortSha}"), ptr("[")}).Resolve(nil, nil, env, commit)
	if err != nil || got == nil || *got != "abcdef0" {
		t.Fatal("no branch should skip regex matching", got, err)
	}
}

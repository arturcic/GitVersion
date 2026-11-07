package config

import (
	"strings"
	"testing"
)

func TestLegacyDocumentAndSelectionBoundaries(t *testing.T) {
	for _, c := range []struct {
		name, input, version string
		bad                  bool
	}{
		{"empty-document", "", "v6", false}, {"legacy-next", "next-version: 2.0.0", "v6", false}, {"version-case", "next-version: 2.0.0", " V6 ", false},
		{"shared", "workflow: GitHubFlow/v1", "v6", false}, {"nested-default", "workflow: GitHubFlow/v1\ncalculation: {}", "", false},
		{"wrong-selected-v7", "next-version: 2.0.0", "v7", true}, {"wrong-selected-v6", "workflow: GitHubFlow/v1\ncalculation: {}", "v6", true},
		{"unknown-selection", "workflow: GitHubFlow/v1", "v8", true}, {"duplicate", "mode: ManualDeployment\nmode: ContinuousDelivery", "v6", true}, {"multiple", "{}\n---\n{}", "v6", true},
		{"unknown-strategy", "strategies: Nope", "v6", true}, {"date-zero", "ignore: {commits-before: '0000-01-01T00:00:00Z'}", "v6", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseSelected([]byte(c.input), c.version)
			if (err != nil) != c.bad {
				t.Fatal(err)
			}
		})
	}
	t.Run("loader-before-calculation-validation", func(t *testing.T) {
		doc, err := ReadLegacyDocument([]byte("next-version: 1.0"))
		if err != nil || doc.NextVersion != "1.0" || !doc.Ignore.Empty() {
			t.Fatal(doc, err)
		}
	})
	t.Run("set-duplicates-and-copy", func(t *testing.T) {
		values := []string{"^a/", "^a/", "^b/"}
		i := (IgnoreSettings{}).WithBranches(values...)
		values[0] = "changed"
		if len(i.Branches) != 2 || i.Branches[0] != "^a/" {
			t.Fatal(i)
		}
	})
	t.Run("validation-order", func(t *testing.T) {
		err := (IgnoreSettings{Branches: []string{"[branch-secret"}, Tags: []string{"[tag-secret"}, Paths: []string{"[path-secret"}}).Validate()
		if err == nil || !strings.Contains(err.Error(), "ignore.branches") || strings.Contains(err.Error(), "secret") {
			t.Fatal(err)
		}
	})
	for _, c := range []struct {
		name string
		want bool
	}{{"refs/remotes/origin/release/1", true}, {"refs/remotes/upstream/release/1", true}, {"release/1", true}, {"feature/release/1", false}} {
		t.Run("remote-filter-"+c.name, func(t *testing.T) {
			yes, err := (IgnoreSettings{Branches: []string{"^RELEASE/"}}).BranchIgnored(c.name)
			if err != nil || yes != c.want {
				t.Fatal(yes, err)
			}
		})
	}
}

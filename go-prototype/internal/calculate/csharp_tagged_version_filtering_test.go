package calculate

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestCSharpTaggedVersionFiltering(t *testing.T) {
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct {
				Cases []struct {
					ID             string
					Tags, Patterns []string
					ExpectedTags   []string `json:"expected_tags"`
				}
			}
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-wix-and-output.json", &corpus); err != nil {
		t.Fatal(err)
	}
	for method, item := range corpus.Suites["TaggedSemanticVersionRepositoryTests"].Methods {
		t.Run(method, func(t *testing.T) {
			for _, tc := range item.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					steps := []fixtures.Step{{Op: "commit", ID: "root"}}
					for _, name := range tc.Tags {
						steps = append(steps, fixtures.Step{Op: "tag", Name: name})
					}
					steps = append(steps, fixtures.Step{Op: "checkpoint", ID: "head"})
					cps, err := fixtures.Generate([]fixtures.Recipe{{ID: "filtered-tags", Steps: steps}}, filepath.Join(t.TempDir(), "fixtures"))
					if err != nil {
						t.Fatal(err)
					}
					repo, err := gitrepo.Open(cps[0].Path)
					if err != nil {
						t.Fatal(err)
					}
					refs, err := repo.References()
					if err != nil {
						t.Fatal(err)
					}
					raw, err := repo.Tags(refs)
					if err != nil {
						t.Fatal(err)
					}
					settings := config.IgnoreSettings{Tags: tc.Patterns}
					filter, err := newCommitFilter(repo, settings)
					if err != nil {
						t.Fatal(err)
					}
					tags, err := readSemanticTags(raw, settings, filter, repo.CommitMetadata)
					if err != nil {
						t.Fatal(err)
					}
					var actual []string
					for _, t := range tags {
						actual = append(actual, strings.TrimPrefix(t.ref.Name, "refs/tags/"))
					}
					slices.Sort(actual)
					if !slices.Equal(actual, tc.ExpectedTags) {
						t.Fatalf("eligible tags = %q, want %q", actual, tc.ExpectedTags)
					}
				})
			}
		})
	}
}

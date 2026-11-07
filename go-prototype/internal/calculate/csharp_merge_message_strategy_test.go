package calculate

import (
	"fmt"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestCSharpMergeMessageStrategy(t *testing.T) {
	var corpus struct {
		Revision string
		Methods  map[string][]struct {
			ID, Message          string
			Merge                bool
			Version, Incremented *string
			DisableHotfix        bool    `json:"disable_hotfix"`
			ReleaseBranch        *string `json:"release_branch"`
			Suffixes             []string
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-merge-message-strategy.json", &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Methods) != 6 {
		t.Fatal("original strategy test inventory changed")
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.ID, func(t *testing.T) {
					document := "workflow: GitFlow/v1\n"
					if tc.DisableHotfix {
						document += "calculation:\n  branches:\n    hotfix:\n      is-release-branch: false\n"
					}
					if tc.ReleaseBranch != nil {
						document += fmt.Sprintf("calculation:\n  branches:\n    %s:\n      is-release-branch: true\n", *tc.ReleaseBranch)
					}
					cfg, err := config.Parse([]byte(document))
					if err != nil {
						t.Fatal(err)
					}
					effective, err := cfg.Effective("main")
					if err != nil {
						t.Fatal(err)
					}
					parents := []string{"first-parent"}
					if tc.Merge {
						parents = append(parents, "second-parent")
					}
					for i, suffix := range tc.Suffixes {
						t.Run(fmt.Sprintf("suffix-%02d", i), func(t *testing.T) {
							load := func(id string) (gitrepo.Commit, error) {
								if id != "tip" {
									return gitrepo.Commit{}, fmt.Errorf("unexpected commit %q", id)
								}
								return gitrepo.Commit{Message: tc.Message + suffix, Parents: parents}, nil
							}
							got, err := workflowCandidates(nil, cfg, effective, "main", "", "tip", nil, []string{"tip"}, nil, load)
							if err != nil {
								t.Fatal(err)
							}
							if tc.Version == nil && tc.Incremented == nil {
								if len(got) != 0 {
									t.Fatalf("expected no candidate, got %v", got[0].semantic)
								}
								return
							}
							if len(got) != 1 {
								t.Fatalf("expected one candidate, got %d", len(got))
							}
							if tc.Version != nil && got[0].semantic.String() != *tc.Version {
								t.Fatalf("semantic version %s, want %s", got[0].semantic, *tc.Version)
							}
							if tc.Incremented != nil && got[0].version.String() != *tc.Incremented {
								t.Fatalf("incremented version %s, want %s", got[0].version, *tc.Incremented)
							}
						})
					}
				})
			}
		})
	}
}

package gitrepo

import (
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestCSharpRepositoryExecutorLayout(t *testing.T) {
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct{ Cases []struct{ ID string } }
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-repository-execution.json", &corpus); err != nil {
		t.Fatal(err)
	}
	for method, item := range corpus.Suites["GitVersionExecutorTests"].Methods {
		if !strings.HasPrefix(method, "GetDotGitDirectory") {
			continue
		}
		t.Run(method, func(t *testing.T) {
			for _, tc := range item.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					dir := t.TempDir()
					path := dir
					if method == "GetDotGitDirectoryNoWorktree" {
						if _, err := git.PlainInit(dir, false); err != nil {
							t.Fatal(err)
						}
					} else {
						cps, err := fixtures.Generate([]fixtures.Recipe{{
							ID: "common-dir", Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "head"}},
						}}, filepath.Join(dir, "fixtures"))
						if err != nil {
							t.Fatal(err)
						}
						dir, path = cps[0].Path, filepath.Join(dir, "worktree")
						if err := fixtures.LinkWorktree(dir, path, "worktree", false); err != nil {
							t.Fatal(err)
						}
					}
					layout, err := discoverLayout(path)
					want := filepath.Join(dir, ".git")
					if err != nil || layout.commonDirectory != want {
						t.Fatalf("common directory %q, want %q, error %v", layout.commonDirectory, want, err)
					}
				})
			}
		})
	}
}

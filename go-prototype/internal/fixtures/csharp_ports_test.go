package fixtures

import (
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"path/filepath"
	"testing"
)

func TestInitialBranchAndDeletionAreDeterministic(t *testing.T) {
	r := Recipe{ID: "branch-lifecycle", InitialBranch: "develop", Config: "workflow: GitFlow/v1\n", Steps: []Step{{Op: "commit", ID: "root"}, {Op: "branch", Name: "feature/test"}, {Op: "checkpoint", ID: "before"}, {Op: "delete-branch", Name: "feature/test"}, {Op: "checkpoint", ID: "after"}}}
	a, err := Generate([]Recipe{r}, filepath.Join(t.TempDir(), "a"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate([]Recipe{r}, filepath.Join(t.TempDir(), "b"))
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 2 || len(b) != 2 {
		t.Fatal("checkpoint count differs")
	}
	if a[0].Head != a[1].Head || a[0].Digest == a[1].Digest {
		t.Fatal("deletion must change refs without changing commit")
	}
	for i, cp := range a {
		if cp.Branch != "develop" || cp.Head != b[i].Head || cp.Digest != b[i].Digest {
			t.Fatal("nondeterministic branch state")
		}
		repo, err := git.PlainOpen(cp.Path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Reference(plumbing.NewBranchReferenceName("main"), true); err != plumbing.ErrReferenceNotFound {
			t.Fatal("unexpected main ref", err)
		}
		_, err = repo.Reference(plumbing.NewBranchReferenceName("feature/test"), true)
		if (i == 0 && err != nil) || (i == 1 && err != plumbing.ErrReferenceNotFound) {
			t.Fatal("branch deletion not applied", err)
		}
		wt, err := repo.Worktree()
		if err != nil {
			t.Fatal(err)
		}
		state, err := wt.Status()
		if err != nil || !state.IsClean() {
			t.Fatal("dirty checkpoint", err)
		}
	}
}
func TestInvalidBranchLifecycleIsRejected(t *testing.T) {
	for _, name := range []string{"main", "missing"} {
		t.Run(name, func(t *testing.T) {
			r := Recipe{ID: "bad-delete", Steps: []Step{{Op: "commit", ID: "root"}, {Op: "delete-branch", Name: name}, {Op: "checkpoint", ID: "head"}}}
			if _, err := Generate([]Recipe{r}, filepath.Join(t.TempDir(), "fixtures")); err == nil {
				t.Fatal("invalid deletion accepted")
			}
		})
	}
	r := Recipe{ID: "bad-initial", InitialBranch: "bad..branch", Steps: []Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "head"}}}
	if _, err := Generate([]Recipe{r}, filepath.Join(t.TempDir(), "fixtures")); err == nil {
		t.Fatal("invalid initial branch accepted")
	}
}

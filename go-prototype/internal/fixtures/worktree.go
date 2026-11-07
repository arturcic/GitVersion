package fixtures

import (
	"fmt"
	"os"
	"path/filepath"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// LinkWorktree creates the same per-worktree HEAD and index, gitdir pointer,
// and shared objects and refs used by git worktree add, for a local fixture.
func LinkWorktree(source, dest, branch string, relative bool) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	dest, err = filepath.Abs(dest)
	if err != nil {
		return err
	}
	repo, err := git.PlainOpen(source)
	if err != nil {
		return err
	}
	tip, err := repo.Head()
	if err != nil {
		return err
	}
	ref := plumbing.NewBranchReferenceName(branch)
	if err := ref.Validate(); err != nil {
		return err
	}
	if _, err := repo.Reference(ref, false); err != plumbing.ErrReferenceNotFound {
		return fmt.Errorf("fixture branch %q already exists or cannot be inspected", branch)
	}
	if err := os.Mkdir(dest, 0755); err != nil {
		return err
	}
	admin := filepath.Join(source, ".git", "worktrees", filepath.Base(dest))
	if err := os.MkdirAll(filepath.Dir(admin), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(admin, 0755); err != nil {
		return err
	}
	target := admin
	if relative {
		target, err = filepath.Rel(dest, admin)
		if err != nil {
			return err
		}
	}
	if err := repo.Storer.SetReference(plumbing.NewHashReference(ref, tip.Hash())); err != nil {
		return err
	}
	for path, contents := range map[string]string{
		filepath.Join(dest, ".git"):       "gitdir: " + target + "\n",
		filepath.Join(admin, "gitdir"):    filepath.Join(dest, ".git") + "\n",
		filepath.Join(admin, "commondir"): "../..\n",
		filepath.Join(admin, "HEAD"):      "ref: " + ref.String() + "\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			return err
		}
	}
	linked, err := git.PlainOpenWithOptions(dest, &git.PlainOpenOptions{EnableDotGitCommonDir: true})
	if err != nil {
		return err
	}
	wt, err := linked.Worktree()
	if err != nil {
		return err
	}
	return wt.Checkout(&git.CheckoutOptions{Branch: ref, Force: true})
}

package gitrepo

import (
	"path/filepath"

	git "github.com/go-git/go-git/v5"
)

func workingTreeStatus(layout repositoryLayout, repo *git.Repository) (git.Status, error) {
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}
	status, err := worktree.Status()
	if err != nil {
		return nil, err
	}
	if err := reconcileUntracked(layout.workingDirectory, layout.commonDirectory, repo, status); err != nil {
		return nil, err
	}
	for _, change := range status {
		if change.Worktree != git.Unmodified && change.Worktree != git.Untracked {
			assumed, err := readIndexAssumeValid(filepath.Join(layout.gitDirectory, "index"))
			if err != nil {
				return nil, err
			}
			for path := range assumed {
				if change := status[path]; change != nil && change.Worktree != git.UpdatedButUnmerged &&
					change.Staging != git.UpdatedButUnmerged {
					change.Worktree = git.Unmodified
				}
			}
			break
		}
	}
	return status, nil
}

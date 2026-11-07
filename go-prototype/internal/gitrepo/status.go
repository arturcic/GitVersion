package gitrepo

import git "github.com/go-git/go-git/v5"

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
	return status, nil
}

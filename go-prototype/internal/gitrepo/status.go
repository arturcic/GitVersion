package gitrepo

import (
	"io"
	"path/filepath"
	"runtime"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
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
	if runtime.GOOS == "windows" && len(status) != 0 {
		if err := ignoreWindowsExecutableMode(worktree, repo, status); err != nil {
			return nil, err
		}
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

// Git records executable modes in the index even on Windows, where the
// working tree cannot represent that bit. Only suppress mode-only changes.
func ignoreWindowsExecutableMode(worktree *git.Worktree, repo *git.Repository, status git.Status) error {
	index, err := repo.Storer.Index()
	if err != nil {
		return err
	}
	for _, entry := range index.Entries {
		change := status[entry.Name]
		if entry.Mode != filemode.Executable || change == nil || change.Worktree != git.Modified {
			continue
		}
		info, err := worktree.Filesystem.Lstat(entry.Name)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		file, err := worktree.Filesystem.Open(entry.Name)
		if err != nil {
			return err
		}
		hash := plumbing.NewHasher(plumbing.BlobObject, info.Size())
		_, err = io.Copy(hash, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if hash.Sum() == entry.Hash {
			change.Worktree = git.Unmodified
		}
	}
	return nil
}

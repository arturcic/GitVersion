package gitrepo

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-billy/v5/osfs"
	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// The pinned C# managed repository reads the common Git configuration, including
// when extensions.worktreeConfig is present. Its repository layout/status/session
// readers do not read config.worktree. Match that contract without rewriting the
// user's configuration or disabling the extension in Git.
//
// go-git v5.19.3 rejects this extension before reading any objects. Remove only
// this recognized marker from the transient parsed configuration presented to
// go-git; every other extension remains subject to go-git's normal validation.
// This adapter does not claim general Git config.worktree overlay semantics.
type commonConfigStorage struct{ *filesystem.Storage }

func (s *commonConfigStorage) Config() (*gitconfig.Config, error) {
	cfg, err := s.Storage.Config()
	if err != nil {
		return nil, err
	}
	if cfg.Raw.HasSection("extensions") {
		cfg.Raw.Section("extensions").RemoveOption("worktreeconfig")
	}
	return cfg, nil
}
func (s *commonConfigStorage) SetConfig(*gitconfig.Config) error {
	return fmt.Errorf("repository configuration is read-only")
}
func openCommonConfigRepository(path string) (*git.Repository, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	st := filesystem.NewStorage(osfs.New(filepath.Join(abs, ".git")), cache.NewObjectLRUDefault())
	return git.Open(&commonConfigStorage{st}, osfs.New(abs))
}

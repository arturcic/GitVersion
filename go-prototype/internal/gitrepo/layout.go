package gitrepo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	errRepositoryNotFound          = errors.New("no Git repository found")
	errUnsupportedRepositoryFormat = errors.New("unsupported repository format")
)

type repositoryLayout struct {
	gitDirectory     string
	commonDirectory  string
	workingDirectory string
}

func (l repositoryLayout) objectsDirectory() string {
	return filepath.Join(l.commonDirectory, "objects")
}

func (l repositoryLayout) readShallowCommits() ([]ObjectID, error) {
	raw, err := os.ReadFile(filepath.Join(l.commonDirectory, "shallow"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []ObjectID
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		id, err := parseObjectID(line)
		if err != nil {
			return nil, fmt.Errorf("shallow boundary: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (l repositoryLayout) isShallow() (bool, error) {
	info, err := os.Stat(filepath.Join(l.commonDirectory, "shallow"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}

func discoverLayout(path string) (repositoryLayout, error) {
	current, err := filepath.Abs(path)
	if err != nil {
		return repositoryLayout{}, err
	}
	for {
		layout, err := openLayout(current)
		if !errors.Is(err, errRepositoryNotFound) {
			return layout, err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return repositoryLayout{}, fmt.Errorf("%w at or above %q", errRepositoryNotFound, path)
		}
		current = parent
	}
}

func openLayout(path string) (repositoryLayout, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return repositoryLayout{}, err
	}
	dotGit := filepath.Join(abs, ".git")
	info, err := os.Stat(dotGit)
	if err == nil {
		if info.IsDir() {
			return layoutFromGitDirectory(dotGit, abs)
		}
		if !info.Mode().IsRegular() {
			return repositoryLayout{}, fmt.Errorf("invalid .git entry %q", dotGit)
		}
		raw, err := os.ReadFile(dotGit)
		if err != nil {
			return repositoryLayout{}, err
		}
		target, ok := strings.CutPrefix(strings.TrimSpace(string(raw)), "gitdir:")
		if !ok || strings.TrimSpace(target) == "" {
			return repositoryLayout{}, fmt.Errorf("malformed .git file %q: expected gitdir pointer", dotGit)
		}
		return layoutFromGitDirectory(resolveLayoutPath(abs, target), abs)
	}
	if !os.IsNotExist(err) {
		return repositoryLayout{}, err
	}
	head, headErr := os.Stat(filepath.Join(abs, "HEAD"))
	objects, objectsErr := os.Stat(filepath.Join(abs, "objects"))
	refs, refsErr := os.Stat(filepath.Join(abs, "refs"))
	if headErr == nil && !head.IsDir() && objectsErr == nil && objects.IsDir() && refsErr == nil && refs.IsDir() {
		return layoutFromGitDirectory(abs, "")
	}
	return repositoryLayout{}, errRepositoryNotFound
}

func resolveLayoutPath(base, target string) string {
	target = strings.TrimSpace(target)
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Join(base, target)
}

func layoutFromGitDirectory(dir, worktree string) (repositoryLayout, error) {
	common := dir
	raw, err := os.ReadFile(filepath.Join(dir, "commondir"))
	if err == nil {
		common = resolveLayoutPath(dir, string(raw))
	} else if !os.IsNotExist(err) {
		return repositoryLayout{}, err
	}
	info, err := os.Stat(filepath.Join(common, "reftable"))
	if err == nil && info.IsDir() {
		return repositoryLayout{}, fmt.Errorf("%w: reftable", errUnsupportedRepositoryFormat)
	}
	if err != nil && !os.IsNotExist(err) {
		return repositoryLayout{}, err
	}
	format, found, err := gitConfigValue(filepath.Join(common, "config"), "extensions", "", "objectformat")
	if err != nil {
		return repositoryLayout{}, err
	}
	if found && !strings.EqualFold(format, "sha1") {
		return repositoryLayout{}, fmt.Errorf("%w: object format %q", errUnsupportedRepositoryFormat, format)
	}
	return repositoryLayout{gitDirectory: dir, commonDirectory: common, workingDirectory: worktree}, nil
}

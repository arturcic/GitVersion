package gitrepo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestDynamicCloneReusesMatchingRemoteAndPreservesCollision(t *testing.T) {
	source := filepath.Join(t.TempDir(), "project.git")
	repo, err := git.PlainInit(source, false)
	if err != nil {
		t.Fatal(err)
	}
	work, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	author := &object.Signature{Name: "Fixture", Email: "fixture@example.invalid", When: time.Unix(1700000000, 0)}
	head, err := work.Commit("initial", &git.CommitOptions{AllowEmptyCommits: true, Author: author, Committer: author})
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	collision := filepath.Join(base, "project")
	if err := os.Mkdir(collision, 0755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(collision, "keep")
	if err := os.WriteFile(sentinel, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "project_1", ".git")
	for range 2 {
		got, err := PrepareClone(t.Context(), source, base, false, Credentials{})
		if err != nil || got != want {
			t.Fatalf("clone path %q, want %q (%v)", got, want, err)
		}
		cloned, err := git.PlainOpen(got)
		if err != nil {
			t.Fatal(err)
		}
		ref, err := cloned.Head()
		if err != nil || ref.Hash() != head {
			t.Fatalf("clone HEAD %v, want %s (%v)", ref, head, err)
		}
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "existing" {
		t.Fatalf("existing checkout changed: %q (%v)", got, err)
	}
	if _, err := PrepareClone(t.Context(), filepath.Join(t.TempDir(), "missing"), base, false, Credentials{}); err == nil {
		t.Fatal("cloning a missing local repository succeeded")
	}
	if entries, err := os.ReadDir(base); err != nil || len(entries) != 2 {
		t.Fatalf("unexpected clone locations: %v (%v)", entries, err)
	}
}

func TestSSHRemoteOperationsAreRejected(t *testing.T) {
	for _, tc := range []struct{ name, url string }{
		{"ssh-url", "ssh://git@127.0.0.1/example.git"},
		{"scp-address", "git@127.0.0.1:example.git"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			cloneBase := filepath.Join(directory, "clones")
			if _, err := PrepareClone(t.Context(), tc.url, cloneBase, false, Credentials{}); !errors.Is(err, errSSHRemoteUnsupported) {
				t.Fatalf("clone: expected SSH rejection, got %v", err)
			}
			if _, err := os.Stat(cloneBase); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("rejected clone created its destination: %v", err)
			}
			repo, err := git.PlainInit(filepath.Join(directory, "checkout"), false)
			if err != nil {
				t.Fatal(err)
			}
			remote, err := repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{tc.url}})
			if err != nil {
				t.Fatal(err)
			}
			worktree, err := repo.Worktree()
			if err != nil {
				t.Fatal(err)
			}
			checkout := worktree.Filesystem.Root()
			if _, err := PrepareRemote(t.Context(), checkout, true, false, Credentials{}); !errors.Is(err, errSSHRemoteUnsupported) {
				t.Fatalf("fetch: expected SSH rejection, got %v", err)
			}
			if _, err := advertisedTips(t.Context(), remote, plumbing.ZeroHash, Credentials{}); !errors.Is(err, errSSHRemoteUnsupported) {
				t.Fatalf("remote ref discovery: expected SSH rejection, got %v", err)
			}
			if name, err := PrepareRemote(t.Context(), checkout, false, false, Credentials{}); err != nil || name != "origin" {
				t.Fatalf("preparing without fetch: got %q (%v)", name, err)
			}
		})
	}
}

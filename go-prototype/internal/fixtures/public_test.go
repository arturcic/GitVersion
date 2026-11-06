package fixtures

import (
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestPublicSnapshotPreservesObjectsAndIsolatesOverlay(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	r, err := git.PlainInit(source, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("public content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("README.md"); err != nil {
		t.Fatal(err)
	}
	hash, err := wt.Commit("public root", &git.CommitOptions{Author: &object.Signature{Name: "Fixture", Email: "fixture@example.invalid", When: time.Unix(1000, 0)}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateTag("v1.2.3", hash, nil); err != nil {
		t.Fatal(err)
	}
	before, err := State(source)
	if err != nil {
		t.Fatal(err)
	}
	spec := PublicCase{ID: "public", Repository: "Example/project", Commit: hash.String(), Branch: "feature/trial", Tags: map[string]string{"v1.2.3": hash.String()}, Config: "workflow: GitHubFlow/v1\n"}
	dest := filepath.Join(root, "snapshot")
	cp, err := PublicSnapshot(source, dest, spec)
	if err != nil {
		t.Fatal(err)
	}
	if cp.Head != hash.String() || cp.Branch != spec.Branch {
		t.Fatal("identity changed", cp)
	}
	copy, err := git.PlainOpen(dest)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := copy.CommitObject(hash)
	if err != nil || commit.Message != "public root" {
		t.Fatal("commit was rewritten", err)
	}
	ref, err := copy.Reference(plumbing.NewTagReferenceName("v1.2.3"), true)
	if err != nil || ref.Hash() != hash {
		t.Fatal("tag changed")
	}
	observed, err := gitrepo.Open(dest)
	if err != nil {
		t.Fatal("overlay dirtied repository", err)
	}
	branch, actualHead := observed.Head()
	if branch != "refs/heads/feature/trial" || actualHead != hash.String() {
		t.Fatalf("projected HEAD differs: %s %s", branch, actualHead)
	}
	if data, err := os.ReadFile(filepath.Join(dest, "GitVersion.yml")); err != nil || string(data) != spec.Config {
		t.Fatal("overlay missing")
	}
	after, err := State(source)
	if err != nil || before != after {
		t.Fatal("source changed")
	}
	if _, err := PublicSnapshot(source, dest, spec); err == nil {
		t.Fatal("overwrote existing snapshot")
	}
	spec.Tags["v1.2.3"] = plumbing.ZeroHash.String()
	if _, err := PublicSnapshot(source, filepath.Join(root, "bad-tag"), spec); err == nil {
		t.Fatal("accepted changed public tag")
	}
}

func TestPublicSnapshotRejectsExistingProjectConfiguration(t *testing.T) {
	cps, err := Generate([]Recipe{{ID: "configured", Config: "workflow: GitHubFlow/v1\n", Steps: []Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "head"}}}}, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = PublicSnapshot(cps[0].Path, filepath.Join(t.TempDir(), "snapshot"), PublicCase{ID: "public", Commit: cps[0].Head, Branch: "main", Config: "workflow: GitHubFlow/v1\n"})
	if err == nil {
		t.Fatal("silently replaced original release policy")
	}
}

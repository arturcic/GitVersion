package fixtures

import (
	"fmt"
	"os"
	"path/filepath"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// PublicCase pins real object identities. Main/feature ref projection and the
// ignored raw configuration are experiment inputs, not the project's policy.
type PublicCase struct {
	ID         string            `json:"id"`
	Repository string            `json:"repository"`
	Commit     string            `json:"commit"`
	Branch     string            `json:"branch"`
	Tags       map[string]string `json:"tags"`
	Config     string            `json:"config"`
}

// PublicSnapshot copies the complete reachable graph from an existing local
// object store. No network, clone, hardlinks, alternates or source mutations.
func PublicSnapshot(source, dest string, spec PublicCase) (Checkpoint, error) {
	var cp Checkpoint
	if !safeID.MatchString(spec.ID) || len(spec.Commit) != 40 {
		return cp, fmt.Errorf("invalid public case")
	}
	src, err := git.PlainOpen(source)
	if err != nil {
		return cp, err
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		return cp, fmt.Errorf("destination must not exist")
	}
	dst, err := git.PlainInit(dest, false)
	if err != nil {
		return cp, err
	}
	seen := map[plumbing.Hash]bool{}
	var copyObject func(plumbing.Hash) error
	copyObject = func(hash plumbing.Hash) error {
		if seen[hash] {
			return nil
		}
		seen[hash] = true
		obj, err := src.Storer.EncodedObject(plumbing.AnyObject, hash)
		if err != nil {
			return err
		}
		if actual, err := dst.Storer.SetEncodedObject(obj); err != nil || actual != hash {
			return fmt.Errorf("copy %s: %v", hash, err)
		}
		switch obj.Type() {
		case plumbing.CommitObject:
			c, err := object.DecodeCommit(src.Storer, obj)
			if err != nil {
				return err
			}
			if err := copyObject(c.TreeHash); err != nil {
				return err
			}
			for _, p := range c.ParentHashes {
				if err := copyObject(p); err != nil {
					return err
				}
			}
		case plumbing.TreeObject:
			tr, err := object.DecodeTree(src.Storer, obj)
			if err != nil {
				return err
			}
			for _, e := range tr.Entries {
				if err := copyObject(e.Hash); err != nil {
					return err
				}
			}
		case plumbing.TagObject:
			tag, err := object.DecodeTag(src.Storer, obj)
			if err != nil {
				return err
			}
			return copyObject(tag.Target)
		}
		return nil
	}
	head := plumbing.NewHash(spec.Commit)
	if err := copyObject(head); err != nil {
		return cp, err
	}
	if err := dst.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), head)); err != nil {
		return cp, err
	}
	branch := plumbing.NewBranchReferenceName(spec.Branch)
	if err := branch.Validate(); err != nil {
		return cp, err
	}
	if err := dst.Storer.SetReference(plumbing.NewHashReference(branch, head)); err != nil {
		return cp, err
	}
	for name, sha := range spec.Tags {
		h := plumbing.NewHash(sha)
		actual, err := src.Reference(plumbing.NewTagReferenceName(name), true)
		if err != nil || actual.Hash() != h {
			return cp, fmt.Errorf("public tag identity changed: %s", name)
		}
		if err := copyObject(h); err != nil {
			return cp, err
		}
		if err := dst.Storer.SetReference(plumbing.NewHashReference(plumbing.NewTagReferenceName(name), h)); err != nil {
			return cp, err
		}
	}
	wt, err := dst.Worktree()
	if err != nil {
		return cp, err
	}
	if err := wt.Checkout(&git.CheckoutOptions{Branch: branch}); err != nil {
		return cp, err
	}
	for _, name := range []string{"GitVersion.yml", "GitVersion.yaml", ".GitVersion.yml", ".GitVersion.yaml"} {
		if _, err := os.Stat(filepath.Join(dest, name)); !os.IsNotExist(err) {
			return cp, fmt.Errorf("public history already contains config %s", name)
		}
	}
	if err := os.MkdirAll(filepath.Join(dest, ".git/info"), 0755); err != nil {
		return cp, err
	}
	if err := os.WriteFile(filepath.Join(dest, ".git/info/exclude"), []byte("/GitVersion.yml\n"), 0644); err != nil {
		return cp, err
	}
	if err := os.WriteFile(filepath.Join(dest, "GitVersion.yml"), []byte(spec.Config), 0644); err != nil {
		return cp, err
	}
	digest, err := Digest(dest)
	if err != nil {
		return cp, err
	}
	return Checkpoint{ID: spec.ID, Recipe: spec.Repository, Head: spec.Commit, Branch: spec.Branch, Digest: digest, Path: dest}, nil
}

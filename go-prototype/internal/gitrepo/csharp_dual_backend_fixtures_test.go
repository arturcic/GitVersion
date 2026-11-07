package gitrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

type dualBackendFixture struct{ root, path string }

func newDualBackendFixture(t *testing.T, method string) dualBackendFixture {
	t.Helper()
	f := newManagedObjectFixture(t)
	path, err := filepath.EvalSymlinks(f.path)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(path)
	add := func(file, message string, parents ...plumbing.Hash) plumbing.Hash {
		f.write(file+".txt", file+"\n")
		return f.addCommit(message, parents...)
	}
	checkout := func(branch string, create bool) { f.checkout(branch, create) }
	head := func() plumbing.Hash {
		t.Helper()
		ref, err := f.repo.Head()
		if err != nil {
			t.Fatal(err)
		}
		return ref.Hash()
	}
	complex := func() {
		initial := add("initial", "initial commit")
		f.tag("1.0.0", initial, false)
		checkout("develop", true)
		dev := add("develop-one", "develop one")
		checkout("main", false)
		main := add("main-two", "main two")
		f.write("develop-one.txt", "develop-one\n")
		merged := f.addCommit("Merge branch 'develop'", main, dev)
		f.annotatedTag("v2.0.0", "an annotated tag", merged)
		checkout("feature/complex", true)
		add("feature-one", "feature one")
		checkout("develop", false)
		add("develop-two", "develop two")
		checkout("main", false)
	}
	switch method {
	case "QueryBySequencesAreIdenticalOnBothBackends", "MergeBasesAreIdenticalOnBothBackendsForAllCommitPairs":
		add("base", "base")
		checkout("develop", true)
		dev := add("develop-one", "develop one")
		checkout("main", false)
		main := add("main-one", "main one")
		f.write("develop-one.txt", "develop-one\n")
		f.addCommit("Merge develop into main", main, dev)
		checkout("develop", false)
		f.write("main-one.txt", "main-one\n")
		f.commits-- // Both merges use the same committer timestamp.
		f.addCommit("Merge main into develop", dev, main)
		add("develop-two", "develop two")
		checkout("main", false)
		add("main-two", "main two")

	case "UncommittedChangesCountIsIdenticalOnBothBackends", "IndexVersionFourBehavesIdenticallyOnBothBackends":
		f.write("tracked.txt", "one")
		f.addCommit("add tracked file")
		if method == "IndexVersionFourBehavesIdenticallyOnBothBackends" {
			idx, err := f.repo.Storer.Index()
			if err != nil {
				t.Fatal(err)
			}
			idx.Version = 4
			if err := f.repo.Storer.SetIndex(idx); err != nil {
				t.Fatal(err)
			}
		} else {
			f.write("staged.txt", "staged")
			f.stage("staged.txt")
		}
		f.write("tracked.txt", "two")
		f.write("untracked.txt", "new")

	case "EmptyRepositoryBehavesIdenticallyOnBothBackends":
		f.write("untracked.txt", "new")

	case "DetachedHeadBehavesIdenticallyOnBothBackends":
		complex()
		tip, err := f.repo.CommitObject(head())
		if err != nil {
			t.Fatal(err)
		}
		if err := f.worktree.Checkout(&git.CheckoutOptions{Hash: tip.ParentHashes[0]}); err != nil {
			t.Fatal(err)
		}

	case "RemotesAndRemoteBranchesAreIdenticalOnBothBackends", "ShallowCloneBehavesIdenticallyOnBothBackends":
		var commits []plumbing.Hash
		for i := range 5 {
			commits = append(commits, add(fmt.Sprintf("commit-%d", i), fmt.Sprintf("commit %d", i)))
		}
		upstream := filepath.Join(root, "upstream")
		if err := fixtures.Copy(path, upstream); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{upstream}}); err != nil {
			t.Fatal(err)
		}
		cfg, err := f.repo.Config()
		if err != nil {
			t.Fatal(err)
		}
		cfg.Branches["main"] = &gitconfig.Branch{Name: "main", Remote: "origin", Merge: "refs/heads/main"}
		if err := f.repo.SetConfig(cfg); err != nil {
			t.Fatal(err)
		}
		f.setRef(plumbing.NewHashReference("refs/remotes/origin/main", head()))
		f.setRef(plumbing.NewSymbolicReference("refs/remotes/origin/HEAD", "refs/remotes/origin/main"))
		if method == "ShallowCloneBehavesIdenticallyOnBothBackends" {
			f.write(".git/shallow", head().String()+"\n")
			for _, id := range commits[:len(commits)-1] {
				sha := id.String()
				if err := os.Remove(filepath.Join(path, ".git", "objects", sha[:2], sha[2:])); err != nil {
					t.Fatal(err)
				}
			}
		}

	case "BareRepositoryHasNullProjectRootDirectoryOnBothBackends":
		path = filepath.Join(root, "bare.git")
		if _, err := git.PlainInit(path, true); err != nil {
			t.Fatal(err)
		}

	case "OctopusMergeBehavesIdenticallyOnBothBackends":
		add("base", "base")
		checkout("topic/one", true)
		one := add("topic-one", "topic one")
		checkout("main", false)
		checkout("topic/two", true)
		two := add("topic-two", "topic two")
		checkout("main", false)
		main := add("main-one", "main one")
		f.write("topic-one.txt", "topic-one\n")
		f.write("topic-two.txt", "topic-two\n")
		f.addCommit("octopus", main, one, two)

	case "PackedAndLooseReferencesBehaveIdenticallyOnBothBackends":
		complex()
		packDualBackendRefs(t, f)
		tip := add("shadow", "shadows the packed main entry")
		checkout("loose/branch", true)
		f.tag("loose-tag", tip, false)

	case "LightweightAndAnnotatedTagOnTheSameCommitBehaveIdenticallyOnBothBackends":
		tip := add("only", "only commit")
		f.tag("lightweight", tip, false)
		f.annotatedTag("annotated", "the same commit, annotated", tip)

	case "WorktreeBehavesIdenticallyOnBothBackends":
		complex()
		linked := filepath.Join(root, "linked")
		if err := fixtures.LinkWorktree(path, linked, "temporary-link", false); err != nil {
			t.Fatal(err)
		}
		repo, err := git.PlainOpenWithOptions(linked, &git.PlainOpenOptions{EnableDotGitCommonDir: true})
		if err != nil {
			t.Fatal(err)
		}
		wt, err := repo.Worktree()
		if err != nil {
			t.Fatal(err)
		}
		if err := wt.Checkout(&git.CheckoutOptions{Branch: "refs/heads/develop", Force: true}); err != nil {
			t.Fatal(err)
		}
		if err := f.repo.Storer.RemoveReference("refs/heads/temporary-link"); err != nil {
			t.Fatal(err)
		}
		path = linked

	default:
		complex()
	}
	return dualBackendFixture{root, path}
}

func packDualBackendRefs(t *testing.T, f *managedObjectFixture) {
	t.Helper()
	iterator, err := f.repo.References()
	if err != nil {
		t.Fatal(err)
	}
	var refs []*plumbing.Reference
	err = iterator.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name() != plumbing.HEAD && ref.Type() == plumbing.HashReference {
			refs = append(refs, ref)
		}
		return nil
	})
	iterator.Close()
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name() < refs[j].Name() })
	var out strings.Builder
	out.WriteString("# pack-refs with: peeled fully-peeled sorted\n")
	for _, ref := range refs {
		fmt.Fprintf(&out, "%s %s\n", ref.Hash(), ref.Name())
		if ref.Name() == "refs/tags/v2.0.0" {
			tag, err := f.repo.TagObject(ref.Hash())
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(&out, "^%s\n", tag.Target)
		}
		if err := os.Remove(filepath.Join(f.path, ".git", ref.Name().String())); err != nil {
			t.Fatal(err)
		}
	}
	f.write(".git/packed-refs", out.String())
}

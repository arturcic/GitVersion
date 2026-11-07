package gitrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type contractFixture struct {
	t        *testing.T
	path     string
	repo     *git.Repository
	worktree *git.Worktree
	commits  int
}

func newContractFixture(t *testing.T) *contractFixture {
	t.Helper()
	path := t.TempDir()
	repo, err := git.PlainInitWithOptions(path, &git.PlainInitOptions{InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")}})
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	return &contractFixture{t: t, path: path, repo: repo, worktree: wt}
}

func (f *contractFixture) write(name, contents string) {
	f.t.Helper()
	path := filepath.Join(f.path, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *contractFixture) stage(name string) {
	f.t.Helper()
	if _, err := f.worktree.Add(name); err != nil {
		f.t.Fatal(err)
	}
}

func (f *contractFixture) remove(name string) {
	f.t.Helper()
	if _, err := f.worktree.Remove(name); err != nil {
		f.t.Fatal(err)
	}
}

func (f *contractFixture) commit(message string, parents ...plumbing.Hash) plumbing.Hash {
	f.t.Helper()
	f.stage(".")
	return f.commitIndex(message, parents...)
}

func (f *contractFixture) commitIndex(message string, parents ...plumbing.Hash) plumbing.Hash {
	f.t.Helper()
	f.commits++
	signature := &object.Signature{Name: "Contract Fixture", Email: "contract@example.invalid", When: time.Unix(1735689600+int64(f.commits), 0).UTC()}
	hash, err := f.worktree.Commit(message+"\n", &git.CommitOptions{Author: signature, Committer: signature, Parents: parents})
	if err != nil {
		f.t.Fatal(err)
	}
	return hash
}

func (f *contractFixture) checkout(branch string, create bool) {
	f.t.Helper()
	if err := f.worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(branch), Create: create}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *contractFixture) setRef(ref *plumbing.Reference) {
	f.t.Helper()
	if err := f.repo.Storer.SetReference(ref); err != nil {
		f.t.Fatal(err)
	}
}

func (f *contractFixture) tag(name string, target plumbing.Hash, annotated bool) plumbing.Hash {
	f.t.Helper()
	var options *git.CreateTagOptions
	if annotated {
		options = &git.CreateTagOptions{Tagger: &object.Signature{Name: "Contract Fixture", Email: "contract@example.invalid", When: time.Unix(1735689601, 0).UTC()}, Message: "annotated"}
	}
	ref, err := f.repo.CreateTag(name, target, options)
	if err != nil {
		f.t.Fatal(err)
	}
	return ref.Hash()
}

func (f *contractFixture) store() referenceStore {
	f.t.Helper()
	return referenceStore{git: f.repo, commonDirectory: filepath.Join(f.path, ".git")}
}

func (f *contractFixture) packRefs(branch, tag, peeled plumbing.Hash) {
	f.t.Helper()
	data := "# pack-refs with: peeled fully-peeled sorted\n" + branch.String() + " refs/heads/main\n"
	refs := []string{"refs/heads/main"}
	if !tag.IsZero() {
		data += fmt.Sprintf("%s refs/tags/v1.0.0\n^%s\n", tag, peeled)
		refs = append(refs, "refs/tags/v1.0.0")
	}
	f.write(".git/packed-refs", data)
	for _, name := range refs {
		if err := os.Remove(filepath.Join(f.path, ".git", name)); err != nil {
			f.t.Fatal(err)
		}
	}
}

func contractReferenceFixture(t *testing.T, method string) (*contractFixture, plumbing.Hash, plumbing.Hash) {
	t.Helper()
	f := newContractFixture(t)
	var head, tag plumbing.Hash
	if method == "HeadOfAnUnbornBranchResolvesToNull" {
		return f, head, tag
	}
	f.write("file.txt", "content\n")
	message := "a commit"
	switch method {
	case "EnumerationMatchesGitForEachRef", "LooseReferencesWinOverPackedReferences":
		message = "first"
	case "ResolvedHeadCanBeReadFromTheObjectStore":
		message = "readable through refs"
	}
	head = f.commit(message)
	switch method {
	case "EnumerationMatchesGitForEachRef":
		f.setRef(plumbing.NewHashReference("refs/heads/feature/a", head))
		f.tag("lightweight", head, false)
		f.tag("v1.0.0", head, true)
		f.write("file.txt", "more content\n")
		head = f.commit("second")
	case "EnumerationSkipsTransientLockFiles":
		f.write(".git/refs/heads/main.lock", head.String()+"\n")
	case "ReadsPackedRefsIncludingPeeledTargets":
		tag = f.tag("v1.0.0", head, true)
		f.packRefs(head, tag, head)
	case "LooseReferencesWinOverPackedReferences":
		f.packRefs(head, plumbing.ZeroHash, plumbing.ZeroHash)
		f.write("file.txt", "more content\n")
		head = f.commit("second")
	case "HeadIsDirectWhenDetached":
		f.setRef(plumbing.NewHashReference(plumbing.HEAD, head))
	case "FollowsChainsOfSymbolicReferences":
		f.setRef(plumbing.NewSymbolicReference("refs/sym/one", "refs/heads/main"))
		f.setRef(plumbing.NewSymbolicReference("refs/sym/two", "refs/sym/one"))
	case "EnumerationCanBeFilteredByPrefix":
		f.setRef(plumbing.NewHashReference("refs/heads/feature/a", head))
		f.tag("v1.0.0", head, false)
		f.tag("v2.0.0", head, false)
	}
	return f, head, tag
}

func contractTreeFixture(t *testing.T, method string) *contractFixture {
	t.Helper()
	f := newContractFixture(t)
	switch method {
	case "ChangedPathsMatchLibGit2ForEveryCommit":
		f.write("a.txt", "one\n")
		f.write("sub/b.txt", "one\n")
		f.write("sub/deep/c.txt", "one\n")
		f.commit("root commit")
		f.write("a.txt", "two\n")
		f.write("sub/new.txt", "new\n")
		f.commit("modify and add")
		f.remove("sub/b.txt")
		f.commit("delete nested file")
		f.write("d.txt", "d\n")
		f.stage(".")
		index, err := f.repo.Storer.Index()
		if err != nil {
			t.Fatal(err)
		}
		entry, err := index.Entry("a.txt")
		if err != nil {
			t.Fatal(err)
		}
		entry.Mode = filemode.Executable
		if err := f.repo.Storer.SetIndex(index); err != nil {
			t.Fatal(err)
		}
		f.commitIndex("exec bit and new file")
		if err := os.Chmod(filepath.Join(f.path, "a.txt"), 0755); err != nil {
			t.Fatal(err)
		}
	case "ChangedPathsMatchLibGit2WhenAFileBecomesADirectory":
		f.write("thing", "file\n")
		f.write("z.txt", "z\n")
		f.commit("file")
		f.remove("thing")
		f.write("thing/inner.txt", "dir\n")
		f.write("thing/other.txt", "dir\n")
		f.commit("becomes a directory")
		f.remove("thing/inner.txt")
		f.remove("thing/other.txt")
		if err := os.Remove(filepath.Join(f.path, "thing")); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		f.write("thing", "file again\n")
		f.commit("becomes a file again")
	case "ChangedPathsOfMergeCommitsUseTheFirstParent":
		f.write("main.txt", "0\n")
		f.commit("main 0")
		f.checkout("feature", true)
		f.write("feature.txt", "1\n")
		feature := f.commit("feature 1")
		f.checkout("main", false)
		f.write("main.txt", "1\n")
		main := f.commit("main 1")
		f.write("feature.txt", "1\n")
		f.commit("Merge branch 'feature'", main, feature)
	case "ChangedPathsAlignEntriesByRawByteOrderNotUtf16Order":
		f.write("\uE000.txt", "private use\n")
		f.write("\U0001F600.txt", "emoji\n")
		f.commit("both files")
		f.remove("\uE000.txt")
		f.commit("delete the private-use file")
	case "IdenticalTreesProduceNoChangedPaths":
		f.write("a.txt", "one\n")
		f.commit("commit")
	case "FlattenTreeListsAllBlobPaths":
		f.write("a.txt", "one\n")
		f.write("sub/b.txt", "one\n")
		f.write("sub/deep/c.txt", "one\n")
		f.commit("commit")
	default:
		t.Fatal("unhandled tree fixture", method)
	}
	return f
}

func contractStatusFixture(t *testing.T, method string) *contractFixture {
	t.Helper()
	f := newContractFixture(t)
	switch method {
	case "ACleanWorkingDirectoryHasNoChanges", "WorksAgainstAnIndexInVersion4Format":
		f.write("a.txt", "one\n")
		f.write("sub/b.txt", "two\n")
		f.commit("commit")
		if method == "WorksAgainstAnIndexInVersion4Format" {
			index, err := f.repo.Storer.Index()
			if err != nil {
				t.Fatal(err)
			}
			index.Version = 4
			if err := f.repo.Storer.SetIndex(index); err != nil {
				t.Fatal(err)
			}
			f.write("a.txt", "two\n")
			f.write("untracked.txt", "new\n")
		}
	case "CountsModifiedStagedDeletedAndUntrackedPathsOnce":
		for _, name := range []string{"modified.txt", "both.txt", "staged.txt", "deleted.txt", "removed.txt"} {
			f.write(name, "one\n")
		}
		f.commit("commit")
		f.write("modified.txt", "two\n")
		f.write("staged.txt", "two\n")
		f.stage("staged.txt")
		f.write("both.txt", "two\n")
		f.stage("both.txt")
		f.write("both.txt", "three\n")
		if err := os.Remove(filepath.Join(f.path, "deleted.txt")); err != nil {
			t.Fatal(err)
		}
		f.remove("removed.txt")
		f.write("untracked.txt", "new\n")
		f.write("sub/untracked.txt", "new\n")
	case "ContentRestoredToTheCommittedStateIsClean":
		f.write("a.txt", "one\n")
		f.commit("commit")
		f.write("a.txt", "one\n")
		touched := time.Unix(1735689700, 0)
		if err := os.Chtimes(filepath.Join(f.path, "a.txt"), touched, touched); err != nil {
			t.Fatal(err)
		}
	case "RespectsGitIgnoreRules":
		f.write(".gitignore", "*.log\nbuild/\n!important.log\n/anchored.txt\n")
		f.write("sub/.gitignore", "local-*\n")
		f.write("a.txt", "one\n")
		f.commit("commit")
		for name, contents := range map[string]string{
			"noise.log": "ignored\n", "important.log": "kept\n", "build/out.txt": "ignored\n", "anchored.txt": "ignored\n",
			"sub/anchored.txt": "not anchored\n", "sub/local-cache": "ignored\n", "sub/tracked-new.txt": "new\n",
		} {
			f.write(name, contents)
		}
	case "TreatsNonBoundaryConsecutiveAsterisksAsSlashExcluding":
		f.write(".gitignore", "out**txt\n")
		f.commit("commit")
		f.write("output.txt", "ignored\n")
		f.write("out/nested/file.txt", "kept\n")
	case "RespectsInfoExclude":
		f.write("a.txt", "one\n")
		f.commit("commit")
		f.write(".git/info/exclude", "*.tmp\n")
		f.write("scratch.tmp", "ignored\n")
		f.write("kept.txt", "untracked\n")
	case "HonorsCoreIgnoreCaseForIgnorePatterns", "MatchesIgnorePatternsCaseSensitivelyWhenIgnoreCaseIsOff":
		f.write(".gitignore", "BIN/\n*.LOG\n")
		f.commit("commit")
		ignoreCase := method == "HonorsCoreIgnoreCaseForIgnorePatterns"
		config, err := f.repo.Config()
		if err != nil {
			t.Fatal(err)
		}
		config.Raw.Section("core").SetOption("ignorecase", fmt.Sprint(ignoreCase))
		if err := f.repo.SetConfig(config); err != nil {
			t.Fatal(err)
		}
		contents := "kept\n"
		if ignoreCase {
			contents = "ignored\n"
		}
		f.write("bin/out.txt", contents)
		f.write("noise.log", contents)
		f.write("kept.txt", "untracked\n")
	case "CountsExecutableBitChanges":
		f.write("script.sh", "#!/bin/sh\n")
		f.commit("commit")
		if err := os.Chmod(filepath.Join(f.path, "script.sh"), 0744); err != nil {
			t.Fatal(err)
		}
	case "CountsAnUntrackedDirectorySymbolicLinkAsASingleEntry":
		f.write("target/one.txt", "one\n")
		f.write("target/two.txt", "two\n")
		f.commit("commit")
		for name, target := range map[string]string{"link": "target", "self": "."} {
			if err := os.Symlink(target, filepath.Join(f.path, name)); err != nil {
				t.Fatal(err)
			}
		}
	case "CountsUntrackedFilesInAnEmptyRepository":
		f.write("untracked.txt", "new\n")
		f.write("also-untracked.txt", "new\n")
	default:
		t.Fatal("unhandled status fixture", method)
	}
	return f
}

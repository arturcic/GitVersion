package gitrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
)

type revisionFixture struct {
	*contractFixture
	root, head, main, feature, olderMain, dev plumbing.Hash
	linear                                    []plumbing.Hash
}

func newRevisionFixture(t *testing.T, history string) *revisionFixture {
	t.Helper()
	f := &revisionFixture{contractFixture: newContractFixture(t)}
	writeCommit := func(path, content, message string, parents ...plumbing.Hash) plumbing.Hash {
		f.write(path, content+"\n")
		if history == "equal" && f.commits > 0 {
			f.commits--
		}
		return f.commit(message, parents...)
	}
	switch history {
	case "linear":
		for i := 0; i < 6; i++ {
			f.linear = append(f.linear, writeCommit("file.txt", fmt.Sprintf("content %d", i), fmt.Sprintf("commit %d", i)))
		}
		f.root, f.head = f.linear[0], f.linear[5]
	case "unrelated":
		f.head = writeCommit("a.txt", "a", "on main")
		f.setRef(plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/detached-root"))
		f.dev = writeCommit("b.txt", "b", "orphan root")
	case "merged", "equal":
		f.root = writeCommit("main.txt", "0", "main 0")
		f.checkout("feature", true)
		writeCommit("feature.txt", "1", "feature 1")
		f.feature = writeCommit("feature.txt", "2", "feature 2")
		f.checkout("main", false)
		f.olderMain = writeCommit("main.txt", "1", "main 1")
		f.main = writeCommit("main.txt", "2", "main 2")
		f.head = writeCommit("feature.txt", "2", "Merge branch 'feature'", f.main, f.feature)
		f.main = f.head
	case "criss-cross":
		f.root = writeCommit("main.txt", "0", "root")
		f.checkout("dev", true)
		dev := writeCommit("dev.txt", "1", "dev 1")
		f.checkout("main", false)
		main := writeCommit("main.txt", "1", "main 1")
		f.main = writeCommit("dev.txt", "1", "Merge dev", main, dev)
		f.checkout("dev", false)
		f.dev = writeCommit("main.txt", "1", "Merge main", dev, main)
		f.head = f.dev
	case "octopus":
		f.root = writeCommit("main.txt", "0", "root")
		f.checkout("b1", true)
		first := writeCommit("b1.txt", "1", "b1 1")
		f.checkout("main", false)
		f.checkout("b2", true)
		second := writeCommit("b2.txt", "1", "b2 1")
		f.checkout("main", false)
		main := writeCommit("main.txt", "1", "main 1")
		f.write("b1.txt", "1\n")
		f.head = writeCommit("b2.txt", "1", "Merge branches 'b1' and 'b2'", main, first, second)
	default:
		t.Fatalf("unknown history %s", history)
	}
	if history == "linear" || history == "merged" {
		f.tag("v0", f.root, false)
	}
	return f
}

func (f *revisionFixture) removeCommit(hash plumbing.Hash) {
	f.t.Helper()
	id := hash.String()
	if err := os.Remove(filepath.Join(f.path, ".git", "objects", id[:2], id[2:])); err != nil {
		f.t.Fatal(err)
	}
}

func (f *revisionFixture) openCommitRepository() *Repository {
	f.t.Helper()
	layout, err := discoverLayout(f.path)
	if err != nil {
		f.t.Fatal(err)
	}
	shallow, err := layout.readShallowCommits()
	if err != nil {
		f.t.Fatal(err)
	}
	g, err := layout.open()
	if err != nil {
		f.t.Fatal(err)
	}
	return newCommitRepository(layout, g, shallow)
}

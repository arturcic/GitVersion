package gitrepo

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

const (
	managedAuthor    = "Test Author"
	managedAuthorID  = "author@example.com"
	managedCommitter = "Test Committer"
	managedCommitID  = "committer@example.com"
)

type managedObjectFixture struct {
	*contractFixture
	tip plumbing.Hash
}

func newManagedObjectFixture(t *testing.T) *managedObjectFixture {
	t.Helper()
	return &managedObjectFixture{contractFixture: newContractFixture(t)}
}

func (f *managedObjectFixture) currentDate() time.Time {
	return time.Date(2023, 6, 1, 10, f.commits, 0, 0, time.FixedZone("+0200", 2*60*60))
}

func (f *managedObjectFixture) addCommit(message string) plumbing.Hash {
	f.t.Helper()
	f.stage(".")
	f.commits++
	author := &object.Signature{Name: managedAuthor, Email: managedAuthorID, When: f.currentDate()}
	committer := &object.Signature{Name: managedCommitter, Email: managedCommitID, When: f.currentDate()}
	hash, err := f.worktree.Commit(message+"\n", &git.CommitOptions{Author: author, Committer: committer})
	if err != nil {
		f.t.Fatal(err)
	}
	f.tip = hash
	return hash
}

func (f *managedObjectFixture) encodedCommit(rawMessage, encoding string) plumbing.Hash {
	f.t.Helper()
	f.addCommit("placeholder")
	commit, err := f.repo.CommitObject(f.tip)
	if err != nil {
		f.t.Fatal(err)
	}
	commit.Message, commit.Encoding = rawMessage, object.MessageEncoding(encoding)
	raw := f.repo.Storer.NewEncodedObject()
	if err := commit.Encode(raw); err != nil {
		f.t.Fatal(err)
	}
	f.tip, err = f.repo.Storer.SetEncodedObject(raw)
	if err != nil {
		f.t.Fatal(err)
	}
	f.setRef(plumbing.NewHashReference("refs/heads/main", f.tip))
	return f.tip
}

func (f *managedObjectFixture) annotatedTag(name, message string, target plumbing.Hash) plumbing.Hash {
	f.t.Helper()
	ref, err := f.repo.CreateTag(name, target, &git.CreateTagOptions{
		Tagger:  &object.Signature{Name: managedCommitter, Email: managedCommitID, When: f.currentDate()},
		Message: message,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return ref.Hash()
}

func (f *managedObjectFixture) openReader() *Repository {
	f.t.Helper()
	layout, err := openLayout(f.path)
	if err != nil {
		f.t.Fatal(err)
	}
	g, err := layout.open()
	if err != nil {
		f.t.Fatal(err)
	}
	return newCommitRepository(layout, g, nil)
}

func (f *managedObjectFixture) packObjects(refDeltas bool, window uint) int {
	f.t.Helper()
	store := f.repo.Storer.(*filesystem.Storage)
	var hashes []plumbing.Hash
	if err := store.ForEachObjectHash(func(hash plumbing.Hash) error {
		hashes = append(hashes, hash)
		return nil
	}); err != nil {
		f.t.Fatal(err)
	}
	slices.SortFunc(hashes, func(a, b plumbing.Hash) int { return bytes.Compare(a[:], b[:]) })
	var data bytes.Buffer
	if _, err := packfile.NewEncoder(&data, store, refDeltas).Encode(hashes, window); err != nil {
		f.t.Fatal(err)
	}
	depth := f.verifyPack(data.Bytes(), refDeltas, window != 0)
	writer, err := store.PackfileWriter()
	if err != nil {
		f.t.Fatal(err)
	}
	_, copyErr := io.Copy(writer, &data)
	if err := writer.Close(); err != nil || copyErr != nil {
		f.t.Fatalf("install pack: copy %v, close %v", copyErr, err)
	}
	for _, hash := range hashes {
		if err := store.DeleteLooseObject(hash); err != nil {
			f.t.Fatal(err)
		}
	}
	loose, err := filepath.Glob(filepath.Join(f.path, ".git", "objects", "[0-9a-f][0-9a-f]", "*"))
	if err != nil || len(loose) != 0 {
		f.t.Fatalf("expected pack-only objects, found %v, error %v", loose, err)
	}
	return depth
}

func (f *managedObjectFixture) verifyPack(data []byte, refDeltas, deltas bool) int {
	f.t.Helper()
	scanner := packfile.NewScanner(bytes.NewReader(data))
	_, count, err := scanner.Header()
	if err != nil {
		f.t.Fatal(err)
	}
	depths := map[int64]int{}
	depth, refs := 0, 0
	for i := uint32(0); i < count; i++ {
		header, err := scanner.NextObjectHeader()
		if err != nil {
			f.t.Fatal(err)
		}
		switch header.Type {
		case plumbing.OFSDeltaObject:
			d := depths[header.OffsetReference] + 1
			depths[header.Offset] = d
			depth = max(depth, d)
		case plumbing.REFDeltaObject:
			refs++
		}
	}
	if deltas && ((refDeltas && (refs == 0 || depth != 0)) || (!refDeltas && (depth < 2 || refs != 0))) {
		f.t.Fatalf("wrong pack shape: offset depth %d, reference deltas %d", depth, refs)
	}
	return depth
}

func (f *managedObjectFixture) growingHistory(count int) ([]plumbing.Hash, string) {
	f.t.Helper()
	var content strings.Builder
	ids := make([]plumbing.Hash, 0, count)
	for i := 0; i < count; i++ {
		fmt.Fprintf(&content, "line %d of a delta friendly file with some repeating content\n", i)
		f.write("file.txt", content.String())
		ids = append(ids, f.addCommit(fmt.Sprintf("commit %d", i)))
	}
	return ids, content.String()
}

func managedLooseFixture(t *testing.T, method string) (*managedObjectFixture, plumbing.Hash) {
	t.Helper()
	f := newManagedObjectFixture(t)
	f.write("file.txt", "content\n")
	var first plumbing.Hash
	switch method {
	case "ReadsACommitFromLooseObjects", "ReadsCommitsFromAPackFile":
		f.write("file.txt", "hello\n")
		first = f.addCommit("first commit")
		f.write("file.txt", "hello world\n")
		f.addCommit("second commit")
	case "ReadsARootCommitWithoutParents":
		f.write("file.txt", "hello\n")
		f.addCommit("initial")
	case "CommitMatchesGitLogOutput":
		f.addCommit("subject line\n\nbody line one\nbody line two")
	case "CommitMessageMatchesGitCatFileOutput":
		f.addCommit("a message with special characters: äöü — ✓")
	case "ReadsACommitMessageHonoringTheEncodingHeader":
		f.encodedCommit("caf\xe9 \xe0 la cr\xe8me\n", "ISO-8859-1")
	case "FallsBackToLatin1ForInvalidUtf8WithoutAnEncodingHeader":
		f.encodedCommit("caf\xe9\n", "")
	case "ReadsABlobFromLooseObjects":
		f.write("file.txt", "blob content\n")
		f.addCommit("add file")
	case "ReadsObjectsFromAnAlternateObjectDatabase":
		f.write("file.txt", "shared content\n")
		f.addCommit("shared commit")
	default:
		f.addCommit("a commit")
	}
	return f, first
}

func managedTreeFixture(t *testing.T, method string) *managedObjectFixture {
	t.Helper()
	f := newManagedObjectFixture(t)
	if method != "ReadsANestedTree" {
		f.write("a.txt", "a\n")
	}
	if method == "ReadsTheRootTreeEntriesMatchingGitLsTree" {
		f.write("b.txt", "b\n")
	}
	f.write("sub/c.txt", "c\n")
	message := "add files"
	if method == "ReadsANestedTree" {
		message = "add nested file"
	}
	f.addCommit(message)
	return f
}

func (f *managedObjectFixture) alternateReader() *Repository {
	f.t.Helper()
	dependent := newManagedObjectFixture(f.t)
	objects := filepath.Join(f.path, ".git", "objects")
	if _, err := os.Stat(objects); err != nil {
		f.t.Fatal(err)
	}
	dependent.write(".git/objects/info/alternates", objects+"\n")
	return dependent.openReader()
}

func managedTagFixture(t *testing.T, method string) (*managedObjectFixture, plumbing.Hash, plumbing.Hash) {
	t.Helper()
	f := newManagedObjectFixture(t)
	f.write("file.txt", "content\n")
	commit := f.addCommit("a commit")
	var tag plumbing.Hash
	switch method {
	case "ReadsAnAnnotatedTag":
		tag = f.annotatedTag("v1.2.3", "release 1.2.3", commit)
	case "ReadsAnAnnotatedTagWithAMultiLineMessage":
		tag = f.annotatedTag("v2.0.0", "subject\n\nbody with details", commit)
	case "ReadsANestedAnnotatedTag":
		inner := f.annotatedTag("inner", "inner tag", commit)
		tag = f.annotatedTag("outer", "outer tag", inner)
	case "ReadsAnAnnotatedTagFromAPackFile":
		tag = f.annotatedTag("v3.0.0", "packed tag", commit)
	default:
		t.Fatal("unknown tag fixture", method)
	}
	return f, commit, tag
}

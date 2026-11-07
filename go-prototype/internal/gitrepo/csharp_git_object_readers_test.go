package gitrepo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type objectReaderCorpus struct {
	Revision, Reference string
	ReferenceSHA256     string `json:"reference_sha256"`
	Suites              map[string]struct {
		Methods map[string]struct{ Cases []struct{ ID string } }
	}
}

type objectReaderReference struct {
	Revision string              `json:"revision"`
	Cases    map[string][]string `json:"cases"`
}

func readObjectReaderCorpus(t *testing.T) objectReaderCorpus {
	t.Helper()
	var corpus objectReaderCorpus
	if err := parity.ReadJSON("../../testdata/csharp-git-object-readers.json", &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Suites) != 4 {
		t.Fatal("original object-reader inventory changed")
	}
	return corpus
}

func readObjectReaderReference(t *testing.T, corpus objectReaderCorpus) objectReaderReference {
	t.Helper()
	path := filepath.Join("../..", corpus.Reference)
	hash, err := parity.FileHash(path)
	if err != nil || hash != corpus.ReferenceSHA256 {
		t.Fatal("Git object expectation hash differs", err)
	}
	var reference objectReaderReference
	if err := parity.ReadJSON(path, &reference); err != nil {
		t.Fatal(err)
	}
	if reference.Revision != corpus.Revision || len(reference.Cases) != 15 {
		t.Fatal("Git object expectation inventory changed")
	}
	return reference
}

func TestCSharpGitObjectReaders(t *testing.T) {
	corpus := readObjectReaderCorpus(t)
	reference := readObjectReaderReference(t, corpus)
	for suite, entry := range corpus.Suites {
		t.Run(suite, func(t *testing.T) {
			for method, spec := range entry.Methods {
				t.Run(method, func(t *testing.T) {
					for _, tc := range spec.Cases {
						t.Run(tc.ID, func(t *testing.T) {
							switch suite {
							case "LooseObjectTests":
								checkLooseObject(t, method, reference.Cases[method])
							case "PackedObjectTests":
								checkPackedObject(t, method, reference)
							case "TagTests":
								checkTagObject(t, method, reference.Cases[method])
							case "TreeTests":
								checkTreeObject(t, method, reference.Cases[method])
							default:
								t.Fatal("unknown suite", suite)
							}
						})
					}
				})
			}
		})
	}
}

func mustLoadCommit(t *testing.T, r *Repository, id plumbing.Hash) *object.Commit {
	t.Helper()
	commit, err := r.load(id.String())
	if err != nil {
		t.Fatal(err)
	}
	return commit
}

func mustLoadTree(t *testing.T, r *Repository, id string) *object.Tree {
	t.Helper()
	tree, err := r.git.TreeObject(plumbing.NewHash(id))
	if err != nil || tree.Hash.String() != id {
		t.Fatalf("tree lookup %s returned %v, error %v", id, tree, err)
	}
	return tree
}

func readObjectBlob(t *testing.T, r *Repository, id string) string {
	t.Helper()
	blob, err := r.git.BlobObject(plumbing.NewHash(id))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := blob.Reader()
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(reader)
	if err := reader.Close(); err != nil || readErr != nil {
		t.Fatalf("blob read %v, close %v", readErr, err)
	}
	return string(data)
}

func checkLooseObject(t *testing.T, method string, expected []string) {
	t.Helper()
	f, first := managedLooseFixture(t, method)
	r := f.openReader()
	switch method {
	case "ReadsACommitFromLooseObjects":
		c := mustLoadCommit(t, r, f.tip)
		if c.Hash != f.tip || c.TreeHash.String() != expected[0] || !slices.Equal(c.ParentHashes, []plumbing.Hash{first}) ||
			c.Message != "second commit\n" || c.Author.Name != managedAuthor || c.Author.Email != managedAuthorID ||
			c.Committer.Name != managedCommitter || c.Committer.Email != managedCommitID {
			t.Fatalf("loose commit differs: %+v", c)
		}
	case "ReadsARootCommitWithoutParents":
		c := mustLoadCommit(t, r, f.tip)
		if len(c.ParentHashes) != 0 || c.Message != "initial\n" {
			t.Fatalf("root commit differs: %+v", c)
		}
	case "CommitMatchesGitLogOutput":
		c := mustLoadCommit(t, r, f.tip)
		actual := []string{c.Author.Name, c.Author.Email, strconv.FormatInt(c.Author.When.Unix(), 10),
			c.Committer.Name, c.Committer.Email, strconv.FormatInt(c.Committer.When.Unix(), 10)}
		_, offset := c.Author.When.Zone()
		if !slices.Equal(actual, expected) || offset != 2*60*60 || !c.Author.When.Equal(f.currentDate()) ||
			c.Message != "subject line\n\nbody line one\nbody line two\n" {
			t.Fatalf("Git log expected %v, got %v, offset %d, message %q", expected, actual, offset, c.Message)
		}
	case "CommitMessageMatchesGitCatFileOutput":
		c := mustLoadCommit(t, r, f.tip)
		if strings.TrimRight(c.Message, "\n") != expected[0] {
			t.Fatalf("raw Git message %q, decoded %q", expected[0], c.Message)
		}
	case "ReadsACommitMessageHonoringTheEncodingHeader":
		c := mustLoadCommit(t, r, f.tip)
		if "encoding "+string(c.Encoding) != expected[0] || c.Message != expected[1]+"\n" ||
			c.Message != "café à la crème\n" {
			t.Fatalf("decoded commit encoding %q message %q", c.Encoding, c.Message)
		}
	case "FallsBackToLatin1ForInvalidUtf8WithoutAnEncodingHeader":
		c := mustLoadCommit(t, r, f.tip)
		if c.Message != "café\n" {
			t.Fatalf("decoded unmarked commit encoding %q message %q", c.Encoding, c.Message)
		}
	case "ReadsABlobFromLooseObjects":
		if got := readObjectBlob(t, r, expected[0]); got != "blob content\n" {
			t.Fatalf("blob content %q", got)
		}
	case "ReportsTheObjectTypeWhenItIsNotKnownUpFront":
		obj, err := r.git.Storer.EncodedObject(plumbing.AnyObject, f.tip)
		if err != nil || obj.Type() != plumbing.CommitObject {
			t.Fatalf("expected commit object, got %v, error %v", obj, err)
		}
	case "DoesNotFindAMissingObject":
		const missing = "0123456789abcdef0123456789abcdef01234567"
		_, lookupErr := r.git.Storer.EncodedObject(plumbing.CommitObject, plumbing.NewHash(missing))
		_, readErr := r.load(missing)
		if !errors.Is(lookupErr, plumbing.ErrObjectNotFound) || !errors.Is(readErr, plumbing.ErrObjectNotFound) {
			t.Fatalf("expected missing object errors, got %v and %v", lookupErr, readErr)
		}
	case "DoesNotFindAnObjectWhenRequestedWithTheWrongType":
		if _, err := r.git.Storer.EncodedObject(plumbing.BlobObject, f.tip); !errors.Is(err, plumbing.ErrObjectNotFound) {
			t.Fatalf("wrong type returned %v", err)
		}
	case "ReadsObjectsFromAnAlternateObjectDatabase":
		c := mustLoadCommit(t, f.alternateReader(), f.tip)
		if c.Message != "shared commit\n" {
			t.Fatalf("alternate commit message %q", c.Message)
		}
	default:
		t.Fatal("unknown loose-object method", method)
	}
}

func checkPackedObject(t *testing.T, method string, reference objectReaderReference) {
	t.Helper()
	switch method {
	case "ReadsCommitsFromAPackFile":
		f, first := managedLooseFixture(t, method)
		f.packObjects(false, 0)
		c := mustLoadCommit(t, f.openReader(), f.tip)
		if c.TreeHash.String() != reference.Cases[method][0] || !slices.Equal(c.ParentHashes, []plumbing.Hash{first}) ||
			c.Message != "second commit\n" || c.Author.Name != managedAuthor || !c.Committer.When.Equal(f.currentDate()) {
			t.Fatalf("packed commit differs: %+v", c)
		}
	case "ReadsDeltifiedObjectsFromAPackWithDeepDeltaChains", "ReadsRefDeltaObjectsFromAPackFile":
		f := newManagedObjectFixture(t)
		ids, _ := f.growingHistory(30)
		depth := f.packObjects(method == "ReadsRefDeltaObjectsFromAPackFile", 50)
		t.Logf("offset delta depth %d", depth)
		r := f.openReader()
		blobs := reference.Cases["PackedDeltaHistoryBlobs"]
		if len(blobs) != len(ids) {
			t.Fatal("Git history blob inventory differs")
		}
		for i, id := range ids {
			c := mustLoadCommit(t, r, id)
			var parents []plumbing.Hash
			if i > 0 {
				parents = ids[i-1 : i]
			}
			if c.Message != fmt.Sprintf("commit %d\n", i) || !slices.Equal(c.ParentHashes, parents) {
				t.Fatalf("packed commit %d differs: %+v", i, c)
			}
			text := readObjectBlob(t, r, blobs[i])
			if lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' }); len(lines) != i+1 {
				t.Fatalf("blob %d: expected %d lines, got %d", i, i+1, len(lines))
			}
		}
	case "ReadsBlobsAndTreesFromAPackFile":
		f := newManagedObjectFixture(t)
		_, content := f.growingHistory(20)
		f.packObjects(false, 50)
		r := f.openReader()
		ids := reference.Cases["PackedTwentyCommitTree"]
		if got := readObjectBlob(t, r, ids[1]); got != content {
			t.Fatal("packed blob bytes differ")
		}
		tree := mustLoadTree(t, r, ids[0])
		if len(tree.Entries) != 1 || tree.Entries[0].Name != "file.txt" {
			t.Fatalf("packed tree entries %v", tree.Entries)
		}
	case "ReadsLooseObjectsCreatedAfterThePack":
		f := newManagedObjectFixture(t)
		f.write("file.txt", "hello\n")
		packed := f.addCommit("packed commit")
		f.packObjects(false, 0)
		f.write("file.txt", "hello again\n")
		loose := f.addCommit("loose commit")
		r := f.openReader()
		if a, b := mustLoadCommit(t, r, packed), mustLoadCommit(t, r, loose); a.Message != "packed commit\n" || b.Message != "loose commit\n" {
			t.Fatalf("mixed packed/loose commits: %q %q", a.Message, b.Message)
		}
	default:
		t.Fatal("unknown packed-object method", method)
	}
}

func checkTagObject(t *testing.T, method string, expected []string) {
	t.Helper()
	f, commit, id := managedTagFixture(t, method)
	if method == "ReadsAnAnnotatedTagFromAPackFile" {
		f.packObjects(false, 0)
	}
	r := f.openReader()
	tag, err := r.git.TagObject(id)
	if err != nil || id.String() != expected[0] || commit.String() != expected[1] {
		t.Fatalf("Git tag IDs %v, created %s and %s, error %v", expected, id, commit, err)
	}
	switch method {
	case "ReadsAnAnnotatedTag":
		if id == commit || tag.Hash != id || tag.Name != "v1.2.3" || tag.TargetType != plumbing.CommitObject ||
			tag.Target != commit || tag.Message != "release 1.2.3\n" ||
			tag.Tagger.Name != managedCommitter || tag.Tagger.Email != managedCommitID || !tag.Tagger.When.Equal(f.currentDate()) {
			t.Fatalf("annotated tag differs: %+v", tag)
		}
	case "ReadsAnAnnotatedTagWithAMultiLineMessage":
		if tag.Message != "subject\n\nbody with details\n" {
			t.Fatalf("annotated multiline message %q", tag.Message)
		}
	case "ReadsANestedAnnotatedTag":
		if tag.TargetType != plumbing.TagObject || tag.Target.String() != expected[2] {
			t.Fatalf("nested tag target differs: %+v", tag)
		}
		refs, err := r.References()
		if err != nil {
			t.Fatal(err)
		}
		tags, err := r.Tags(refs)
		if err != nil || !slices.Contains(tags, Tag{"refs/tags/outer", expected[2], expected[1]}) {
			t.Fatalf("one-hop and peeled targets differ: %v, error %v", tags, err)
		}
	case "ReadsAnAnnotatedTagFromAPackFile":
		if tag.Name != "v3.0.0" || tag.Target != commit || tag.Message != "packed tag\n" {
			t.Fatalf("packed tag differs: %+v", tag)
		}
	}
}

func checkTreeObject(t *testing.T, method string, expected []string) {
	t.Helper()
	f := managedTreeFixture(t, method)
	r := f.openReader()
	tree := mustLoadTree(t, r, expected[0])
	switch method {
	case "ReadsTheRootTreeEntriesMatchingGitLsTree":
		var actual []string
		for _, entry := range tree.Entries {
			kind := "blob"
			if entry.Mode == filemode.Dir {
				kind = "tree"
			}
			actual = append(actual, fmt.Sprintf("%06o %s %s\t%s", entry.Mode, kind, entry.Hash, entry.Name))
		}
		if !slices.Equal(actual, expected[1:]) {
			t.Fatalf("Git ls-tree expected %v, got %v", expected[1:], actual)
		}
	case "ReadsANestedTree":
		entry, err := tree.FindEntry("sub")
		if err != nil || entry.Mode != filemode.Dir || entry.Hash.String() != expected[1] {
			t.Fatalf("subtree entry %v, error %v", entry, err)
		}
		sub := mustLoadTree(t, r, expected[1])
		if len(sub.Entries) != 1 || sub.Entries[0].Name != "c.txt" ||
			sub.Entries[0].Mode == filemode.Dir || sub.Entries[0].Hash.String() != expected[2] {
			t.Fatalf("nested tree entries %v", sub.Entries)
		}
	case "FindsANodeUsingTheStreamingReader":
		entry, err := tree.FindEntry("sub")
		if err != nil || entry.Hash.String() != expected[1] {
			t.Fatalf("sub lookup %v, error %v", entry, err)
		}
		if _, err := tree.FindEntry("missing"); !errors.Is(err, object.ErrEntryNotFound) {
			t.Fatalf("missing node returned %v", err)
		}
	}
}

func TestDeclaredLatin1TakesPrecedenceOverUTF8(t *testing.T) {
	f := newManagedObjectFixture(t)
	f.write("file.txt", "content\n")
	f.encodedCommit("caf\xc3\xa9\n", "ISO-8859-1")
	if got := mustLoadCommit(t, f.openReader(), f.tip).Message; got != "cafÃ©\n" {
		t.Fatalf("declared Latin-1 decoded as %q", got)
	}
}

// Check the normal repository entry point when both HEAD and its tree are
// available only through the alternate.
func TestOpenWithAnAlternateObjectDatabase(t *testing.T) {
	f, _ := managedLooseFixture(t, "ReadsObjectsFromAnAlternateObjectDatabase")
	alternate := filepath.Join(t.TempDir(), "objects")
	objects := filepath.Join(f.path, ".git", "objects")
	if err := os.Rename(objects, alternate); err != nil {
		t.Fatal(err)
	}
	f.write(".git/objects/info/alternates", alternate+"\n")
	repo, err := Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	_, head := repo.Head()
	commit, err := repo.Commit(head)
	if err != nil || head != f.tip.String() || commit.Message != "shared commit\n" ||
		!slices.Equal(commit.DiffPaths, []string{"file.txt"}) {
		t.Fatalf("alternate-backed repository: head %s, commit %+v, error %v", head, commit, err)
	}
}

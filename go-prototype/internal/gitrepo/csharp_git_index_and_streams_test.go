package gitrepo

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	gitsync "github.com/go-git/go-git/v5/utils/sync"
)

type indexStreamCorpus struct {
	Revision, Reference string
	ReferenceSHA256     string `json:"reference_sha256"`
	Suites              map[string]struct {
		Methods map[string]struct {
			Cases []struct {
				ID       string
				Argument json.RawMessage
			}
		}
	}
}

type indexReference struct {
	Revision string              `json:"revision"`
	Versions map[string][]string `json:"versions"`
}

func readIndexStreamCorpus(t *testing.T) indexStreamCorpus {
	t.Helper()
	var c indexStreamCorpus
	if err := parity.ReadJSON("../../testdata/csharp-git-index-and-streams.json", &c); err != nil {
		t.Fatal(err)
	}
	if c.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(c.Suites) != 4 {
		t.Fatal("pinned index and stream inventory changed")
	}
	return c
}

func readIndexReference(t *testing.T, c indexStreamCorpus) indexReference {
	t.Helper()
	path := filepath.Join("../..", c.Reference)
	hash, err := parity.FileHash(path)
	if err != nil || hash != c.ReferenceSHA256 {
		t.Fatal("Git index expectations changed", err)
	}
	var reference indexReference
	if err := parity.ReadJSON(path, &reference); err != nil {
		t.Fatal(err)
	}
	if reference.Revision != c.Revision || len(reference.Versions) != 3 {
		t.Fatal("Git index inventory changed")
	}
	return reference
}

func TestCSharpGitIndexAndStreams(t *testing.T) {
	c := readIndexStreamCorpus(t)
	ref := readIndexReference(t, c)
	for suite, spec := range c.Suites {
		t.Run(suite, func(t *testing.T) {
			for method, m := range spec.Methods {
				t.Run(method, func(t *testing.T) {
					for _, tc := range m.Cases {
						t.Run(tc.ID, func(t *testing.T) {
							switch suite {
							case "GitIndexTests":
								checkGitIndex(t, method, tc.Argument, ref)
							case "DeltaStreamReaderTests":
								checkDeltaStream(t, method)
							case "RandomAccessStreamTests":
								checkRandomAccess(t, method, tc.Argument)
							case "GitZLibStreamTests":
								checkZLibStream(t, method, tc.Argument)
							default:
								t.Fatal("unknown source suite", suite)
							}
						})
					}
				})
			}
		})
	}
}

func indexVersionFixture(t *testing.T, version uint32) *managedObjectFixture {
	t.Helper()
	f := newManagedObjectFixture(t)
	f.write("a.txt", "one\n")
	f.write("sub/b.txt", "two\n")
	f.write("sub/deep/c.txt", "three\n")
	f.addCommit("commit")
	idx, err := f.repo.Storer.Index()
	if err != nil {
		t.Fatal(err)
	}
	idx.Version = version
	for _, entry := range idx.Entries {
		switch entry.Name {
		case "a.txt":
			entry.Mode = filemode.Executable
		case "sub/b.txt":
			entry.SkipWorktree = version == 3
		}
	}
	if err := f.repo.Storer.SetIndex(idx); err != nil {
		t.Fatal(err)
	}
	return f
}

func checkGitIndex(t *testing.T, method string, argument json.RawMessage, reference indexReference) {
	t.Helper()
	switch method {
	case "IndexEntriesMatchGitLsFiles":
		var version uint32
		if err := json.Unmarshal(argument, &version); err != nil {
			t.Fatal(err)
		}
		f := indexVersionFixture(t, version)
		idx, err := f.openReader().git.Storer.Index()
		if err != nil || idx.Version != version {
			t.Fatalf("expected index version %d, got %v, error %v", version, idx, err)
		}
		var actual []string
		for _, entry := range idx.Entries {
			actual = append(actual, fmt.Sprintf("%06o %s %d\t%s", entry.Mode, entry.Hash, entry.Stage, entry.Name))
		}
		want := reference.Versions[strconv.FormatUint(uint64(version), 10)]
		a, aErr := idx.Entry("a.txt")
		b, bErr := idx.Entry("sub/b.txt")
		if !slices.Equal(actual, want) || aErr != nil || bErr != nil ||
			a.Mode != filemode.Executable || b.Mode == filemode.Executable {
			t.Fatalf("index %d expected %v, got %v", version, want, actual)
		}
	case "ReadsStatDataUsedForCleanDetection":
		f := newManagedObjectFixture(t)
		f.write("a.txt", "content\n")
		f.addCommit("commit")
		idx, err := f.openReader().git.Storer.Index()
		if err != nil || len(idx.Entries) != 1 {
			t.Fatalf("expected one index entry, got %v, error %v", idx, err)
		}
		entry := idx.Entries[0]
		flags, err := readIndexAssumeValid(filepath.Join(f.path, ".git", "index"))
		if err != nil || entry.Size != uint32(len("content\n")) || entry.ModifiedAt.Unix() <= 0 ||
			flags[entry.Name] || entry.SkipWorktree || entry.IntentToAdd {
			t.Fatalf("stat data or flags differ: %v, assume valid %v, error %v", entry, flags, err)
		}
	case "ReadsSkipWorktreeExtendedFlags":
		f := newManagedObjectFixture(t)
		f.write("a.txt", "one\n")
		f.write("b.txt", "two\n")
		f.addCommit("commit")
		idx, err := f.repo.Storer.Index()
		if err != nil {
			t.Fatal(err)
		}
		idx.Version = 3
		a, err := idx.Entry("a.txt")
		if err != nil {
			t.Fatal(err)
		}
		a.SkipWorktree = true
		if err := f.repo.Storer.SetIndex(idx); err != nil {
			t.Fatal(err)
		}
		got, err := f.openReader().git.Storer.Index()
		if err != nil {
			t.Fatal(err)
		}
		readA, aErr := got.Entry("a.txt")
		readB, bErr := got.Entry("b.txt")
		if aErr != nil || bErr != nil || !readA.SkipWorktree || readB.SkipWorktree {
			t.Fatalf("skip-worktree flags differ: %v and %v", readA, readB)
		}
	case "AMissingIndexFileYieldsAnEmptyIndex":
		f := newManagedObjectFixture(t)
		idx, err := f.openReader().git.Storer.Index()
		if err != nil || len(idx.Entries) != 0 {
			t.Fatalf("missing index returned %v, error %v", idx, err)
		}
	default:
		t.Fatal("unknown index method", method)
	}
}

func checkDeltaStream(t *testing.T, method string) {
	t.Helper()
	source := make([]byte, 128)
	for i := range source {
		source[i] = byte(i)
	}
	base := &plumbing.MemoryObject{}
	base.SetType(plumbing.BlobObject)
	if _, err := base.Write(source); err != nil {
		t.Fatal(err)
	}
	// The pack reader consumes two size varints before the original instruction.
	delta := []byte{0x80, 0x01, 0x07, 0x91, 0x42, 0x07}
	switch method {
	case "ReturnsNullAtTheEndOfTheStream":
		delta = []byte{0x80, 0x01, 0}
	case "ThrowsWhenTheStreamEndsInsideACopyInstruction":
		delta = delta[:len(delta)-1]
	case "ReadsACompleteCopyInstruction":
	default:
		t.Fatal("unknown delta method", method)
	}
	// Filesystem storage uses this buffered decoder with the default object limit.
	target := &plumbing.MemoryObject{}
	err := packfile.ApplyDelta(target, base, delta)
	if method == "ThrowsWhenTheStreamEndsInsideACopyInstruction" {
		if !errors.Is(err, packfile.ErrInvalidDelta) || target.Size() != 0 {
			t.Fatalf("truncated instruction wrote %d bytes, error %v", target.Size(), err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	reader, err := target.Reader()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if method == "ReturnsNullAtTheEndOfTheStream" {
		if n, err := reader.Read(make([]byte, 1)); n != 0 || !errors.Is(err, io.EOF) {
			t.Fatalf("empty instruction stream returned %d bytes and %v", n, err)
		}
		return
	}
	data, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(data, source[0x42:0x42+0x07]) {
		t.Fatalf("copy offset/size differ: data %x, error %v", data, err)
	}
}

func checkRandomAccess(t *testing.T, method string, argument json.RawMessage) {
	t.Helper()
	var data []byte
	var length int64
	switch method {
	case "ReadsUnsignedBytesAndAdvancesPositionAlongsideSpanReads":
		data = []byte{0, 128, 255}
		length = int64(len(data))
	case "ReturnsEndOfStreamAtTheDeclaredLengthWithoutAdvancing":
		data = []byte{1, 2, 3}
		if err := json.Unmarshal(argument, &length); err != nil {
			t.Fatal(err)
		}
	case "ReturnsEndOfStreamWhenTheFileIsShorterThanTheDeclaredLength":
		data, length = []byte{42}, 2
	case "KeepsPositionsIndependentAndLeavesTheSharedHandleOpen":
		data, length = []byte{10, 20, 30}, 3
	case "ReadsBytesWithoutAllocatingPerCall":
		length = 2048
		data = make([]byte, length)
	default:
		t.Fatal("unknown random-access method", method)
	}
	path := filepath.Join(t.TempDir(), "bytes")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	view := io.NewSectionReader(file, 0, length)
	seek := func(s *io.SectionReader, offset int64, whence int) int64 {
		t.Helper()
		at, err := s.Seek(offset, whence)
		if err != nil {
			t.Fatal(err)
		}
		return at
	}
	position := func(s *io.SectionReader, expected int64) {
		t.Helper()
		if at := seek(s, 0, io.SeekCurrent); at != expected {
			t.Fatalf("view position %d, expected %d", at, expected)
		}
	}
	var buf [1]byte
	read := func(s *io.SectionReader, expected byte) {
		t.Helper()
		if n, err := s.Read(buf[:]); n != 1 || err != nil || buf[0] != expected {
			t.Fatalf("read byte %d/%d, error %v, want %d", buf[0], n, err, expected)
		}
	}
	eof := func() {
		t.Helper()
		if n, err := view.Read(buf[:]); n != 0 || !errors.Is(err, io.EOF) {
			t.Fatalf("expected EOF, got %d bytes and %v", n, err)
		}
	}
	switch method {
	case "ReadsUnsignedBytesAndAdvancesPositionAlongsideSpanReads":
		read(view, 0)
		position(view, 1)
		span := make([]byte, 1)
		if n, err := view.Read(span); n != 1 || err != nil || span[0] != 128 {
			t.Fatalf("span read %v, count %d, error %v", span, n, err)
		}
		read(view, 255)
		position(view, 3)
	case "ReturnsEndOfStreamAtTheDeclaredLengthWithoutAdvancing":
		seek(view, length, io.SeekStart)
		eof()
		eof()
		position(view, length)
	case "ReturnsEndOfStreamWhenTheFileIsShorterThanTheDeclaredLength":
		read(view, 42)
		eof()
		position(view, 1)
	case "KeepsPositionsIndependentAndLeavesTheSharedHandleOpen":
		other := io.NewSectionReader(file, 0, length)
		seek(view, 2, io.SeekStart)
		read(view, 30)
		read(other, 10)
		position(other, 1)
		seek(view, 1, io.SeekStart)
		// The SectionReader has no Close method and does not own the shared file.
		read(view, 20)
		position(view, 2)
	case "ReadsBytesWithoutAllocatingPerCall":
		var readErr error
		allocs := testing.AllocsPerRun(1, func() {
			// AllocsPerRun executes the warm-up loop before measuring.
			for i := 0; i < 1024; i++ {
				if _, err := view.Read(buf[:]); err != nil {
					readErr = err
					return
				}
			}
		})
		if allocs != 0 || readErr != nil {
			t.Fatalf("expected zero allocations, got %.0f, error %v", allocs, readErr)
		}
		position(view, 2048)
	}
}

func checkZLibStream(t *testing.T, method string, argument json.RawMessage) {
	t.Helper()
	raw := "18954BCF2C294B2D2ACECCCF53C84DCC4B4C4F4D51A8CAC94C02006F440909"
	if method == "RejectsInvalidOrUnsupportedHeaders" {
		if err := json.Unmarshal(argument, &raw); err != nil {
			t.Fatal(err)
		}
	} else if method != "DecompressesAStreamWithASmallerWindowSize" {
		t.Fatal("unknown zlib method", method)
	}
	compressed, err := hex.DecodeString(raw)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gitsync.GetZlibReader(bytes.NewReader(compressed))
	defer gitsync.PutZlibReader(reader)
	if method == "RejectsInvalidOrUnsupportedHeaders" {
		if err == nil {
			t.Fatal("invalid zlib header accepted", raw)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader.Reader)
	if err != nil || string(got) != "gitversion managed zlib" {
		t.Fatalf("small-window zlib returned %q, error %v", got, err)
	}
}

func assumeValidFixture(t *testing.T, version uint32, staged bool) *managedObjectFixture {
	t.Helper()
	f := newManagedObjectFixture(t)
	f.write("GitVersion.yml", "workflow: GitFlow/v1\n")
	f.write("a.txt", "one\n")
	f.write("b.txt", "two\n")
	f.addCommit("commit")
	if staged {
		f.write("b.txt", "staged\n")
		f.stage("b.txt")
	}
	idx, err := f.repo.Storer.Index()
	if err != nil {
		t.Fatal(err)
	}
	idx.Version = version
	if err := f.repo.Storer.SetIndex(idx); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.path, ".git", "index")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// The tracked config sorts first, and the test files share no path prefix.
	offset := 12
	previous := ""
	for _, entry := range idx.Entries {
		if entry.Name == "a.txt" || entry.Name == "b.txt" {
			raw[offset+60] |= 0x80
		}
		if version == 4 {
			prefix := 0
			for prefix < min(len(previous), len(entry.Name)) && previous[prefix] == entry.Name[prefix] {
				prefix++
			}
			offset += 62 + 1 + len(entry.Name) - prefix + 1
		} else {
			offset += (62 + len(entry.Name) + 8) &^ 7
		}
		previous = entry.Name
	}
	checksum := sha1.Sum(raw[:len(raw)-sha1.Size])
	copy(raw[len(raw)-sha1.Size:], checksum[:])
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	f.write("a.txt", "unstaged\n")
	f.write("b.txt", "different from the index\n")
	return f
}

func TestAssumeValidSuppressesOnlyUnstagedChanges(t *testing.T) {
	for _, version := range []uint32{2, 3, 4} {
		t.Run(fmt.Sprintf("index-v%d", version), func(t *testing.T) {
			f := assumeValidFixture(t, version, true)
			layout, err := openLayout(f.path)
			if err != nil {
				t.Fatal(err)
			}
			status, err := workingTreeStatus(layout, f.openReader().git)
			if err != nil {
				t.Fatal(err)
			}
			if a := status["a.txt"]; a == nil || a.Staging != git.Unmodified || a.Worktree != git.Unmodified {
				t.Fatalf("assume-valid worktree status %v", a)
			}
			if b := status["b.txt"]; b == nil || b.Staging != git.Modified || b.Worktree != git.Unmodified {
				t.Fatalf("assume-valid staged status %v", b)
			}
		})
	}
}

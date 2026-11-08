package gitrepo

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type contractCorpus struct {
	Revision        string
	Reference       string
	ReferenceSHA256 string `json:"reference_sha256"`
	Suites          map[string]struct {
		Methods map[string]struct {
			Cases []struct {
				ID            string
				ExpectedCount *int `json:"expected_count"`
			}
		}
	}
}

type contractReference struct {
	Revision string                     `json:"revision"`
	Cases    map[string]json.RawMessage `json:"cases"`
}

func readContractCorpus(t *testing.T) contractCorpus {
	t.Helper()
	var corpus contractCorpus
	if err := parity.ReadJSON("../../testdata/csharp-repository-contracts.json", &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Suites) != 3 {
		t.Fatal("source inventory changed")
	}
	return corpus
}

func readContractReference(t *testing.T, corpus contractCorpus) contractReference {
	t.Helper()
	path := filepath.Join("../..", corpus.Reference)
	hash, err := parity.FileHash(path)
	if err != nil || hash != corpus.ReferenceSHA256 {
		t.Fatal("oracle reference hash differs", err)
	}
	var reference contractReference
	if err := parity.ReadJSON(path, &reference); err != nil {
		t.Fatal(err)
	}
	if reference.Revision != corpus.Revision || len(reference.Cases) != 19 {
		t.Fatal("oracle reference inventory changed")
	}
	return reference
}

func TestCSharpRepositoryContracts(t *testing.T) {
	corpus := readContractCorpus(t)
	reference := readContractReference(t, corpus)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for suite, entry := range corpus.Suites {
		t.Run(suite, func(t *testing.T) {
			for method, spec := range entry.Methods {
				t.Run(method, func(t *testing.T) {
					for _, tc := range spec.Cases {
						t.Run(tc.ID, func(t *testing.T) {
							expected := reference.Cases[method]
							switch suite {
							case "GitReferenceStoreTests":
								checkReferenceContract(t, method, expected)
							case "GitTreeDiffTests":
								checkTreeContract(t, method, expected)
							case "GitStatusCalculatorTests":
								if tc.ExpectedCount == nil {
									t.Fatal("original status count missing")
								}
								checkStatusContract(t, method, expected, *tc.ExpectedCount)
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

func checkReferenceContract(t *testing.T, method string, expected json.RawMessage) {
	t.Helper()
	f, head, tag := contractReferenceFixture(t, method)
	store := f.store()
	lookup := func(name plumbing.ReferenceName) *plumbing.Reference {
		t.Helper()
		ref, err := store.lookup(name)
		if err != nil || ref == nil {
			t.Fatal(ref, err)
		}
		return ref
	}
	resolve := func(name plumbing.ReferenceName, wantName plumbing.ReferenceName) {
		t.Helper()
		ref, err := store.resolve(name)
		if err != nil || ref == nil || ref.Name() != wantName || ref.Hash() != head {
			t.Fatalf("resolved %q to %v, %v", name, ref, err)
		}
	}
	list := func(prefix string) []*plumbing.Reference {
		t.Helper()
		refs, err := store.list(prefix)
		if err != nil {
			t.Fatal(err)
		}
		return refs
	}
	listNames := func(prefix string, want []string) {
		t.Helper()
		names := []string{}
		for _, ref := range list(prefix) {
			names = append(names, ref.Name().String())
		}
		if !slices.Equal(names, want) {
			t.Fatalf("want %v, got %v", want, names)
		}
	}
	const mainRef = "refs/heads/main"
	switch method {
	case "ResolvesALooseBranchToItsCommit":
		ref := lookup(mainRef)
		if ref.Type() != plumbing.HashReference || ref.Hash() != head {
			t.Fatal(ref)
		}
		resolve(mainRef, mainRef)
	case "EnumerationMatchesGitForEachRef":
		var want []string
		if err := json.Unmarshal(expected, &want); err != nil {
			t.Fatal(err)
		}
		actual := []string{}
		for _, ref := range list("refs/") {
			actual = append(actual, ref.Name().String()+" "+ref.Hash().String())
		}
		if !slices.Equal(actual, want) {
			t.Fatalf("git for-each-ref: want %v, got %v", want, actual)
		}
	case "EnumerationSkipsTransientLockFiles":
		listNames("refs/", []string{mainRef})
		opened, err := Open(f.path)
		if err != nil {
			t.Fatal(err)
		}
		refs, err := opened.References()
		if err != nil || len(refs) != 1 || refs[0].Name != mainRef {
			t.Fatal(refs, err)
		}
	case "ReadsPackedRefsIncludingPeeledTargets":
		if _, err := os.Stat(filepath.Join(f.path, ".git", mainRef)); !os.IsNotExist(err) {
			t.Fatal("branch must be packed", err)
		}
		resolve(mainRef, mainRef)
		ref := lookup("refs/tags/v1.0.0")
		var want []string
		if err := json.Unmarshal(expected, &want); err != nil || len(want) != 2 {
			t.Fatal(want, err)
		}
		peeled, err := store.packedTarget("refs/tags/v1.0.0")
		if err != nil || peeled == nil || ref.Hash() != tag || ref.Hash().String() != want[0] || peeled.String() != want[1] {
			t.Fatal(ref, peeled, err)
		}
	case "LooseReferencesWinOverPackedReferences":
		resolve(mainRef, mainRef)
		refs := list("refs/heads/")
		if len(refs) != 1 || refs[0].Hash() != head {
			t.Fatal(refs)
		}
	case "HeadIsSymbolicWhenABranchIsCheckedOut":
		ref := lookup(plumbing.HEAD)
		if ref.Type() != plumbing.SymbolicReference || ref.Target() != mainRef {
			t.Fatal(ref)
		}
		resolve(plumbing.HEAD, mainRef)
	case "HeadIsDirectWhenDetached":
		ref := lookup(plumbing.HEAD)
		if ref.Type() != plumbing.HashReference || ref.Hash() != head {
			t.Fatal(ref)
		}
	case "HeadOfAnUnbornBranchResolvesToNull":
		if ref := lookup(plumbing.HEAD); ref.Type() != plumbing.SymbolicReference {
			t.Fatal(ref)
		}
		if ref, err := store.resolve(plumbing.HEAD); ref != nil || !errors.Is(err, plumbing.ErrReferenceNotFound) {
			t.Fatal(ref, err)
		}
	case "FollowsChainsOfSymbolicReferences":
		resolve("refs/sym/two", mainRef)
	case "ReturnsNullForAMissingReference":
		for _, read := range []func(plumbing.ReferenceName) (*plumbing.Reference, error){store.lookup, store.resolve} {
			if ref, err := read("refs/heads/does-not-exist"); ref != nil || !errors.Is(err, plumbing.ErrReferenceNotFound) {
				t.Fatal(ref, err)
			}
		}
	case "EnumerationCanBeFilteredByPrefix":
		listNames("refs/tags/", []string{"refs/tags/v1.0.0", "refs/tags/v2.0.0"})
		listNames("refs/heads/", []string{"refs/heads/feature/a", mainRef})
	case "ResolvedHeadCanBeReadFromTheObjectStore":
		resolve(plumbing.HEAD, mainRef)
		ref, err := store.resolve(plumbing.HEAD)
		if err != nil {
			t.Fatal(err)
		}
		commit, err := f.repo.CommitObject(ref.Hash())
		if err != nil || commit.Message != "readable through refs\n" {
			t.Fatal(commit, err)
		}
	default:
		t.Fatal("unhandled reference method", method)
	}
}

func checkTreeContract(t *testing.T, method string, expected json.RawMessage) {
	t.Helper()
	f := contractTreeFixture(t, method)
	opened, err := Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	switch method {
	case "IdenticalTreesProduceNoChangedPaths", "FlattenTreeListsAllBlobPaths":
		commit, err := opened.load(opened.head)
		if err != nil {
			t.Fatal(err)
		}
		tree, err := commit.Tree()
		if err != nil {
			t.Fatal(err)
		}
		if method == "IdenticalTreesProduceNoChangedPaths" {
			if paths, err := changedTreePaths(tree, tree); err != nil || len(paths) != 0 {
				t.Fatal(paths, err)
			}
			return
		}
		files, err := flattenTree(tree)
		if err != nil {
			t.Fatal(err)
		}
		paths := []string{}
		for name := range files {
			paths = append(paths, name)
		}
		sort.Strings(paths)
		if !slices.Equal(paths, []string{"a.txt", "sub/b.txt", "sub/deep/c.txt"}) {
			t.Fatal(paths)
		}
		var want string
		if err := json.Unmarshal(expected, &want); err != nil || files["a.txt"].Hash.String() != want {
			t.Fatal(files["a.txt"], want, err)
		}
	default:
		var want map[string][]string
		if err := json.Unmarshal(expected, &want); err != nil {
			t.Fatal(err)
		}
		ids, err := opened.Walk(WalkOptions{Include: opened.head})
		if err != nil {
			t.Fatal(err)
		}
		actual := map[string][]string{}
		for _, id := range ids {
			commit, err := opened.Commit(id)
			if err != nil {
				t.Fatal(err)
			}
			actual[strings.TrimRight(commit.Message, "\r\n")] = commit.DiffPaths
		}
		if !reflect.DeepEqual(actual, want) {
			t.Fatalf("libgit2 diff paths: want %v, got %v", want, actual)
		}
		if method == "ChangedPathsAlignEntriesByRawByteOrderNotUtf16Order" && !slices.Equal(actual["delete the private-use file"], []string{"\uE000.txt"}) {
			t.Fatal(actual)
		}
	}
}

func checkStatusContract(t *testing.T, method string, expected json.RawMessage, original int) {
	t.Helper()
	f := contractStatusFixture(t, method)
	layout, err := openLayout(f.path)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := layout.open()
	if err != nil {
		t.Fatal(err)
	}
	status, err := workingTreeStatus(layout, opened)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range status {
		if entry.Staging != git.Unmodified || entry.Worktree != git.Unmodified {
			count++
		}
	}
	var oracle int
	if err := json.Unmarshal(expected, &oracle); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" && method == "CountsExecutableBitChanges" {
		// The pinned managed backend ignores executable mode on Windows.
		original, oracle = 0, 0
	}
	if count != original || count != oracle {
		t.Fatalf("want original %d and libgit2 %d, got %d: %v", original, oracle, count, status)
	}
}

package gitrepo

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/go-git/go-git/v5/plumbing"
)

type revisionCase struct {
	ID, Argument string
}

type revisionCorpus struct {
	Revision, Reference string
	ReferenceSHA256     string `json:"reference_sha256"`
	Methods             map[string]struct{ Cases []revisionCase }
}

func readRevisionCorpus(t *testing.T) revisionCorpus {
	t.Helper()
	var corpus revisionCorpus
	if err := parity.ReadJSON("../../testdata/csharp-revision-walker.json", &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Methods) != 16 {
		t.Fatal("revision-walker source inventory changed")
	}
	return corpus
}

func readRevisionReference(t *testing.T, corpus revisionCorpus) contractReference {
	t.Helper()
	path := filepath.Join("../..", corpus.Reference)
	hash, err := parity.FileHash(path)
	if err != nil || hash != corpus.ReferenceSHA256 {
		t.Fatal("revision-walker reference hash differs", err)
	}
	var reference contractReference
	if err := parity.ReadJSON(path, &reference); err != nil {
		t.Fatal(err)
	}
	if reference.Revision != corpus.Revision || len(reference.Cases) != 47 {
		t.Fatal("revision-walker reference inventory changed")
	}
	return reference
}

type revisionQuery struct {
	options  WalkOptions
	sort     string
	includes []string
	base     bool
}

func (q revisionQuery) arguments() []string {
	if q.base {
		return append([]string{"merge-base"}, q.includes...)
	}
	return append([]string{"walk", q.sort, strconv.FormatBool(q.options.FirstParent), q.options.Exclude}, q.includes...)
}

func revisionHistory(method string) string {
	return map[string]string{
		"LinearHistoryMatchesLibGit2":             "linear",
		"MergedHistoryMatchesLibGit2":             "merged",
		"MergedHistoryWithExcludesMatchesLibGit2": "merged",
		"EqualCommitterTimestampsMatchLibGit2":    "equal",
		"CrissCrossHistoryMatchesLibGit2":         "criss-cross",
		"OctopusMergeMatchesLibGit2":              "octopus",
		"FirstParentWalksMatchLibGit2":            "merged",
		"MultipleIncludesMatchLibGit2":            "merged",
		"FindsTheMergeBaseOfDivergedBranches":     "merged",
		"FindsTheMergeBaseOfCrissCrossBranches":   "criss-cross",
	}[method]
}

func revisionQueries(t *testing.T, method, arg string) (*revisionFixture, []revisionQuery) {
	t.Helper()
	history := revisionHistory(method)
	if history == "" {
		return nil, nil
	}
	f := newRevisionFixture(t, history)
	if method == "FindsTheMergeBaseOfDivergedBranches" {
		return f, []revisionQuery{{base: true, includes: []string{f.main.String(), f.feature.String()}}}
	}
	if method == "FindsTheMergeBaseOfCrissCrossBranches" {
		return f, []revisionQuery{
			{base: true, includes: []string{f.main.String(), f.dev.String()}},
			{base: true, includes: []string{f.dev.String(), f.main.String()}},
		}
	}
	q := revisionQuery{sort: arg, includes: []string{f.head.String()}}
	switch method {
	case "FirstParentWalksMatchLibGit2":
		q.sort = "None"
		q.options.FirstParent = true
		if arg == "true" {
			q.options.Exclude = f.root.String()
		}
	case "MergedHistoryWithExcludesMatchesLibGit2":
		q.options.Exclude = f.root.String()
	case "MultipleIncludesMatchLibGit2":
		q.sort = "Topological, Time"
		q.includes = []string{f.olderMain.String(), f.feature.String()}
	}
	for _, flag := range strings.Split(q.sort, ", ") {
		switch flag {
		case "None":
		case "Topological":
			q.options.Sort |= Topological
		case "Time":
			q.options.Sort |= Time
		case "Reverse":
			q.options.Sort |= Reverse
		default:
			t.Fatal("unrecognized sort flag", flag)
		}
	}
	return f, []revisionQuery{q}
}

func TestCSharpRevisionWalker(t *testing.T) {
	corpus := readRevisionCorpus(t)
	reference := readRevisionReference(t, corpus)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for method, spec := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, tc := range spec.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					key := method + "/" + tc.ID
					fixture, queries := revisionQueries(t, method, tc.Argument)
					if fixture == nil {
						checkRevisionStorage(t, method)
						return
					}
					var expected [][]string
					if err := json.Unmarshal(reference.Cases[key], &expected); err != nil || len(expected) != len(queries) {
						t.Fatalf("missing reference query results for %s: %v", key, err)
					}
					repo := fixture.openCommitRepository()
					for i, q := range queries {
						if q.base {
							actual, err := repo.MergeBase(q.includes[0], q.includes[1])
							if err != nil || actual == nil || len(expected[i]) != 1 || *actual != expected[i][0] {
								t.Fatalf("merge base query %d: expected %v, got %v, error %v", i, expected[i], actual, err)
							}
						} else {
							var actual []string
							var err error
							if len(q.includes) == 1 {
								q.options.Include = q.includes[0]
								actual, err = repo.Walk(q.options)
							} else {
								actual, err = repo.walkFrom(q.options, q.includes)
							}
							if err != nil || len(actual) == 0 || !slices.Equal(actual, expected[i]) {
								t.Fatalf("sort %s, first parent %t, exclude %q: expected %v, got %v, error %v", q.sort, q.options.FirstParent, q.options.Exclude, expected[i], actual, err)
							}
						}
					}
				})
			}
		})
	}
}

func checkRevisionStorage(t *testing.T, method string) {
	t.Helper()
	if method == "UnrelatedHistoriesHaveNoMergeBase" {
		f := newRevisionFixture(t, "unrelated")
		base, err := f.openCommitRepository().MergeBase(f.head.String(), f.dev.String())
		if err != nil || base != nil {
			t.Fatalf("unrelated histories have merge base %v, error %v", base, err)
		}
		return
	}
	f := newRevisionFixture(t, "linear")
	options := WalkOptions{Include: f.head.String()}
	switch method {
	case "TheMergeBaseOfACommitAndItsAncestorIsTheAncestor":
		repo := f.openCommitRepository()
		ancestor, tip := f.linear[2].String(), f.head.String()
		for _, test := range []struct{ first, second, want string }{
			{tip, ancestor, ancestor}, {ancestor, tip, ancestor}, {tip, tip, tip},
		} {
			base, err := repo.MergeBase(test.first, test.second)
			if err != nil || base == nil || *base != test.want {
				t.Fatalf("merge base of %s and %s: expected %s, got %v, error %v", test.first, test.second, test.want, base, err)
			}
		}
	case "WalksUpToTheShallowBoundary", "TheShallowBoundaryWinsOverPresentParentObjects":
		depth := 3
		if method == "WalksUpToTheShallowBoundary" {
			depth = 2
			for _, commit := range f.linear[:len(f.linear)-depth] {
				f.removeCommit(commit)
			}
		}
		boundary := f.linear[len(f.linear)-depth].String()
		f.write(".git/shallow", boundary+"\n")
		repo := f.openCommitRepository()
		ids, err := repo.Walk(options)
		if err != nil || len(ids) != depth || ids[depth-1] != boundary {
			t.Fatalf("expected %d commits ending at %s, got %v, error %v", depth, boundary, ids, err)
		}
		metadata, err := repo.CommitMetadata(ids[depth-1])
		if err != nil || len(metadata.Parents) != 0 {
			t.Fatalf("boundary parents %v, error %v", metadata.Parents, err)
		}
	case "AMissingParentObjectFailsTheWalkInsteadOfTruncatingHistory", "WalkingFromAMissingCommitFails":
		missing := "0123456789012345678901234567890123456789"
		if method == "AMissingParentObjectFailsTheWalkInsteadOfTruncatingHistory" {
			missing = f.linear[3].String()
			f.removeCommit(f.linear[3])
		} else {
			options.Include = missing
		}
		ids, err := f.openCommitRepository().Walk(options)
		if !errors.Is(err, plumbing.ErrObjectNotFound) || !strings.Contains(fmt.Sprint(err), missing) || len(ids) != 0 {
			t.Fatalf("expected missing object %s, got %v, error %v", missing, ids, err)
		}
	default:
		t.Fatal("missing original assertion for", method)
	}
}

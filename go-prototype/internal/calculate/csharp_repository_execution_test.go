package calculate

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestCSharpRepositoryExecution(t *testing.T) {
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct {
				Cases []struct {
					ID          string
					Seeds       []uint32
					ReleaseBase string `json:"expected_release_base"`
					DevelopBase string `json:"expected_develop_base"`
				}
			}
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-repository-execution.json", &corpus); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"RepositoryStoreTests", "RepositoryStoreEqualTimestampTests"} {
		for method, item := range corpus.Suites[name].Methods {
			t.Run(method, func(t *testing.T) {
				for _, tc := range item.Cases {
					t.Run(tc.ID, func(t *testing.T) {
						if len(tc.Seeds) == 0 {
							checkForwardMergeBases(t, method, tc.ReleaseBase, tc.DevelopBase)
							return
						}
						for _, seed := range tc.Seeds {
							t.Run(fmt.Sprintf("seed-%02d", seed), func(t *testing.T) {
								checkEqualTimestampLog(t, seed, method)
							})
						}
					})
				}
			})
		}
	}
}

func checkForwardMergeBases(t *testing.T, method, releaseBase, developBase string) {
	t.Helper()
	const release = "release-2.0.0"
	steps := []fixtures.Step{
		{Op: "commit", ID: "initial", Message: "initial"},
		{Op: "checkpoint", ID: "initial"},
		{Op: "branch", Name: "develop"}, {Op: "checkout", Name: "develop"},
		{Op: "branch", Name: release}, {Op: "checkout", Name: release},
		{Op: "commit", ID: "release-1", Message: "release 1"},
		{Op: "commit", ID: "release-2", Message: "release 2"},
		{Op: "checkpoint", ID: "release-2"},
		{Op: "checkout", Name: "develop"}, {Op: "merge", ID: "forward-1", From: release},
		{Op: "checkout", Name: release}, {Op: "commit", ID: "release-3", Message: "release 3 - after first merge"},
		{Op: "checkout", Name: "develop"},
	}
	if method != "FindsCorrectMergeBaseForForwardMerge" {
		steps = append(steps, fixtures.Step{Op: "commit", ID: "develop-after", Message: "develop after merge"})
	}
	if method == "FindsCorrectMergeBaseForMultipleForwardMerges" {
		steps = append(steps, fixtures.Step{Op: "checkout", Name: release},
			fixtures.Step{Op: "commit", ID: "release-4", Message: "release 4"},
			fixtures.Step{Op: "commit", ID: "release-5", Message: "release 5"},
			fixtures.Step{Op: "checkpoint", ID: "release-5"},
			fixtures.Step{Op: "checkout", Name: "develop"}, fixtures.Step{Op: "merge", ID: "forward-2", From: release})
	}
	steps = append(steps, fixtures.Step{Op: "checkout", Name: release}, fixtures.Step{Op: "checkpoint", ID: "head"})
	cps, err := fixtures.Generate([]fixtures.Recipe{{ID: "forward-merge", Steps: steps}}, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{}
	for _, cp := range cps {
		expected[cp.ID] = cp.Head
	}
	repo, err := gitrepo.Open(cps[len(cps)-1].Path)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := repo.References()
	if err != nil {
		t.Fatal(err)
	}
	tips := map[string]string{}
	for _, ref := range refs {
		tips[ref.Name] = ref.SHA
	}
	for _, direction := range []struct{ from, to, base string }{
		{release, "develop", releaseBase}, {"develop", release, developBase},
	} {
		actual, err := branchMergeBase(repo, tips["refs/heads/"+direction.from], tips["refs/heads/"+direction.to])
		want := expected["forward-merge--"+direction.base]
		if err != nil || want == "" || actual != want {
			t.Fatalf("merge base %s -> %s = %s, want %s, error %v", direction.from, direction.to, actual, want, err)
		}
	}
}

func checkEqualTimestampLog(t *testing.T, seed uint32, method string) {
	t.Helper()
	repo := equalTimestampRepo(t, seed)
	_, head := repo.Head()
	ignored, err := newCommitFilter(repo, config.IgnoreSettings{})
	if err != nil {
		t.Fatal(err)
	}
	log := commitLog{walk: repo.Walk, load: repo.CommitMetadata, ignored: ignored}
	history, err := repo.Walk(gitrepo.WalkOptions{Include: head, Sort: commitLogSort})
	if err != nil || len(history) != 15 {
		t.Fatalf("history length = %d, error %v", len(history), err)
	}
	merges := 0
	for _, base := range history {
		commit, err := repo.CommitMetadata(base)
		if err != nil {
			t.Fatal(err)
		}
		if len(commit.Parents) > 1 {
			merges++
		}
		got, err := log.derive(head, base, ignored, nil)
		if err != nil {
			t.Fatal(err)
		}
		switch method {
		case "DerivedCommitLogReturnsTheSameCommitsWhenTimestampsAreEqual":
			want, err := repo.Walk(gitrepo.WalkOptions{Include: head, Exclude: base, Sort: commitLogSort})
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))) {
				t.Fatalf("base %s: derived %v, walk %v", base, got, want)
			}
		case "DerivedCommitLogIsAValidTopologicalOrderWhenTimestampsAreEqual":
			positions := map[string]int{}
			for i, sha := range got {
				positions[sha] = i
			}
			for i, sha := range got {
				commit, err := repo.CommitMetadata(sha)
				if err != nil {
					t.Fatal(err)
				}
				for _, parent := range commit.Parents {
					if index, ok := positions[parent]; ok && index <= i {
						t.Fatalf("base %s: parent %s at %d precedes child %s at %d", base, parent, index, sha, i)
					}
				}
			}
		default:
			t.Fatal("missing original assertion", method)
		}
	}
	if merges == 0 {
		t.Fatal("generated history has no merge")
	}
}

func equalTimestampRepo(t *testing.T, seed uint32) *gitrepo.Repository {
	t.Helper()
	path := t.TempDir()
	raw, err := git.PlainInit(path, false)
	if err != nil {
		t.Fatal(err)
	}
	store := raw.Storer
	tree := &object.Tree{}
	encoded := store.NewEncodedObject()
	if err := tree.Encode(encoded); err != nil {
		t.Fatal(err)
	}
	treeID, err := store.SetEncodedObject(encoded)
	if err != nil {
		t.Fatal(err)
	}
	signature := object.Signature{Name: "test", Email: "test@test.io", When: time.Unix(1700000000, 0).UTC()}
	write := func(message string, parents []plumbing.Hash) plumbing.Hash {
		t.Helper()
		commit := &object.Commit{Author: signature, Committer: signature, Message: message, TreeHash: treeID, ParentHashes: parents}
		encoded := store.NewEncodedObject()
		if err := commit.Encode(encoded); err != nil {
			t.Fatal(err)
		}
		id, err := store.SetEncodedObject(encoded)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	state := seed + 0x9E3779B9
	next := func(limit uint32) uint32 {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		return state % limit
	}
	var created []plumbing.Hash
	for i := 0; i < 14; i++ {
		count := min(len(created), 1+int(next(3)))
		type ranked struct {
			id  plumbing.Hash
			key uint32
		}
		rankedParents := make([]ranked, len(created))
		for i, id := range created {
			rankedParents[i] = ranked{id, next(1<<31 - 1)}
		}
		slices.SortStableFunc(rankedParents, func(a, b ranked) int { return cmp.Compare(a.key, b.key) })
		parents := make([]plumbing.Hash, count)
		for j := range parents {
			parents[j] = rankedParents[j].id
		}
		created = append(created, write(fmt.Sprintf("commit %d", i), parents))
	}
	ref := plumbing.NewBranchReferenceName("main")
	for _, value := range []*plumbing.Reference{plumbing.NewHashReference(ref, write("tip", created)), plumbing.NewSymbolicReference(plumbing.HEAD, ref)} {
		if err := store.SetReference(value); err != nil {
			t.Fatal(err)
		}
	}
	repo, err := gitrepo.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

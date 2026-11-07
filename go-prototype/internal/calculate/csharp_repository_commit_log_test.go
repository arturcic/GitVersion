package calculate

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func commitLogRecipe(unmerged bool) fixtures.Recipe {
	steps := []fixtures.Step{
		{Op: "commit", ID: "initial", Message: "initial"},
		{Op: "commit", ID: "main-one", Message: "main one"},
		{Op: "branch", Name: "develop"},
		{Op: "checkout", Name: "develop"},
		{Op: "commit", ID: "develop-one", Message: "develop one"},
		{Op: "commit", ID: "develop-two", Message: "develop two"},
		{Op: "checkout", Name: "main"},
		{Op: "commit", ID: "main-two", Message: "main two"},
		{Op: "branch", Name: "feature/a"},
		{Op: "checkout", Name: "feature/a"},
		{Op: "commit", ID: "feature-a-one", Message: "feature a one"},
		{Op: "checkout", Name: "develop"},
		{Op: "merge", ID: "feature-merge", From: "feature/a"},
		{Op: "commit", ID: "develop-three", Message: "develop three"},
		{Op: "checkout", Name: "main"},
		{Op: "merge", ID: "develop-merge", From: "develop"},
		{Op: "commit", ID: "main-three", Message: "main three"},
	}
	if unmerged {
		steps = append(steps,
			fixtures.Step{Op: "branch", Name: "feature/unmerged"},
			fixtures.Step{Op: "checkout", Name: "feature/unmerged"},
			fixtures.Step{Op: "commit", ID: "unmerged-one", Message: "unmerged one"},
			fixtures.Step{Op: "commit", ID: "unmerged-two", Message: "unmerged two"},
			fixtures.Step{Op: "checkout", Name: "main"},
			fixtures.Step{Op: "commit", ID: "main-after", Message: "main after branching off"},
		)
	}
	return fixtures.Recipe{
		ID: "commit-log", Config: "workflow: GitFlow/v1\n",
		Steps: append(steps, fixtures.Step{Op: "checkpoint", ID: "head"}),
	}
}

func TestCSharpRepositoryStoreCommitLog(t *testing.T) {
	var corpus struct {
		Revision string
		Methods  map[string][]struct{ ID string }
	}
	if err := parity.ReadJSON("../../testdata/csharp-repository-commit-log.json", &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Methods) != 9 {
		t.Fatal("original commit-log inventory changed")
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.ID, func(t *testing.T) { checkCommitLogContract(t, method) })
			}
		})
	}
}

func checkCommitLogContract(t *testing.T, method string) {
	t.Helper()
	unmerged := method == "DerivedCommitLogMatchesRevisionWalkWhenBaseVersionSourceIsUnreachableFromHead"
	checkpoints, err := fixtures.Generate([]fixtures.Recipe{commitLogRecipe(unmerged)}, filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	repo, err := gitrepo.Open(checkpoints[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	_, head := repo.Head()
	walks := 0
	log := commitLog{
		walk: func(options gitrepo.WalkOptions) ([]string, error) {
			walks++
			return repo.Walk(options)
		},
		load: repo.CommitMetadata,
	}
	filter := func(shas ...string) *commitFilter {
		t.Helper()
		out, err := newCommitFilter(repo, config.IgnoreSettings{SHAs: shas})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	unfiltered := filter()
	log.ignored = unfiltered
	walk := func(head, base string, sort gitrepo.Sort) []string {
		t.Helper()
		ids, err := repo.Walk(gitrepo.WalkOptions{Include: head, Exclude: base, Sort: sort})
		if err != nil {
			t.Fatal(err)
		}
		return ids
	}
	derive := func(head, base string, ignored *commitFilter, excluded ...string) []string {
		t.Helper()
		ids, err := log.derive(head, base, ignored, excluded)
		if err != nil {
			t.Fatal(err)
		}
		return ids
	}
	assertSame := func(got, expected []string, base, ignored string) {
		t.Helper()
		if !slices.Equal(got, expected) {
			t.Fatalf("base %s ignoring %s: got %v, want %v", base, ignored, got, expected)
		}
	}
	requireWalks := func(want int) {
		t.Helper()
		if walks != want {
			t.Fatalf("revision walks %d, want %d", walks, want)
		}
	}

	switch method {
	case "DerivedCommitLogMatchesRevisionWalkForEveryBaseVersionSource":
		for _, base := range walk(head, "", 0) {
			assertSame(derive(head, base, unfiltered), walk(head, base, commitLogSort), base, "")
		}

	case "DerivedCommitLogMatchesRevisionWalkWhenBaseVersionSourceIsUnreachableFromHead":
		refs, err := repo.References()
		if err != nil {
			t.Fatal(err)
		}
		var base string
		for _, ref := range refs {
			if ref.Name == "refs/heads/feature/unmerged" {
				base = ref.SHA
				break
			}
		}
		if base == "" || slices.Contains(walk(head, "", 0), base) {
			t.Fatal("expected an unreachable base commit")
		}
		assertSame(derive(head, base, unfiltered), walk(head, base, commitLogSort), base, "")

	case "DerivedCommitLogExcludesTheAncestorsOfTheGivenShas":
		for _, excluded := range walk(head, "", commitLogSort) {
			assertSame(derive(head, "", unfiltered, excluded), walk(head, excluded, commitLogSort), excluded, "")
		}

	case "DerivedCommitLogStopsExcludingAtIgnoredCommits":
		history := walk(head, "", commitLogSort)
		root, shield := history[len(history)-1], history[len(history)-2]
		if got := derive(head, "", filter(shield), history[0]); !slices.Contains(got, root) {
			t.Fatalf("ignored commit %s should shield root %s, got %v", shield, root, got)
		}

	case "DerivedCommitLogHonoursIgnoredCommits":
		history := walk(head, "", commitLogSort)
		ignored := history[len(history)-1]
		if got := derive(head, "", filter(ignored)); slices.Contains(got, ignored) {
			t.Fatalf("ignored commit %s still present", ignored)
		}

	case "DerivedCommitLogMatchesRevisionWalkWhenACommitInTheMiddleOfTheHistoryIsIgnored":
		history := walk(head, "", commitLogSort)
		for _, ignored := range history {
			settings := filter(ignored)
			for _, base := range history {
				expected := slices.DeleteFunc(walk(head, base, commitLogSort), func(id string) bool { return id == ignored })
				assertSame(derive(head, base, settings), expected, base, ignored)
			}
		}

	case "DerivedCommitLogPerformsExactlyOneRevisionWalk":
		derive(head, "", unfiltered)
		for _, base := range walk(head, "", 0) {
			derive(head, base, unfiltered, base)
		}
		requireWalks(1)

	case "IncrementStrategyFinderReadsTheWholeHistoryWithASingleRevisionWalk":
		cfg, err := config.Load(checkpoints[0].Path)
		if err != nil {
			t.Fatal(err)
		}
		e, err := cfg.Effective("main")
		if err != nil {
			t.Fatal(err)
		}
		for _, base := range walk(head, "", 0) {
			got, err := log.increment(head, base, "", e.Increment, e.Messages, nil)
			if err != nil || got != e.Increment {
				t.Fatalf("expected default increment %v, got %v, error %v", e.Increment, got, err)
			}
		}
		requireWalks(1)

	case "DerivedCommitLogRetainsOnlyTheProjectionOfTheCurrentHead":
		tip, err := repo.CommitMetadata(head)
		if err != nil {
			t.Fatal(err)
		}
		derive(head, "", unfiltered)
		derive(tip.Parents[0], "", unfiltered)
		requireWalks(2)
		actual := derive(head, "", unfiltered)
		requireWalks(3)
		assertSame(actual, walk(head, "", commitLogSort), "", "")

	default:
		t.Fatal("missing original assertion", method)
	}
}

func TestIncrementHistoryUsesTheBoundedWalkForReset(t *testing.T) {
	recipe := commitLogRecipe(false)
	for i, step := range recipe.Steps {
		if step.ID == "develop-three" {
			recipe.Steps[i].Message = "=semver: none"
			break
		}
	}
	checkpoints, err := fixtures.Generate([]fixtures.Recipe{recipe}, filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	repo, err := gitrepo.Open(checkpoints[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	_, head := repo.Head()
	history, err := repo.Walk(gitrepo.WalkOptions{Include: head, Sort: commitLogSort})
	if err != nil || len(history) < 2 {
		t.Fatalf("fixture history %v, error %v", history, err)
	}
	base := history[len(history)-2]
	var calls []gitrepo.WalkOptions
	log := commitLog{
		walk: func(options gitrepo.WalkOptions) ([]string, error) {
			calls = append(calls, options)
			return repo.Walk(options)
		},
		load: repo.CommitMetadata,
	}
	filter, err := newCommitFilter(repo, config.IgnoreSettings{})
	if err != nil {
		t.Fatal(err)
	}
	log.ignored = filter
	got, err := log.incrementHistory(head, base, nil)
	wantOptions := gitrepo.WalkOptions{Include: head, Exclude: base, Sort: commitLogSort}
	want, walkErr := repo.Walk(wantOptions)
	if err != nil || walkErr != nil || !slices.Equal(got, want) {
		t.Fatalf("bounded history got %v, want %v, errors %v and %v", got, want, err, walkErr)
	}
	if len(calls) != 2 || calls[1] != wantOptions {
		t.Fatalf("reset requires bounded walk after the projection, got %+v", calls)
	}
	if !slices.ContainsFunc(got, func(id string) bool {
		commit, err := repo.CommitMetadata(id)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Contains(commit.Message, "=semver: none")
	}) {
		t.Fatal("reset commit is missing from bounded fixture")
	}
}

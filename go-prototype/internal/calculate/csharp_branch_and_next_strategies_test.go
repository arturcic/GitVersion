package calculate

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

func TestCSharpBranchAndNextStrategies(t *testing.T) {
	var corpus struct {
		Revision string
		Suites   map[string]struct {
			Methods map[string]struct {
				Cases []struct {
					ID        string
					Arguments []*string
				}
			}
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-branch-and-next-strategies.json", &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Suites) != 2 {
		t.Fatal("original strategy inventory changed")
	}
	for suite, record := range corpus.Suites {
		t.Run(suite, func(t *testing.T) {
			for method, spec := range record.Methods {
				t.Run(method, func(t *testing.T) {
					for _, tc := range spec.Cases {
						t.Run(tc.ID, func(t *testing.T) {
							switch suite {
							case "VersionInBranchNameBaseVersionStrategyTests":
								checkBranchVersionStrategy(t, method, tc.Arguments)
							case "ConfiguredNextVersionVersionStrategyTests":
								checkConfiguredNextVersionStrategy(t, method, tc.Arguments)
							default:
								t.Fatal("unknown strategy suite", suite)
							}
						})
					}
				})
			}
		})
	}
}

func checkBranchVersionStrategy(t *testing.T, method string, args []*string) {
	t.Helper()
	branch := *args[0]
	ref := "refs/heads/" + branch
	var expected *string
	if len(args) > 1 {
		expected = args[1]
	}
	step := fixtures.Step{Op: "branch", Name: branch}
	if method == "CanTakeVersionFromNameOfRemoteReleaseBranch" {
		branch = *args[1]
		step = fixtures.Step{Op: "remote-branch", Name: *args[0] + "/" + branch}
		ref = "refs/remotes/" + step.Name
		expected = args[2]
	}
	document := "workflow: GitFlow/v1\n"
	if method == "CanTakeVersionFromNameOfConfiguredReleaseBranch" {
		document += "calculation:\n  branches:\n    support:\n      is-release-branch: true\n"
	}
	recipe := fixtures.Recipe{ID: "branch-source", Config: document, Steps: []fixtures.Step{
		{Op: "commit", ID: "root", Message: "root"}, step, {Op: "checkpoint", ID: "head"},
	}}
	checkpoints, err := fixtures.Generate([]fixtures.Recipe{recipe}, filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	path := checkpoints[0].Path
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := gitrepo.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := repo.References()
	if err != nil {
		t.Fatal(err)
	}
	_, head := repo.Head()
	found := false
	for _, current := range refs {
		if current.Name == ref {
			found = current.SHA == head
			break
		}
	}
	if !found {
		t.Fatal("branch at original fixture tip missing", ref)
	}
	effective, err := cfg.Effective(ref)
	if err != nil {
		t.Fatal(err)
	}
	got, err := workflowCandidates(repo, cfg, effective, "main", ref, head, nil, nil, nil, repo.CommitMetadata, nil)
	if err != nil {
		t.Fatal(err)
	}
	if expected == nil {
		if len(got) != 0 {
			t.Fatalf("expected no branch version, got %s", got[0].semantic)
		}
	} else if len(got) != 1 || got[0].semantic.String() != *expected {
		t.Fatalf("expected one branch version %s, got %+v", *expected, got)
	}
}

func checkConfiguredNextVersionStrategy(t *testing.T, method string, args []*string) {
	t.Helper()
	var document string
	switch method {
	case "ReturnsNullWhenNoNextVersionIsInConfig":
	case "ConfiguredNextVersionTest":
		document = nextVersionDocument(*args[0], *args[2])
	case "ConfiguredNextVersionTestShouldFail":
		_, err := config.ParseProvided([]byte(nextVersionDocument(*args[0], *args[1])), "v7")
		var diagnostic *config.ValidationError
		if !errors.As(err, &diagnostic) || diagnostic.Code != "GVCONFIG_VALUE" ||
			!strings.HasSuffix(diagnostic.PropertyPath, "next-version") || strings.Contains(diagnostic.Error(), *args[0]) {
			t.Fatalf("expected a safe next-version validation diagnostic, got %v", err)
		}
		return
	default:
		t.Fatal("unknown original method", method)
	}
	cfg, err := config.ParseProvided([]byte(document), "v7")
	if err != nil {
		t.Fatal(err)
	}
	effective, err := cfg.Effective("main")
	if err != nil {
		t.Fatal(err)
	}
	got, found, err := configuredNextVersionCandidate(cfg, effective.Label)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) == 0 || args[1] == nil {
		if found {
			t.Fatalf("expected no configured candidate, got %s", got.semantic)
		}
		return
	}
	semantic, incremented := *args[1], *args[3]
	if !found || got.semantic.String() != semantic || got.semanticSHA != "" || got.sha != "" {
		t.Fatalf("expected a source-free semantic version %s, got %+v", semantic, got)
	}
	if got.version.String() != incremented || got.inc != semver.None {
		t.Fatalf("expected no core increment and version %s, got %s (%v)", incremented, got.version, got.inc)
	}
	if semantic != incremented {
		if got.version.Label != "" || got.version.Number == nil || *got.version.Number != 1 ||
			got.version.Core() != got.semantic.Core() {
			t.Fatalf("expected a number-only prerelease operator, got %s", got.version)
		}
	} else if !reflect.DeepEqual(got.version, got.semantic) {
		t.Fatalf("expected unchanged version %s, got %s", got.semantic, got.version)
	}
}

func nextVersionDocument(version, format string) string {
	return fmt.Sprintf("calculation:\n  semantic-version-format: %s\n  next-version: %q\n", format, version)
}

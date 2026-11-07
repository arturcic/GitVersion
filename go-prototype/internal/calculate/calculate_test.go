package calculate

import (
	"encoding/json"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoCalculationMatchesFrozenCorpus(t *testing.T) {
	recipes, err := fixtures.Load("../../testdata/recipes.json")
	if err != nil {
		t.Fatal(err)
	}
	cps, err := fixtures.Generate(recipes, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Outputs map[string]json.RawMessage `json:"outputs"`
	}
	if err = json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if len(cps) != 41 || len(golden.Outputs) != 41 {
		t.Fatal("frozen corpus inventory changed")
	}
	for _, cp := range cps {
		t.Run(cp.ID, func(t *testing.T) {
			before, err := fixtures.State(cp.Path)
			if err != nil {
				t.Fatal(err)
			}
			values, trace, err := Calculate(cp.Path)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := json.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			diff, err := parity.Compare(golden.Outputs[cp.ID], actual)
			if err != nil {
				t.Fatal(err)
			}
			if len(diff) > 0 {
				t.Logf("trace: %+v", trace)
				for _, d := range diff {
					t.Errorf("%s: expected %s actual %s", d.Field, d.Expected, d.Actual)
				}
			}
			after, err := fixtures.State(cp.Path)
			if err != nil {
				t.Fatal(err)
			}
			if before != after {
				t.Fatal("calculation changed repository")
			}
			if len(values) != 33 {
				t.Fatalf("output field count %d", len(values))
			}
		})
	}
}

func TestCalculationSelectsLatestCommitByReferenceOrdering(t *testing.T) {
	late, early := int64(120), int64(60)
	r := fixtures.Recipe{ID: "skewed", Config: "workflow: GitHubFlow/v1\n", Steps: []fixtures.Step{
		{Op: "commit", ID: "root", Seconds: &late}, {Op: "tag", Name: "1.2.3"}, {Op: "checkpoint", ID: "tagged"},
		{Op: "commit", ID: "tip", Seconds: &early, Message: "+semver: major"}, {Op: "checkpoint", ID: "tip"},
	}}
	cps, err := fixtures.Generate([]fixtures.Recipe{r}, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	values, _, err := Calculate(cps[1].Path)
	if err != nil {
		t.Fatal(err)
	}
	if values["Sha"] != cps[0].Head || values["SemVer"] != "1.2.3" || values["CommitCountSourceDistance"] != int64(0) {
		t.Fatalf("must match pinned latest-time selection, not physical HEAD: %+v", values)
	}
}

func TestInheritedBranchControlsAlternativeVersion(t *testing.T) {
	for _, tc := range []struct{ name, tagBranch, want string }{
		{"feature-only-tag", "feature/topic", "0.0.1-topic.1"},
		{"inherited-main-tag", "main", "3.4.0-topic.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := fixtures.Recipe{ID: "inheritance", Config: "workflow: GitHubFlow/v1\n", Steps: []fixtures.Step{
				{Op: "commit", ID: "root"}, {Op: "branch", Name: "feature/topic"},
				{Op: "checkout", Name: tc.tagBranch}, {Op: "commit", ID: "tagged"}, {Op: "tag", Name: "3.4.0-beta.2"},
				{Op: "checkout", Name: "feature/topic"}, {Op: "commit", ID: "head"}, {Op: "checkpoint", ID: "head"},
			}}
			cps, err := fixtures.Generate([]fixtures.Recipe{r}, filepath.Join(t.TempDir(), "fixtures"))
			if err != nil {
				t.Fatal(err)
			}
			values, _, err := Calculate(cps[0].Path)
			if err != nil {
				t.Fatal(err)
			}
			if values["SemVer"] != tc.want {
				t.Fatalf("expected %s, got %v", tc.want, values["SemVer"])
			}
		})
	}
}

func TestCalculationRejectsUnsupportedPolicies(t *testing.T) {
	for _, tc := range []struct{ name, config, branch, message, tag string }{
		{"workflow", "workflow: UnknownFlow/v1\n", "", "", ""},
		{"custom-format", "workflow: GitHubFlow/v1\noutput: {custom-version-format: foo}\n", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			steps := []fixtures.Step{{Op: "commit", ID: "root"}}
			if tc.branch != "" {
				steps = append(steps, fixtures.Step{Op: "branch", Name: tc.branch}, fixtures.Step{Op: "checkout", Name: tc.branch})
			}
			steps = append(steps, fixtures.Step{Op: "commit", ID: "work", Message: tc.message})
			if tc.tag != "" {
				steps = append(steps, fixtures.Step{Op: "tag", Name: tc.tag})
			}
			steps = append(steps, fixtures.Step{Op: "checkpoint", ID: "head"})
			cps, err := fixtures.Generate([]fixtures.Recipe{{ID: "unsupported", Config: tc.config, Steps: steps}}, filepath.Join(t.TempDir(), "fixtures"))
			if err != nil {
				t.Fatal(err)
			}
			if values, _, err := Calculate(cps[0].Path); err == nil || values != nil {
				t.Fatalf("unsupported state emitted version: %+v %v", values, err)
			}
		})
	}
}

func TestCalculationConfiguredSourceKeepsTagAnchor(t *testing.T) {
	recipe := fixtures.Recipe{ID: "configured", Config: "workflow: GitHubFlow/v1\ncalculation:\n  next-version: 9.0.0\n", Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.0"}, {Op: "checkpoint", ID: "tagged"}, {Op: "commit", ID: "after"}, {Op: "checkpoint", ID: "after"}}}
	cps, err := fixtures.Generate([]fixtures.Recipe{recipe}, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	tagged, _, err := Calculate(cps[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if tagged["FullSemVer"] != "1.2.0" {
		t.Fatal("configured version overrode tagged main HEAD")
	}
	got, trace, err := Calculate(cps[1].Path)
	if err != nil {
		t.Fatal(err)
	}
	if got["FullSemVer"] != "9.0.0-1" || got["SemVerSourceSha"] != nil || got["SemVerSourceSemVer"] != "9.0.0" || got["CommitCountSourceSha"] != cps[0].Head || got["CommitCountSourceDistance"] != int64(1) {
		t.Fatalf("source split lost: %+v", got)
	}
	if trace.SemanticSHA != "" || trace.CountSHA != cps[0].Head || len(trace.Counted) != 1 || trace.Counted[0] != cps[1].Head {
		t.Fatalf("trace source split lost: %+v", trace)
	}
}

func TestCalculationErrorForMissingRepository(t *testing.T) {
	if _, _, err := Calculate(t.TempDir()); err == nil || !strings.Contains(err.Error(), "repository") {
		t.Fatalf("missing config: %v", err)
	}
}

func TestCalculationAlternativeAndUnicodeBumps(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		steps        []fixtures.Step
		want         string
	}{
		{"alternative-resets-number", "workflow: GitHubFlow/v1\ncalculation:\n  branches:\n    main:\n      increment: None\n", []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.0"}, {Op: "branch", Name: "feature/foo"}, {Op: "checkout", Name: "feature/foo"}, {Op: "commit", ID: "one"}, {Op: "tag", Name: "2.0.0-foo.8"}, {Op: "commit", ID: "two"}, {Op: "tag", Name: "3.0.0-bar.5"}, {Op: "checkpoint", ID: "head"}}, "3.0.0-foo.1+1"},
		{"unicode-whitespace", "workflow: GitHubFlow/v1\n", []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "2.3.4"}, {Op: "commit", ID: "bump", Message: "Change +semver:\u00a0minor"}, {Op: "checkpoint", ID: "head"}}, "2.4.0-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cps, err := fixtures.Generate([]fixtures.Recipe{{ID: "edge", Config: tc.config, Steps: tc.steps}}, filepath.Join(t.TempDir(), "fixtures"))
			if err != nil {
				t.Fatal(err)
			}
			got, _, err := Calculate(cps[0].Path)
			if err != nil {
				t.Fatal(err)
			}
			if got["FullSemVer"] != tc.want {
				t.Fatalf("got %v want %s", got["FullSemVer"], tc.want)
			}
		})
	}
}

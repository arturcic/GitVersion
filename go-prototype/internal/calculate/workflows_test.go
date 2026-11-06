package calculate

import (
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainlineAccumulatesEachTrunkCommit(t *testing.T) {
	for _, tc := range []struct {
		strategy, want string
		distance       int64
	}{{"", "0.0.1-3", 3}, {"calculation: {strategies: [Mainline]}\n", "0.0.3-1", 1}} {
		r := fixtures.Recipe{ID: "linear", Config: "workflow: GitFlow/v1\n" + tc.strategy, Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "commit", ID: "second"}, {Op: "commit", ID: "third"}, {Op: "checkpoint", ID: "head"}}}
		cps, err := fixtures.Generate([]fixtures.Recipe{r}, filepath.Join(t.TempDir(), "fixtures"))
		if err != nil {
			t.Fatal(err)
		}
		got, _, err := Calculate(cps[0].Path)
		if err != nil {
			t.Fatal(err)
		}
		if got["FullSemVer"] != tc.want || got["CommitCountSourceDistance"] != tc.distance {
			t.Fatalf("increment/count contract: %+v", got)
		}
	}
}
func TestGitFlowReleaseTrackingSeparatesSemanticAndCountSources(t *testing.T) {
	r := fixtures.Recipe{ID: "release-tracking", Config: "workflow: GitFlow/v1\n", Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}, {Op: "branch", Name: "develop"}, {Op: "checkout", Name: "develop"}, {Op: "commit", ID: "dev"}, {Op: "branch", Name: "release/1.3.0"}, {Op: "checkpoint", ID: "head"}}}
	cps, err := fixtures.Generate([]fixtures.Recipe{r}, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := Calculate(cps[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if got["FullSemVer"] != "1.4.0-alpha.0" || got["SemVerSourceSemVer"] != "1.3.0" || got["SemVerSourceSha"] != nil || got["CommitCountSourceSha"] != cps[0].Head || got["CommitCountSourceDistance"] != int64(0) {
		t.Fatalf("release source contract %+v", got)
	}
}

func TestMainlineRejectsUnresolvedInheritedIncrement(t *testing.T) {
	var reference struct {
		Recipe fixtures.Recipe `json:"recipe"`
		Digest string          `json:"digest"`
	}
	if err := parity.ReadJSON("../../testdata/workflow-reference-failure.json", &reference); err != nil {
		t.Fatal(err)
	}
	cps, err := fixtures.Generate([]fixtures.Recipe{reference.Recipe}, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if cps[0].Digest != reference.Digest {
		t.Fatal("rejection history changed")
	}
	_, _, err = Calculate(cps[0].Path)
	if err == nil || !strings.Contains(err.Error(), "increment remains Inherit") {
		t.Fatalf("expected unresolved inheritance rejection, got %v", err)
	}
}

package calculate

import (
	"fmt"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"path/filepath"
	"testing"
)

func TestDeploymentModesControlNumberAndMetadata(t *testing.T) {
	for _, tc := range []struct {
		branch, mode, want string
		metadata           any
	}{
		{"main", "ManualDeployment", "1.2.4-1+2", int64(2)},
		{"main", "ContinuousDelivery", "1.2.4-2", nil},
		{"main", "ContinuousDeployment", "1.2.4", nil},
		{"feature/topic", "ManualDeployment", "1.2.4-topic.1+2", int64(2)},
		{"feature/topic", "ContinuousDelivery", "1.2.4-topic.2", nil},
		{"feature/topic", "ContinuousDeployment", "1.2.4", nil},
	} {
		t.Run(tc.branch+"/"+tc.mode, func(t *testing.T) {
			kind := "main"
			steps := []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}}
			if tc.branch != "main" {
				kind = "feature"
				steps = append(steps, fixtures.Step{Op: "branch", Name: tc.branch}, fixtures.Step{Op: "checkout", Name: tc.branch})
			}
			steps = append(steps, fixtures.Step{Op: "commit", ID: "one"}, fixtures.Step{Op: "commit", ID: "two"}, fixtures.Step{Op: "checkpoint", ID: "head"})
			r := fixtures.Recipe{ID: "mode", Config: fmt.Sprintf("workflow: GitHubFlow/v1\ncalculation: {branches: {%s: {mode: %s}}}\n", kind, tc.mode), Steps: steps}
			cps, err := fixtures.Generate([]fixtures.Recipe{r}, filepath.Join(t.TempDir(), "fixtures"))
			if err != nil {
				t.Fatal(err)
			}
			got, trace, err := Calculate(cps[0].Path)
			if err != nil {
				t.Fatal(err)
			}
			if got["FullSemVer"] != tc.want || got["BuildMetaData"] != tc.metadata || got["CommitCountSourceDistance"] != int64(2) || trace.Mode != tc.mode {
				t.Fatalf("mode contract: %+v trace=%+v", got, trace)
			}
		})
	}
}

func TestTaggedFeaturePolicyAndDisabledMessages(t *testing.T) {
	for _, prevent := range []bool{false, true} {
		t.Run(fmt.Sprint(prevent), func(t *testing.T) {
			r := fixtures.Recipe{ID: "tagged", Config: fmt.Sprintf("workflow: GitHubFlow/v1\ncalculation: {branches: {feature: {mode: ContinuousDeployment, increment: Patch, commit-message-incrementing: Disabled, prevent-increment: {when-current-commit-tagged: %t}}}}\noutput: {tag-pre-release-weight: 0}\n", prevent), Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "branch", Name: "feature/topic"}, {Op: "checkout", Name: "feature/topic"}, {Op: "commit", ID: "work", Message: "+semver: major"}, {Op: "tag", Name: "1.2.3-topic.4"}, {Op: "checkpoint", ID: "head"}, {Op: "commit", ID: "next", Message: "+semver: major"}, {Op: "checkpoint", ID: "next"}}}
			cps, err := fixtures.Generate([]fixtures.Recipe{r}, filepath.Join(t.TempDir(), "fixtures"))
			if err != nil {
				t.Fatal(err)
			}
			for _, cp := range cps {
				got, _, err := Calculate(cp.Path)
				if err != nil {
					t.Fatal(err)
				}
				if got["SemVer"] != "1.2.3" || got["PreReleaseNumber"] != nil || got["WeightedPreReleaseNumber"] != int64(0) || got["BuildMetaData"] != nil {
					t.Fatalf("tag/mode/message interaction: %+v", got)
				}
			}
		})
	}
}

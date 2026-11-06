package output

import (
	"encoding/json"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"strings"
	"testing"
)

func TestOutputVariablesAndSourceNullability(t *testing.T) {
	v, _ := semver.Parse("7.8.9-foo.4")
	source, _ := semver.Parse("7.8.8")
	sha := strings.Repeat("a", 40)
	got, err := Variables(Result{Version: v, SemanticSource: source, Increment: semver.Patch, LegacySource: source, Distance: 3, Branch: "feature/foo", SHA: sha, CommitTime: 1735689600})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"AssemblySemFileVer": "7.8.9.0", "AssemblySemVer": "7.8.9.0", "BranchName": "feature/foo", "BuildMetaData": 3, "CommitCountSourceDistance": 3, "CommitCountSourceSha": nil, "CommitDate": "2025-01-01", "CustomVersion": "", "EscapedBranchName": "feature-foo", "FullBuildMetaData": "3.Branch.feature-foo.Sha." + sha, "FullSemVer": "7.8.9-foo.4+3", "InformationalVersion": "7.8.9-foo.4+3.Branch.feature-foo.Sha." + sha, "Major": 7, "MajorMinorPatch": "7.8.9", "Minor": 8, "Patch": 9, "PreReleaseLabelName": "foo", "PreReleaseLabelNameWithDash": "-foo", "PreReleaseNumber": 4, "PreReleaseLabel": "foo.4", "PreReleaseLabelWithDash": "-foo.4", "SemVer": "7.8.9-foo.4", "SemVerSourceIncrement": "Patch", "SemVerSourceSemVer": "7.8.8", "SemVerSourceSha": nil, "Sha": sha, "ShortSha": "aaaaaaa", "UncommittedChanges": 0, "VersionSourceDistance": 3, "VersionSourceIncrement": "None", "VersionSourceSemVer": "7.8.8", "VersionSourceSha": "", "WeightedPreReleaseNumber": 30004}
	actual, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := parity.Compare(expected, actual)
	if err != nil || len(diff) != 0 {
		t.Fatalf("output mismatch %+v %v", diff, err)
	}
}
func TestOutputStableAndMainDeployment(t *testing.T) {
	sha := strings.Repeat("b", 40)
	v, _ := semver.Parse("4.5.6")
	base := Result{Version: v, SemanticSource: v, SemanticSHA: sha, LegacySource: v, CountSHA: sha, Branch: "main", SHA: sha, Main: true, Tagged: true}
	got, err := Variables(base)
	if err != nil {
		t.Fatal(err)
	}
	if got["BuildMetaData"] != nil || got["PreReleaseNumber"] != nil || got["WeightedPreReleaseNumber"] != int64(60000) || got["FullSemVer"] != "4.5.6" || got["PreReleaseLabelWithDash"] != "" {
		t.Fatalf("stable %+v", got)
	}
	base.Version, _ = semver.Parse("4.5.7-3")
	base.Tagged = false
	base.Distance = 3
	got, err = Variables(base)
	if err != nil {
		t.Fatal(err)
	}
	if got["BuildMetaData"] != nil || got["WeightedPreReleaseNumber"] != int64(55003) || got["FullSemVer"] != "4.5.7-3" {
		t.Fatalf("main %+v", got)
	}
	base.Distance = -1
	if _, err := Variables(base); err == nil {
		t.Fatal("negative count accepted")
	}
}

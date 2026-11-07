package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

func TestCSharpVariableProvider(t *testing.T) {
	var corpus struct {
		Methods map[string][]struct {
			ID        string
			Arguments []json.RawMessage
		}
		Approvals map[string]struct {
			Variables json.RawMessage
		}
	}
	data, err := os.ReadFile("../../testdata/csharp-variable-provider.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Methods) != 18 || len(corpus.Approvals) != 8 {
		t.Fatal("incomplete variable provider corpus")
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, c := range cases {
				t.Run(c.ID, func(t *testing.T) {
					testVariableProviderCase(t, method, c.Arguments, corpus.Approvals[method].Variables)
				})
			}
		})
	}
}

func testVariableProviderCase(t *testing.T, method string, args []json.RawMessage, approved json.RawMessage) {
	t.Helper()
	five, nine := int64(5), int64(9)
	date := time.Date(2014, 3, 6, 23, 59, 59, 0, time.UTC)
	v := semver.Version{Major: 1, Minor: 2, Patch: 3}
	metadata := variableInput{version: v, branch: "develop", sha: "commitSha", shortSHA: "commitShortSha",
		countSHA: "versionSourceSha", date: &date, distance: five, tagWeight: 60000}
	settings := config.OutputSettings{}
	if len(approved) > 0 {
		switch method {
		case "ProvidesVariablesInContinuousDeliveryModeForPreRelease":
			metadata.version = releaseVersion(v, "unstable", 4)
			metadata.build = &five
		case "ProvidesVariablesInContinuousDeploymentModeForPreRelease":
			metadata.version = releaseVersion(v, "unstable", 8)
		case "ProvidesVariablesInContinuousDeliveryModeForStable":
			metadata.build, metadata.tagWeight = &five, 0
		case "ProvidesVariablesInContinuousDeploymentModeForStable":
			metadata.version = releaseVersion(v, "ci", 5)
		case "ProvidesVariablesInContinuousDeploymentModeForStableWhenCurrentCommitIsTagged":
			metadata.branch, metadata.build, metadata.tagWeight = "", &five, 0
		case "ProvidesVariablesInContinuousDeliveryModeForFeatureBranch":
			metadata.branch, metadata.build, metadata.tagWeight = "feature/123", &five, 0
		case "ProvidesVariablesInContinuousDeliveryModeForFeatureBranchWithCustomAssemblyInfoFormat":
			metadata.branch, metadata.build, metadata.tagWeight = "feature/123", &five, 0
			settings.InformationalFormat = "{Major}.{Minor}.{Patch}+{VersionSourceDistance}.Branch.{BranchName}.Sha.{ShortSha}"
		case "ProvidesVariablesInContinuousDeploymentModeForMainBranchWithEmptyLabel":
			metadata.version.Number = &nine
			metadata.branch, metadata.branchWeight = "main", 55000
		default:
			t.Fatalf("unhandled approval %s", method)
		}
		got, err := json.Marshal(renderCase(t, metadata, settings))
		if err != nil {
			t.Fatal(err)
		}
		diff, err := parity.Compare(approved, got)
		if err != nil || len(diff) != 0 {
			t.Fatalf("approved JSON differs: %+v, %v", diff, err)
		}
		return
	}

	empty := variableInput{version: v, tagWeight: 60000}
	switch method {
	case "ProvidesVariablesInContinuousDeploymentModeWithTagNamePattern":
		metadata.version = releaseVersion(v, "PullRequest2", 5)
		metadata.branch, metadata.countSHA, metadata.distance = "pull/2/merge", "", 0
		assertVariable(t, renderCase(t, metadata, settings), "FullSemVer", "1.2.3-PullRequest2.5")
	case "ProvidesVariablesInContinuousDeploymentModeWithTagSetToBranchName":
		metadata.version = releaseVersion(v, "feature", 5)
		metadata.branch, metadata.countSHA, metadata.distance = "feature", "", 0
		assertVariable(t, renderCase(t, metadata, settings), "FullSemVer", "1.2.3-feature.5")
	case "CustomVersionIsEmptyWhenFormatIsNotConfigured":
		assertVariable(t, renderCase(t, empty, settings), "CustomVersion", "")
	case "CustomVersionSupportsPep440FormatForIssue2065":
		empty.version = releaseVersion(semver.Version{Minor: 6, Patch: 3}, "beta", 10)
		settings.CustomFormat = "{Major}.{Minor}.{Patch}{PreReleaseLabelName:l}{PreReleaseNumber}"
		assertVariable(t, renderCase(t, empty, settings), "CustomVersion", "0.6.3beta10")
	case "ProvidesUnambiguousPreReleaseVariables":
		readVariableArgument(t, args[0], &empty.version.Label)
		readVariableArgument(t, args[1], &empty.version.Number)
		got := renderCase(t, empty, settings)
		roundTrip := roundTripVariables(t, got)
		for i, key := range []string{"PreReleaseLabelName", "PreReleaseNumber", "PreReleaseLabel", "PreReleaseLabelNameWithDash", "PreReleaseLabelWithDash"} {
			var want string
			readVariableArgument(t, args[i+2], &want)
			assertVariable(t, got, key, want)
			assertVariable(t, roundTrip, key, want)
		}
	case "CustomVersionFormatCanBeConfiguredPerBranch":
		cfg, err := config.Parse([]byte(`workflow: GitFlow/v1
output:
  custom-version-format: "{SemVer}-global"
  branches:
    feature: {custom-version-format: "{SemVer}-feature"}
    release: {custom-version-format: ""}
`))
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct{ branch, want string }{{"feature/example", "1.2.3-feature"}, {"main", "1.2.3-global"}, {"release/1.2.3", ""}} {
			kind, _, err := cfg.Branch(tc.branch)
			if err != nil {
				t.Fatal(err)
			}
			assertVariable(t, renderCase(t, empty, cfg.OutputFor(kind)), "CustomVersion", tc.want)
		}
	case "CustomVersionSupportsMonotonicallyIncreasingIntegerFormatForIssue3340":
		settings.CustomFormat = "{Major:00}{Minor:00}{Patch:000}"
		empty.version = semver.Version{Patch: 123}
		patch := renderCase(t, empty, settings)
		assertVariable(t, patch, "CustomVersion", "0000123")
		empty.version = semver.Version{Minor: 1}
		minor := renderCase(t, empty, settings)
		assertVariable(t, minor, "CustomVersion", "0001000")
		patchCode, patchErr := strconv.ParseInt(patch["CustomVersion"].(string), 10, 64)
		minorCode, minorErr := strconv.ParseInt(minor["CustomVersion"].(string), 10, 64)
		if patchErr != nil || minorErr != nil || minorCode <= patchCode {
			t.Fatalf("minor code %d must exceed patch code %d", minorCode, patchCode)
		}
	case "Format_Allows_CSharp_FormatStrings":
		metadata.version.Number = &nine
		metadata.branch, metadata.distance = "main", 42
		settings.InformationalFormat = "{Major}.{Minor}.{Patch}-{VersionSourceDistance:0000}"
		assertVariable(t, renderCase(t, metadata, settings), "InformationalVersion", "1.2.3-0042")
	case "SeparateSourcesSurviveFormattingEnumerationAndJson":
		var source *string
		readVariableArgument(t, args[0], &source)
		empty.version = semver.Version{Major: 5}
		empty.semanticSource = &empty.version
		empty.legacySource = &semver.Version{Major: 1}
		if source != nil {
			empty.semanticSHA = *source
		}
		empty.countSHA, empty.sha, empty.distance, empty.increment = "count-source-sha", "head-sha", 2, semver.Minor
		settings.InformationalFormat = "{SemVerSourceSemVer}-{SemVerSourceIncrement}+{CommitCountSourceDistance:000}"
		got := renderCase(t, empty, settings)
		assertVariable(t, got, "InformationalVersion", "5.0.0-Minor+002")
		assertNullableVariable(t, got, "SemVerSourceSha", source)
		assertVariable(t, got, "CommitCountSourceSha", "count-source-sha")
		roundTrip := roundTripVariables(t, got)
		if roundTrip["CommitCountSourceDistance"] != json.Number("2") {
			t.Fatalf("JSON counting distance has wrong type or value: %T %v", roundTrip["CommitCountSourceDistance"], roundTrip["CommitCountSourceDistance"])
		}
		assertNullableVariable(t, roundTrip, "SemVerSourceSha", source)
		for key, want := range map[string]string{
			"SemVerSourceSemVer": "5.0.0", "SemVerSourceIncrement": "Minor",
			"CommitCountSourceSha": "count-source-sha", "CommitCountSourceDistance": "2",
			"VersionSourceSemVer": "1.0.0", "VersionSourceIncrement": "None", "Sha": "head-sha",
		} {
			assertVariable(t, roundTrip, key, want)
		}
	case "CopyingEnumeratedVariablesDoesNotReuseOldSourceValues":
		empty.version = semver.Version{Major: 1}
		original := renderCase(t, empty, settings)
		if _, ok := original["SemVerSourceSha"]; !ok {
			t.Fatal("original is missing SemVerSourceSha")
		}
		copy := maps.Clone(original)
		copy["SemVerSourceSha"] = "updated-source"
		setSourceDistance(copy, int64(3))
		assertVariable(t, copy, "SemVerSourceSha", "updated-source")
		assertVariable(t, copy, "CommitCountSourceDistance", "3")
		assertNullableVariable(t, original, "SemVerSourceSha", nil)
	default:
		t.Fatalf("unhandled original method %s", method)
	}
}

func releaseVersion(version semver.Version, label string, number int64) semver.Version {
	version.Label, version.Number = label, &number
	return version
}

func renderCase(t *testing.T, input variableInput, settings config.OutputSettings) map[string]any {
	t.Helper()
	got, err := renderVariables(input, settings, nil)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func readVariableArgument(t *testing.T, argument json.RawMessage, target any) {
	t.Helper()
	if err := json.Unmarshal(argument, target); err != nil {
		t.Fatal(err)
	}
}

func roundTripVariables(t *testing.T, values map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var result map[string]any
	if err = decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertVariable(t *testing.T, values map[string]any, key, want string) {
	t.Helper()
	value, ok := values[key]
	got := ""
	if value != nil {
		got = fmt.Sprint(value)
	}
	if !ok || got != want {
		t.Fatalf("%s = %q (present %t), want %q", key, got, ok, want)
	}
}

func assertNullableVariable(t *testing.T, values map[string]any, key string, want *string) {
	t.Helper()
	var expected any
	if want != nil {
		expected = *want
	}
	if value, ok := values[key]; !ok || value != expected {
		t.Fatalf("%s = %v (present %t), want %v", key, value, ok, expected)
	}
}

package output

import "strings"

// VariableNames preserves the pinned public enumeration order.
var VariableNames = strings.Fields(
	"AssemblySemFileVer AssemblySemVer BranchName BuildMetaData CommitDate CommitCountSourceDistance CommitCountSourceSha " +
		"CustomVersion EscapedBranchName FullBuildMetaData FullSemVer InformationalVersion Major MajorMinorPatch Minor Patch " +
		"PreReleaseLabelName PreReleaseLabelNameWithDash PreReleaseNumber PreReleaseLabel PreReleaseLabelWithDash SemVer " +
		"SemVerSourceIncrement SemVerSourceSemVer SemVerSourceSha Sha ShortSha UncommittedChanges VersionSourceDistance " +
		"VersionSourceIncrement VersionSourceSemVer VersionSourceSha WeightedPreReleaseNumber",
)

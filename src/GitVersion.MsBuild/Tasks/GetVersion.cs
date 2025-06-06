using Microsoft.Build.Framework;

namespace GitVersion.MsBuild.Tasks;

public class GetVersion : GitVersionTaskBase
{
    [Output]
    public string Major { get; set; }

    [Output]
    public string Minor { get; set; }

    [Output]
    public string Patch { get; set; }

    [Output]
    public string PreReleaseTag { get; set; }

    [Output]
    public string PreReleaseTagWithDash { get; set; }

    [Output]
    public string PreReleaseLabel { get; set; }

    [Output]
    public string PreReleaseLabelWithDash { get; set; }

    [Output]
    public string PreReleaseNumber { get; set; }

    [Output]
    public string WeightedPreReleaseNumber { get; set; }

    [Output]
    public string BuildMetaData { get; set; }

    [Output]
    public string FullBuildMetaData { get; set; }

    [Output]
    public string MajorMinorPatch { get; set; }

    [Output]
    public string SemVer { get; set; }

    [Output]
    public string AssemblySemVer { get; set; }

    [Output]
    public string AssemblySemFileVer { get; set; }

    [Output]
    public string FullSemVer { get; set; }

    [Output]
    public string InformationalVersion { get; set; }

    [Output]
    public string BranchName { get; set; }

    [Output]
    public string EscapedBranchName { get; set; }

    [Output]
    public string Sha { get; set; }

    [Output]
    public string ShortSha { get; set; }

    [Output]
    public string CommitDate { get; set; }

    [Output]
    public string VersionSourceSha { get; set; }

    [Output]
    public string CommitsSinceVersionSource { get; set; }

    [Output]
    public string UncommittedChanges { get; set; }
}

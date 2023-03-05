using System.Text.Json.Serialization;
using GitVersion.Attributes;
using GitVersion.VersionCalculation;

namespace GitVersion.Model.Configuration;

public class BranchConfig
{
    public BranchConfig()
    {
    }

    /// <summary>
    /// Creates a clone of the given <paramref name="branchConfiguration"/>.
    /// </summary>
    public BranchConfig(BranchConfig branchConfiguration)
    {
        VersioningMode = branchConfiguration.VersioningMode;
        Tag = branchConfiguration.Tag;
        Increment = branchConfiguration.Increment;
        PreventIncrementOfMergedBranchVersion = branchConfiguration.PreventIncrementOfMergedBranchVersion;
        TagNumberPattern = branchConfiguration.TagNumberPattern;
        TrackMergeTarget = branchConfiguration.TrackMergeTarget;
        CommitMessageIncrementing = branchConfiguration.CommitMessageIncrementing;
        TracksReleaseBranches = branchConfiguration.TracksReleaseBranches;
        Regex = branchConfiguration.Regex;
        IsReleaseBranch = branchConfiguration.IsReleaseBranch;
        IsMainline = branchConfiguration.IsMainline;
        Name = branchConfiguration.Name;
        SourceBranches = branchConfiguration.SourceBranches;
        IsSourceBranchFor = branchConfiguration.IsSourceBranchFor;
        PreReleaseWeight = branchConfiguration.PreReleaseWeight;
    }

    [JsonPropertyName("mode")]
    [JsonPropertyDescription("The versioning mode for this branch. Can be 'ContinuousDelivery', 'ContinuousDeployment', 'Mainline'.")]
    public VersioningMode? VersioningMode { get; set; }

    /// <summary>
    /// Special value 'useBranchName' will extract the tag from the branch name
    /// </summary>
    [JsonPropertyName("tag")]
    [JsonPropertyDescription("The label to use for this branch. Can be 'useBranchName' to extract the label from the branch name.")]
    public string? Tag { get; set; }

    [JsonPropertyName("increment")]
    [JsonPropertyDescription("The increment strategy for this branch. Can be 'Inherit', 'Patch', 'Minor', 'Major', 'None'.")]
    public IncrementStrategy? Increment { get; set; }

    [JsonPropertyName("prevent-increment-of-merged-branch-version")]
    [JsonPropertyDescription("Prevent increment of merged branch version.")]
    public bool? PreventIncrementOfMergedBranchVersion { get; set; }

    [JsonPropertyName("tag-number-pattern")]
    [JsonPropertyDescription(@"The regex pattern to use to extract the number from the branch name. Defaults to '[/-](?<number>\d+)[-/]'.")]
    [JsonPropertyPattern(@"[/-](?<number>\d+)[-/]")]
    public string? TagNumberPattern { get; set; }

    [JsonPropertyName("track-merge-target")]
    [JsonPropertyDescription("Strategy which will look for tagged merge commits directly off the current branch.")]
    public bool? TrackMergeTarget { get; set; }

    [JsonPropertyName("commit-message-incrementing")]
    [JsonPropertyDescription("Sets whether it should be possible to increment the version with special syntax in the commit message. Can be 'Disabled', 'Enabled' or 'MergeMessageOnly'.")]
    public CommitMessageIncrementMode? CommitMessageIncrementing { get; set; }

    [JsonPropertyName("regex")]
    [JsonPropertyDescription("The regex pattern to use to match this branch.")]
    public string? Regex { get; set; }

    [JsonPropertyName("source-branches")]
    [JsonPropertyDescription("The source branches for this branch.")]
    public HashSet<string>? SourceBranches { get; set; }

    [JsonPropertyName("is-source-branch-for")]
    [JsonPropertyDescription("The branches that this branch is a source branch.")]
    public HashSet<string>? IsSourceBranchFor { get; set; }

    [JsonPropertyName("tracks-release-branches")]
    [JsonPropertyDescription("Indicates this branch config represents develop in GitFlow.")]
    public bool? TracksReleaseBranches { get; set; }

    [JsonPropertyName("is-release-branch")]
    [JsonPropertyDescription("Indicates this branch config represents a release branch in GitFlow.")]
    public bool? IsReleaseBranch { get; set; }

    [JsonPropertyName("is-mainline")]
    [JsonPropertyDescription("When using Mainline mode, this indicates that this branch is a mainline. By default main and support/* are mainlines.")]
    public bool? IsMainline { get; set; }

    [JsonPropertyName("pre-release-weight")]
    [JsonPropertyDescription("Provides a way to translate the PreReleaseLabel to a number.")]
    public int? PreReleaseWeight { get; set; }

    /// <summary>
    /// The name given to this configuration in the configuration file.
    /// </summary>
    [JsonIgnore]
    public string Name { get; set; }

    public void MergeTo(BranchConfig targetConfig)
    {
        if (targetConfig == null) throw new ArgumentNullException(nameof(targetConfig));

        targetConfig.VersioningMode = this.VersioningMode ?? targetConfig.VersioningMode;
        targetConfig.Tag = this.Tag ?? targetConfig.Tag;
        targetConfig.Increment = this.Increment ?? targetConfig.Increment;
        targetConfig.PreventIncrementOfMergedBranchVersion = this.PreventIncrementOfMergedBranchVersion ?? targetConfig.PreventIncrementOfMergedBranchVersion;
        targetConfig.TagNumberPattern = this.TagNumberPattern ?? targetConfig.TagNumberPattern;
        targetConfig.TrackMergeTarget = this.TrackMergeTarget ?? targetConfig.TrackMergeTarget;
        targetConfig.CommitMessageIncrementing = this.CommitMessageIncrementing ?? targetConfig.CommitMessageIncrementing;
        targetConfig.Regex = this.Regex ?? targetConfig.Regex;
        targetConfig.SourceBranches = this.SourceBranches ?? targetConfig.SourceBranches;
        targetConfig.IsSourceBranchFor = this.IsSourceBranchFor ?? targetConfig.IsSourceBranchFor;
        targetConfig.TracksReleaseBranches = this.TracksReleaseBranches ?? targetConfig.TracksReleaseBranches;
        targetConfig.IsReleaseBranch = this.IsReleaseBranch ?? targetConfig.IsReleaseBranch;
        targetConfig.IsMainline = this.IsMainline ?? targetConfig.IsMainline;
        targetConfig.PreReleaseWeight = this.PreReleaseWeight ?? targetConfig.PreReleaseWeight;
    }

    public BranchConfig Apply(BranchConfig overrides)
    {
        if (overrides == null) throw new ArgumentNullException(nameof(overrides));

        overrides.MergeTo(this);
        return this;
    }

    public static BranchConfig CreateDefaultBranchConfig(string name) => new()
    {
        Name = name,
        Tag = "useBranchName",
        PreventIncrementOfMergedBranchVersion = false,
        TrackMergeTarget = false,
        TracksReleaseBranches = false,
        IsReleaseBranch = false,
        IsMainline = false
    };
}

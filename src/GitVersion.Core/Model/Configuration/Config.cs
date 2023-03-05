using System.Globalization;
using System.Text.Json.Serialization;
using GitVersion.Attributes;
using GitVersion.Configuration;
using GitVersion.Extensions;
using GitVersion.VersionCalculation;

namespace GitVersion.Model.Configuration;

public class Config
{
    private string? nextVersion;

    public Config()
    {
        Branches = new Dictionary<string, BranchConfig>();
        Ignore = new IgnoreConfig();
    }

    [JsonPropertyName("assembly-versioning-scheme")]
    [JsonPropertyDescription("The scheme to use when setting AssemblyVersion attribute. Can be 'MajorMinorPatchTag', 'MajorMinorPatch', 'MajorMinor', 'Major', 'None'.")]
    public AssemblyVersioningScheme? AssemblyVersioningScheme { get; set; }

    [JsonPropertyName("assembly-file-versioning-scheme")]
    [JsonPropertyDescription("The scheme to use when setting AssemblyFileVersion attribute. Can be 'MajorMinorPatchTag', 'MajorMinorPatch', 'MajorMinor', 'Major', 'None'.")]
    public AssemblyFileVersioningScheme? AssemblyFileVersioningScheme { get; set; }

    [JsonPropertyName("assembly-informational-format")]
    [JsonPropertyDescription("Specifies the format of AssemblyInformationalVersion. The default value is {InformationalVersion}.")]
    public string? AssemblyInformationalFormat { get; set; }

    [JsonPropertyName("assembly-versioning-format")]
    [JsonPropertyDescription("Specifies the format of AssemblyVersion and overwrites the value of assembly-versioning-scheme.")]
    public string? AssemblyVersioningFormat { get; set; }

    [JsonPropertyName("assembly-file-versioning-format")]
    [JsonPropertyDescription("Specifies the format of AssemblyFileVersion and overwrites the value of assembly-file-versioning-scheme.")]
    public string? AssemblyFileVersioningFormat { get; set; }

    [JsonPropertyName("mode")]
    [JsonPropertyDescription("The versioning mode for this branch. Can be 'ContinuousDelivery', 'ContinuousDeployment', 'Mainline'.")]
    public VersioningMode? VersioningMode { get; set; }

    [JsonPropertyName("tag-prefix")]
    [JsonPropertyDescription($"A regex which is used to trim Git tags before processing. Defaults to {DefaultTagPrefix}")]
    public string? TagPrefix { get; set; }

    [JsonPropertyName("continuous-delivery-fallback-tag")]
    public string? ContinuousDeploymentFallbackTag { get; set; }

    [JsonPropertyName("next-version")]
    [JsonPropertyDescription("Allows you to bump the next version explicitly. Useful for bumping main or a feature branch with breaking changes")]
    public string? NextVersion
    {
        get => nextVersion;
        set =>
            nextVersion = int.TryParse(value, NumberStyles.Any, NumberFormatInfo.InvariantInfo, out var major)
                ? $"{major}.0"
                : value;
    }

    [JsonPropertyName("major-version-bump-message")]
    [JsonPropertyDescription(@"The regex to match commit messages with to perform a major version increment. Default set to '\+semver:\s?(breaking|major)'")]
    [JsonPropertyPattern(@"'\+semver:\s?(breaking|major)'")]
    public string? MajorVersionBumpMessage { get; set; }

    [JsonPropertyName("minor-version-bump-message")]
    [JsonPropertyDescription(@"The regex to match commit messages with to perform a minor version increment. Default set to '\+semver:\s?(feature|minor)'")]
    [JsonPropertyPattern(@"'\+semver:\s?(feature|minor)'")]
    public string? MinorVersionBumpMessage { get; set; }

    [JsonPropertyName("patch-version-bump-message")]
    [JsonPropertyDescription(@"The regex to match commit messages with to perform a patch version increment. Default set to '\+semver:\s?(fix|patch)'")]
    [JsonPropertyPattern(@"'\+semver:\s?(fix|patch)'")]
    public string? PatchVersionBumpMessage { get; set; }

    [JsonPropertyName("no-bump-message")]
    [JsonPropertyDescription(@"Used to tell GitVersion not to increment when in Mainline development mode. . Default set to '\+semver:\s?(none|skip)'")]
    [JsonPropertyPattern(@"'\+semver:\s?(none|skip)'")]
    public string? NoBumpMessage { get; set; }

    [JsonPropertyName("legacy-semver-padding")]
    public int? LegacySemVerPadding { get; set; }

    [JsonPropertyName("build-metadata-padding")]
    public int? BuildMetaDataPadding { get; set; }

    [JsonPropertyName("commits-since-version-source-padding")]
    public int? CommitsSinceVersionSourcePadding { get; set; }

    [JsonPropertyName("tag-pre-release-weight")]
    [JsonPropertyDescription("The pre-release weight in case of tagged commits. Defaults to 60000.")]
    public int? TagPreReleaseWeight { get; set; }
    [JsonPropertyName("commit-date-format")]
    [JsonPropertyDescription("The format to use when calculating the commit date. Defaults to 'yyyy-MM-dd'.")]
    [JsonPropertyPattern("'yyyy-MM-dd'", PatternFormat.DateTime)]
    public string? CommitDateFormat { get; set; }

    [JsonPropertyName("merge-message-formats")]
    [JsonPropertyDescription("Custom merge message formats to enable identification of merge messages that do not follow the built-in conventions.")]
    public Dictionary<string, string> MergeMessageFormats { get; set; } = new();

    [JsonPropertyName("update-build-number")]
    [JsonPropertyDescription("Whether to update the build number in the project file. Defaults to true.")]
    public bool? UpdateBuildNumber { get; set; } = true;


    [JsonPropertyName("commit-message-incrementing")]
    public CommitMessageIncrementMode? CommitMessageIncrementing { get; set; }

    [JsonPropertyName("branches")]
    [JsonPropertyDescription("The header for all the individual branch configuration.")]
    public Dictionary<string, BranchConfig> Branches { get; set; }

	[JsonPropertyName("ignore")]
    [JsonPropertyDescription("The header property for the ignore configuration.")]
    public IgnoreConfig Ignore { get; set; }

    [JsonPropertyName("increment")]
    [JsonPropertyDescription("The increment strategy for this branch. Can be 'Inherit', 'Patch', 'Minor', 'Major', 'None'.")]
    public IncrementStrategy? Increment { get; set; }
    public override string ToString()
    {
        var stringBuilder = new StringBuilder();
        using var stream = new StringWriter(stringBuilder);
        ConfigSerializer.Write(this, stream);
        stream.Flush();
        return stringBuilder.ToString();
    }

    public const string DefaultTagPrefix = "[vV]";
    public const string ReleaseBranchRegex = "^releases?[/-]";
    public const string FeatureBranchRegex = "^features?[/-]";
    public const string PullRequestRegex = @"^(pull|pull\-requests|pr)[/-]";
    public const string HotfixBranchRegex = "^hotfix(es)?[/-]";
    public const string SupportBranchRegex = "^support[/-]";
    public const string DevelopBranchRegex = "^dev(elop)?(ment)?$";
    public const string MainBranchRegex = "^master$|^main$";
    public const string MainBranchKey = "main";
    public const string MasterBranchKey = "master";
    public const string ReleaseBranchKey = "release";
    public const string FeatureBranchKey = "feature";
    public const string PullRequestBranchKey = "pull-request";
    public const string HotfixBranchKey = "hotfix";
    public const string SupportBranchKey = "support";
    public const string DevelopBranchKey = "develop";
}

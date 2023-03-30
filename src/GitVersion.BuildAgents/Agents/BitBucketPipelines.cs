using GitVersion.Extensions;
using GitVersion.Logging;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal class BitBucketPipelines : ICurrentBuildAgent
{
    private readonly IEnvironment environment;
    private readonly ILog log;
    public const string EnvironmentVariableName = "BITBUCKET_WORKSPACE";
    public const string BranchEnvironmentVariableName = "BITBUCKET_BRANCH";
    public const string TagEnvironmentVariableName = "BITBUCKET_TAG";
    public const string PullRequestEnvironmentVariableName = "BITBUCKET_PR_ID";
    private string? file;

    public BitBucketPipelines(IEnvironment environment, ILog log)
    {
        this.environment = environment.NotNull();
        this.log = log.NotNull();
        WithPropertyFile("gitversion.properties");
    }

    public string EnvironmentVariable => EnvironmentVariableName;

    public bool CanApplyToCurrentContext() => !this.environment.GetEnvironmentVariable(EnvironmentVariable).IsNullOrEmpty();

    public string GenerateSetVersionMessage(GitVersionVariables variables) => variables.FullSemVer;

    public void WithPropertyFile(string propertiesFileName) => this.file = propertiesFileName;

    public string[] GenerateSetParameterMessage(string name, string? value) => new[] { $"GITVERSION_{name.ToUpperInvariant()}={value}" };

    public void WriteIntegration(Action<string?> writer, GitVersionVariables variables, bool updateBuildNumber = true)
    {
        if (this.file is null)
            return;

        var @base = (ICurrentBuildAgent)this;
        @base.WriteIntegration(writer, variables, updateBuildNumber);
        writer($"Outputting variables to '{this.file}' ... ");
        writer("To import the file into your build environment, add the following line to your build step:");
        writer($"  - source {this.file}");
        writer("");
        writer("To reuse the file across build steps, add the file as a build artifact:");
        writer("  artifacts:");
        writer($"    - {this.file}");

        var exports = variables
            .Select(variable => $"export GITVERSION_{variable.Key.ToUpperInvariant()}={variable.Value}")
            .ToList();

        File.WriteAllLines(this.file, exports);
    }

    public string? GetCurrentBranch(bool usingDynamicRepos)
    {
        var branchName = EvaluateEnvironmentVariable(BranchEnvironmentVariableName);
        if (branchName?.StartsWith("refs/heads/") == true)
        {
            return branchName;
        }

        return null;
    }

    private string? EvaluateEnvironmentVariable(string variableName)
    {
        var branchName = this.environment.GetEnvironmentVariable(variableName);
        this.log.Info("Evaluating environment variable {0} : {1}", variableName, branchName ?? "(null)");
        return branchName;
    }
}

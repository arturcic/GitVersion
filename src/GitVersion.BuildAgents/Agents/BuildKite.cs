using GitVersion.Extensions;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal class BuildKite : ICurrentBuildAgent
{
    private readonly IEnvironment environment;

    public BuildKite(IEnvironment environment) => this.environment = environment.NotNull();

    public const string EnvironmentVariableName = "BUILDKITE";

    public string EnvironmentVariable => EnvironmentVariableName;

    public bool CanApplyToCurrentContext() => "true".Equals(this.environment.GetEnvironmentVariable(EnvironmentVariable), StringComparison.OrdinalIgnoreCase);

    public string GenerateSetVersionMessage(GitVersionVariables variables) =>
        string.Empty; // There is no equivalent function in BuildKite.

    public string[] GenerateSetParameterMessage(string name, string? value) =>
        Array.Empty<string>(); // There is no equivalent function in BuildKite.

    public string? GetCurrentBranch(bool usingDynamicRepos)
    {
        var pullRequest = this.environment.GetEnvironmentVariable("BUILDKITE_PULL_REQUEST");
        if (string.IsNullOrEmpty(pullRequest) || pullRequest == "false")
        {
            return this.environment.GetEnvironmentVariable("BUILDKITE_BRANCH");
        }
        else
        {
            // For pull requests BUILDKITE_BRANCH refers to the head, so adjust the
            // branch name for pull request versioning to function as expected
            return string.Format("refs/pull/{0}/head", pullRequest);
        }
    }

    public bool PreventFetch() => true;
}

using GitVersion.Extensions;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal class SpaceAutomation : ICurrentBuildAgent
{
    private readonly IEnvironment environment;

    public SpaceAutomation(IEnvironment environment) => this.environment = environment.NotNull();

    public const string EnvironmentVariableName = "JB_SPACE_PROJECT_KEY";

    public string EnvironmentVariable => EnvironmentVariableName;

    public bool CanApplyToCurrentContext() => !this.environment.GetEnvironmentVariable(EnvironmentVariable).IsNullOrEmpty();

    public string? GetCurrentBranch(bool usingDynamicRepos) => this.environment.GetEnvironmentVariable("JB_SPACE_GIT_BRANCH");

    public string[] GenerateSetParameterMessage(string name, string? value) => Array.Empty<string>();

    public string GenerateSetVersionMessage(GitVersionVariables variables) => string.Empty;
}

using GitVersion.Extensions;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal class ContinuaCi : ICurrentBuildAgent
{
    private readonly IEnvironment environment;

    public ContinuaCi(IEnvironment environment) => this.environment = environment.NotNull();

    public const string EnvironmentVariableName = "ContinuaCI.Version";

    public string EnvironmentVariable => EnvironmentVariableName;

    public bool CanApplyToCurrentContext() => !this.environment.GetEnvironmentVariable(EnvironmentVariable).IsNullOrEmpty();

    public string[] GenerateSetParameterMessage(string name, string? value) => new[]
    {
        $"@@continua[setVariable name='GitVersion_{name}' value='{value}' skipIfNotDefined='true']"
    };

    public string GenerateSetVersionMessage(GitVersionVariables variables) => $"@@continua[setBuildVersion value='{variables.FullSemVer}']";

    public bool PreventFetch() => false;
}

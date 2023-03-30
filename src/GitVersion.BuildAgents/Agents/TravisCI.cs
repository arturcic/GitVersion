using GitVersion.Extensions;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal class TravisCi : ICurrentBuildAgent
{
    private readonly IEnvironment environment;

    public TravisCi(IEnvironment environment) => this.environment = environment.NotNull();

    public const string EnvironmentVariableName = "TRAVIS";
    public string EnvironmentVariable => EnvironmentVariableName;

    public bool CanApplyToCurrentContext() => "true".Equals(this.environment.GetEnvironmentVariable(EnvironmentVariable))
                                              && "true".Equals(this.environment.GetEnvironmentVariable("CI"));

    public string GenerateSetVersionMessage(GitVersionVariables variables) => variables.FullSemVer;

    public string[] GenerateSetParameterMessage(string name, string? value) => new[] { $"GitVersion_{name}={value}" };

    public bool PreventFetch() => true;
}

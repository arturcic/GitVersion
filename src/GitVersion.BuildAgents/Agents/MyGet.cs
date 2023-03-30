using GitVersion.Extensions;
using GitVersion.Helpers;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal class MyGet : ICurrentBuildAgent
{
    private readonly IEnvironment environment;

    public MyGet(IEnvironment environment) => this.environment = environment.NotNull();

    public const string EnvironmentVariableName = "BuildRunner";
    public string EnvironmentVariable => EnvironmentVariableName;
    public bool CanApplyToCurrentContext()
    {
        var buildRunner = this.environment.GetEnvironmentVariable(EnvironmentVariable);

        return !buildRunner.IsNullOrEmpty()
               && buildRunner.Equals("MyGet", StringComparison.InvariantCultureIgnoreCase);
    }

    public string[] GenerateSetParameterMessage(string name, string? value)
    {
        var messages = new List<string>
        {
            $"##myget[setParameter name='GitVersion.{name}' value='{ServiceMessageEscapeHelper.EscapeValue(value)}']"
        };

        if (string.Equals(name, "SemVer", StringComparison.InvariantCultureIgnoreCase))
        {
            messages.Add($"##myget[buildNumber '{ServiceMessageEscapeHelper.EscapeValue(value)}']");
        }

        return messages.ToArray();
    }

    public string? GenerateSetVersionMessage(GitVersionVariables variables) => null;

    public bool PreventFetch() => false;
}

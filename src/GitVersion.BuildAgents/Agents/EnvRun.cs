using GitVersion.Extensions;
using GitVersion.Logging;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal class EnvRun : ICurrentBuildAgent
{
    private readonly IEnvironment environment;
    private readonly ILog log;

    public EnvRun(IEnvironment environment, ILog log)
    {
        this.environment = environment.NotNull();
        this.log = log;
    }

    public const string EnvironmentVariableName = "ENVRUN_DATABASE";
    public string EnvironmentVariable => EnvironmentVariableName;
    public bool CanApplyToCurrentContext()
    {
        var envRunDatabasePath = this.environment.GetEnvironmentVariable(EnvironmentVariableName);
        if (!envRunDatabasePath.IsNullOrEmpty())
        {
            if (!File.Exists(envRunDatabasePath))
            {
                this.log.Error($"The database file of EnvRun.exe was not found at {envRunDatabasePath}.");
                return false;
            }

            return true;
        }

        return false;
    }

    public string GenerateSetVersionMessage(GitVersionVariables variables) => variables.FullSemVer;

    public string[] GenerateSetParameterMessage(string name, string? value) => new[]
    {
        $"@@envrun[set name='GitVersion_{name}' value='{value}']"
    };
    public bool PreventFetch() => true;
}

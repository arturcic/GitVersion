using GitVersion.Extensions;
using GitVersion.Helpers;
using GitVersion.Logging;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal class TeamCity : ICurrentBuildAgent
{
    private readonly IEnvironment environment;
    private readonly ILog log;

    public TeamCity(IEnvironment environment, ILog log)
    {
        this.environment = environment.NotNull();
        this.log = log.NotNull();
    }

    public const string EnvironmentVariableName = "TEAMCITY_VERSION";

    public string EnvironmentVariable => EnvironmentVariableName;

    public bool CanApplyToCurrentContext() => !this.environment.GetEnvironmentVariable(EnvironmentVariable).IsNullOrEmpty();

    public string? GetCurrentBranch(bool usingDynamicRepos)
    {
        var branchName = this.environment.GetEnvironmentVariable("Git_Branch");

        if (branchName.IsNullOrEmpty())
        {
            if (!usingDynamicRepos)
            {
                WriteBranchEnvVariableWarning();
            }

            var @base = (ICurrentBuildAgent)this;
            return @base.GetCurrentBranch(usingDynamicRepos);
        }

        return branchName;
    }

    private void WriteBranchEnvVariableWarning() => this.log.Warning(@"TeamCity doesn't make the current branch available through environmental variables.
Depending on your authentication and transport setup of your git VCS root things may work. In that case, ignore this warning.
In your TeamCity build configuration, add a parameter called `env.Git_Branch` with value %teamcity.build.vcs.branch.<vcsid>%
See https://gitversion.net/docs/reference/build-servers/teamcity for more info");

    public bool PreventFetch() => !string.IsNullOrEmpty(this.environment.GetEnvironmentVariable("Git_Branch"));

    public string[] GenerateSetParameterMessage(string name, string? value) => new[]
    {
        $"##teamcity[setParameter name='GitVersion.{name}' value='{ServiceMessageEscapeHelper.EscapeValue(value)}']",
        $"##teamcity[setParameter name='system.GitVersion.{name}' value='{ServiceMessageEscapeHelper.EscapeValue(value)}']"
    };

    public string GenerateSetVersionMessage(GitVersionVariables variables) => $"##teamcity[buildNumber '{ServiceMessageEscapeHelper.EscapeValue(variables.FullSemVer)}']";
}

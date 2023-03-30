using System.Text.RegularExpressions;
using GitVersion.Extensions;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal class AzurePipelines : ICurrentBuildAgent
{
    private readonly IEnvironment environment;

    public AzurePipelines(IEnvironment environment) => this.environment = environment.NotNull();

    public const string EnvironmentVariableName = "TF_BUILD";

    public string EnvironmentVariable => EnvironmentVariableName;

    public bool CanApplyToCurrentContext() => !this.environment.GetEnvironmentVariable(EnvironmentVariable).IsNullOrEmpty();

    public string[] GenerateSetParameterMessage(string name, string? value) => new[]
    {
        $"##vso[task.setvariable variable=GitVersion.{name}]{value}",
        $"##vso[task.setvariable variable=GitVersion.{name};isOutput=true]{value}"
    };

    public string? GetCurrentBranch(bool usingDynamicRepos) => this.environment.GetEnvironmentVariable("BUILD_SOURCEBRANCH");

    public bool PreventFetch() => true;

    public string GenerateSetVersionMessage(GitVersionVariables variables)
    {
        // For AzurePipelines, we'll get the Build Number and insert GitVersion variables where
        // specified
        var buildNumberEnv = this.environment.GetEnvironmentVariable("BUILD_BUILDNUMBER");
        if (buildNumberEnv.IsNullOrWhiteSpace())
            return variables.FullSemVer;

        var newBuildNumber = variables.OrderBy(x => x.Key).Aggregate(buildNumberEnv, ReplaceVariables);

        // If no variable substitution has happened, use FullSemVer
        if (buildNumberEnv == newBuildNumber)
        {
            var buildNumber = variables.FullSemVer.EndsWith("+0")
                ? variables.FullSemVer.Substring(0, variables.FullSemVer.Length - 2)
                : variables.FullSemVer;

            return $"##vso[build.updatebuildnumber]{buildNumber}";
        }

        return $"##vso[build.updatebuildnumber]{newBuildNumber}";
    }

    private static string ReplaceVariables(string buildNumberEnv, KeyValuePair<string, string?> variable)
    {
        var pattern = $@"\$\(GITVERSION[_\.]{variable.Key}\)";
        var replacement = variable.Value;
        return replacement switch
        {
            null => buildNumberEnv,
            _ => buildNumberEnv.RegexReplace(pattern, replacement, RegexOptions.IgnoreCase)
        };
    }
}

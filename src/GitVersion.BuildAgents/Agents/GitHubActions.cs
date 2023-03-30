using GitVersion.Extensions;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal class GitHubActions : ICurrentBuildAgent
{
    private readonly IEnvironment environment;
    // https://help.github.com/en/actions/automating-your-workflow-with-github-actions/using-environment-variables#default-environment-variables

    public GitHubActions(IEnvironment environment) => this.environment = environment.NotNull();

    public const string EnvironmentVariableName = "GITHUB_ACTIONS";
    public const string GitHubSetEnvTempFileEnvironmentVariableName = "GITHUB_ENV";

    public string EnvironmentVariable => EnvironmentVariableName;

    public bool CanApplyToCurrentContext() => !this.environment.GetEnvironmentVariable(EnvironmentVariable).IsNullOrEmpty();

    public string GenerateSetVersionMessage(GitVersionVariables variables) =>
        string.Empty; // There is no equivalent function in GitHub Actions.

    public string[] GenerateSetParameterMessage(string name, string? value) =>
        Array.Empty<string>(); // There is no equivalent function in GitHub Actions.

    public void WriteIntegration(Action<string?> writer, GitVersionVariables variables, bool updateBuildNumber = true)
    {
        var @base = (ICurrentBuildAgent)this;
        @base.WriteIntegration(writer, variables, updateBuildNumber);

        // https://docs.github.com/en/free-pro-team@latest/actions/reference/workflow-commands-for-github-actions#environment-files
        // The outgoing environment variables must be written to a temporary file (identified by the $GITHUB_ENV environment
        // variable, which changes for every step in a workflow) which is then parsed. That file must also be UTF-8 or it will fail.
        var gitHubSetEnvFilePath = this.environment.GetEnvironmentVariable(GitHubSetEnvTempFileEnvironmentVariableName);

        if (gitHubSetEnvFilePath != null)
        {
            writer($"Writing version variables to $GITHUB_ENV file for '{GetType().Name}'.");
            using var streamWriter = File.AppendText(gitHubSetEnvFilePath);
            foreach (var (key, value) in variables)
            {
                if (!value.IsNullOrEmpty())
                {
                    streamWriter.WriteLine($"GitVersion_{key}={value}");
                }
            }
        }
        else
        {
            writer($"Unable to write GitVersion variables to ${GitHubSetEnvTempFileEnvironmentVariableName} because the environment variable is not set.");
        }
    }

    public string? GetCurrentBranch(bool usingDynamicRepos) => this.environment.GetEnvironmentVariable("GITHUB_REF");

    public bool PreventFetch() => true;
}

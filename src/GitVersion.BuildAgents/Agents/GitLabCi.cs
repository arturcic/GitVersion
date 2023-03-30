using GitVersion.Extensions;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal class GitLabCi : ICurrentBuildAgent
{
    private readonly IEnvironment environment;
    public const string EnvironmentVariableName = "GITLAB_CI";
    private string? file;

    public GitLabCi(IEnvironment environment)
    {
        this.environment = environment.NotNull();
        WithPropertyFile("gitversion.properties");
    }

    public void WithPropertyFile(string propertiesFileName) => this.file = propertiesFileName;

    public string EnvironmentVariable => EnvironmentVariableName;

    public bool CanApplyToCurrentContext() => !this.environment.GetEnvironmentVariable(EnvironmentVariable).IsNullOrEmpty();

    public string GenerateSetVersionMessage(GitVersionVariables variables) => variables.FullSemVer;

    public string[] GenerateSetParameterMessage(string name, string? value) => new[]
    {
        $"GitVersion_{name}={value}"
    };

    public string? GetCurrentBranch(bool usingDynamicRepos) => this.environment.GetEnvironmentVariable("CI_COMMIT_REF_NAME");

    public bool PreventFetch() => true;

    public void WriteIntegration(Action<string?> writer, GitVersionVariables variables, bool updateBuildNumber = true)
    {
        if (this.file is null)
            return;

        var @base = (ICurrentBuildAgent)this;
        @base.WriteIntegration(writer, variables, updateBuildNumber);
        writer($"Outputting variables to '{this.file}' ... ");

        File.WriteAllLines(this.file, @base.GenerateBuildLogOutput(variables));
    }
}

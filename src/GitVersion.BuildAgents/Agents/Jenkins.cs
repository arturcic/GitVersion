using GitVersion.Extensions;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal class Jenkins : ICurrentBuildAgent
{
    private readonly IEnvironment environment;
    public const string EnvironmentVariableName = "JENKINS_URL";
    private string? file;

    public Jenkins(IEnvironment environment)
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

    public string? GetCurrentBranch(bool usingDynamicRepos) => IsPipelineAsCode()
        ? this.environment.GetEnvironmentVariable("BRANCH_NAME")
        : this.environment.GetEnvironmentVariable("GIT_LOCAL_BRANCH") ?? this.environment.GetEnvironmentVariable("GIT_BRANCH");

    private bool IsPipelineAsCode() => !this.environment.GetEnvironmentVariable("BRANCH_NAME").IsNullOrEmpty();

    public bool PreventFetch() => true;

    /// <summary>
    /// When Jenkins uses pipeline-as-code, it creates two remotes: "origin" and "origin1".
    /// This should be cleaned up, so that normalizing the Git repo will not fail.
    /// </summary>
    /// <returns></returns>
    public bool ShouldCleanUpRemotes() => IsPipelineAsCode();

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

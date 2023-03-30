using GitVersion.Extensions;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal sealed class CodeBuild : ICurrentBuildAgent
{
    private readonly IEnvironment environment;
    private string? file;
    public const string WebHookEnvironmentVariableName = "CODEBUILD_WEBHOOK_HEAD_REF";
    public const string SourceVersionEnvironmentVariableName = "CODEBUILD_SOURCE_VERSION";

    public CodeBuild(IEnvironment environment)
    {
        this.environment = environment.NotNull();
        WithPropertyFile("gitversion.properties");
    }

    public void WithPropertyFile(string propertiesFileName) => this.file = propertiesFileName;

    public string EnvironmentVariable => WebHookEnvironmentVariableName;

    public string GenerateSetVersionMessage(GitVersionVariables variables) => variables.FullSemVer;

    public string[] GenerateSetParameterMessage(string name, string? value) => new[]
    {
        $"GitVersion_{name}={value}"
    };

    public string? GetCurrentBranch(bool usingDynamicRepos)
    {
        var currentBranch = this.environment.GetEnvironmentVariable(WebHookEnvironmentVariableName);

        return currentBranch.IsNullOrEmpty() ? this.environment.GetEnvironmentVariable(SourceVersionEnvironmentVariableName) : currentBranch;
    }

    public void WriteIntegration(Action<string?> writer, GitVersionVariables variables, bool updateBuildNumber = true)
    {
        if (this.file is null)
            return;

        var @base = (ICurrentBuildAgent)this;
        @base.WriteIntegration(writer, variables, updateBuildNumber);
        writer($"Outputting variables to '{this.file}' ... ");
        File.WriteAllLines(this.file, @base.GenerateBuildLogOutput(variables));
    }

    public bool PreventFetch() => true;

    public bool CanApplyToCurrentContext() => !this.environment.GetEnvironmentVariable(WebHookEnvironmentVariableName).IsNullOrEmpty()
                                            || !this.environment.GetEnvironmentVariable(SourceVersionEnvironmentVariableName).IsNullOrEmpty();
}

using GitVersion.OutputVariables;

namespace GitVersion.Agents;

public interface IBuildAgent
{
    bool IsDefault => false;
    bool CanApplyToCurrentContext();
    string? GetCurrentBranch(bool usingDynamicRepos) => null;
    bool PreventFetch() => true;
    bool ShouldCleanUpRemotes() => false;
}

public interface ICurrentBuildAgent : IBuildAgent
{
    protected string EnvironmentVariable { get; }
    protected string? GenerateSetVersionMessage(GitVersionVariables variables);
    protected string[] GenerateSetParameterMessage(string name, string? value);
    void WriteIntegration(Action<string?> writer, GitVersionVariables variables, bool updateBuildNumber = true)
    {
        if (updateBuildNumber)
        {
            writer($"Executing GenerateSetVersionMessage for '{GetType().Name}'.");
            writer(GenerateSetVersionMessage(variables));
        }
        writer($"Executing GenerateBuildLogOutput for '{GetType().Name}'.");
        foreach (var buildParameter in GenerateBuildLogOutput(variables))
        {
            writer(buildParameter);
        }
    }
    protected IEnumerable<string> GenerateBuildLogOutput(GitVersionVariables variables)
    {
        var output = new List<string>();

        foreach (var (key, value) in variables)
        {
            output.AddRange(GenerateSetParameterMessage(key, value));
        }

        return output;
    }
}

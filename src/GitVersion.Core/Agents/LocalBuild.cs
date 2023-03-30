using GitVersion.OutputVariables;

namespace GitVersion.Agents;

public class LocalBuild : ICurrentBuildAgent
{
    public bool IsDefault => true;

    public string EnvironmentVariable => string.Empty;
    public bool CanApplyToCurrentContext() => true;
    public string? GenerateSetVersionMessage(GitVersionVariables variables) => null;
    public string[] GenerateSetParameterMessage(string name, string? value) => Array.Empty<string>();
}

using Common.Utilities;

namespace Build.Tasks;

public class ValidateVersion
{
    public void Run(BuildContext context)
    {
        ArgumentNullException.ThrowIfNull(context.Version);
        var gitVersionTool = context.GetGitVersionToolLocation();
        context.ValidateOutput("dotnet", $"\"{gitVersionTool}\" --version", context.Version.GitVersion.InformationalVersion);
    }
}

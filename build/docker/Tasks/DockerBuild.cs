using Common.Utilities;

namespace Docker.Tasks;

[DockerRegistryArgument]
[DockerDotnetArgument]
[DockerDistroArgument]
[ArchitectureArgument]
public class DockerBuild
{
    public bool ShouldRun(BuildContext context)
    {
        var shouldRun = true;
        shouldRun &= context.ShouldRun(context.IsDockerOnLinux, $"{nameof(DockerBuild)} works only on Docker on Linux agents.");

        return shouldRun;
    }

    public void Run(BuildContext context)
    {
        var tool = Paths.Nuget.CombineWithFilePath("GitVersion.Tool*");
        var dest = Paths.Build.Combine("docker").Combine("nuget");
        context.EnsureDirectoryExists(dest);
        context.CopyFiles(tool.FullPath, dest);

        foreach (var dockerImage in context.Images)
        {
            context.DockerBuildImage(dockerImage);
        }
    }
}

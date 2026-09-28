using Common.Utilities;

namespace Artifacts.Tasks;

[DockerRegistryArgument]
[DockerDotnetArgument]
[DockerDistroArgument]
public class ArtifactsDotnetToolTest
{
    public bool ShouldRun(BuildContext context)
    {
        var shouldRun = true;
        shouldRun &= context.ShouldRun(context.IsDockerOnLinux, $"{nameof(ArtifactsDotnetToolTest)} works only on Docker on Linux agents.");

        return shouldRun;
    }

    public void Run(BuildContext context)
    {
        if (context.Version == null)
        {
            return;
        }

        var rootPrefix = string.Empty;
        var version = context.Version.NugetVersion;

        foreach (var dockerImage in context.Images)
        {
            if (context.SkipImageTesting(dockerImage))
            {
                continue;
            }

            var cmd = $"{rootPrefix}/scripts/test-global-tool.sh --version {version} --nugetPath {rootPrefix}/nuget --repoPath {rootPrefix}/repo";

            context.DockerTestArtifact(dockerImage, cmd);
        }
    }
}

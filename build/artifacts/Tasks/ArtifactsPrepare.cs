using Common.Utilities;

namespace Artifacts.Tasks;

[DockerRegistryArgument]
[DockerDotnetArgument]
[DockerDistroArgument]
public class ArtifactsPrepare
{
    public bool ShouldRun(BuildContext context)
    {
        var shouldRun = true;
        shouldRun &= context.ShouldRun(context.IsDockerOnLinux, $"{nameof(ArtifactsPrepare)} works only on Docker on Linux agents.");

        return shouldRun;
    }

    public void Run(BuildContext context)
    {
        foreach (var dockerImage in context.Images)
        {
            if (context.SkipImageTesting(dockerImage))
            {
                continue;
            }

            context.DockerPullImage(dockerImage);
        }
    }
}

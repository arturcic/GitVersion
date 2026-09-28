using Common.Utilities;

namespace Artifacts.Tasks;

[DockerRegistryArgument]
[DockerDotnetArgument]
[DockerDistroArgument]
public class ArtifactsTest
{
    public bool ShouldRun(BuildContext context)
    {
        var shouldRun = true;
        shouldRun &= context.ShouldRun(context.IsDockerOnLinux, $"{nameof(ArtifactsTest)} works only on Docker on Linux agents.");

        return shouldRun;
    }
}

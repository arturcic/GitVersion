using Common.Utilities;

namespace Docker.Tasks;

[DockerRegistryArgument]
[DockerDotnetArgument]
[DockerDistroArgument]
[ArchitectureArgument]
public class DockerTest
{
    public bool ShouldRun(BuildContext context)
    {
        var shouldRun = true;
        shouldRun &= context.ShouldRun(context.IsDockerOnLinux, $"{nameof(DockerTest)} works only on Docker on Linux agents.");

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

            context.DockerTestImage(dockerImage);
        }
    }
}

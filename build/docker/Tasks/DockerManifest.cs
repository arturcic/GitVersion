using Common.Utilities;

namespace Docker.Tasks;

[DockerRegistryArgument]
[DockerDotnetArgument]
[DockerDistroArgument]
public class DockerManifest
{
    public bool ShouldRun(BuildContext context)
    {
        var shouldRun = true;
        shouldRun &= context.ShouldRun(context.IsGitHubActionsBuild, $"{nameof(DockerPublish)} works only on GitHub Actions.");
        return shouldRun;
    }
}

public class DockerManifestInternal
{
    public bool ShouldRun(BuildContext context)
    {
        var shouldRun = true;
        shouldRun &= context.ShouldRun(context.IsGitHubActionsBuild, $"{nameof(DockerPublish)} works only on GitHub Actions.");
        shouldRun &= context.ShouldRun(context.IsDockerOnLinux, $"{nameof(DockerPublish)} works only on Docker on Linux agents.");

        if (context.DockerRegistry == DockerRegistry.GitHub)
        {
            shouldRun &= context.ShouldRun(context.IsInternalPreRelease, $"{nameof(DockerPublish)} to GitHub Package Registry works only internal releases.");
        }
        if (context.DockerRegistry == DockerRegistry.DockerHub)
        {
            shouldRun &= context.ShouldRun(context.IsTaggedRelease || context.IsTaggedPreRelease, $"{nameof(DockerPublish)} to DockerHub works only for tagged releases.");
        }

        return shouldRun;
    }

    public void Run(BuildContext context)
    {
        foreach (var group in context.Images.GroupBy(x => new { x.Distro, x.TargetFramework }))
        {
            var dockerImage = group.First();
            context.DockerManifest(dockerImage);
        }
    }
}

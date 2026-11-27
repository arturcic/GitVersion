using GitVersion.Configuration;
using GitVersion.Git;

namespace GitVersion;

internal sealed class DynamicClonePlan(
    IOptions<GitVersionOptions> options,
    BranchResolver branchResolver,
    Lazy<IGitVersionConfiguration> configuration)
{
    private readonly Lazy<DynamicCloneMode> mode = new(() => SelectMode(options.Value.RepositoryInfo, branchResolver.TargetBranch, configuration));

    public DynamicCloneMode Mode => this.mode.Value;

    private static DynamicCloneMode SelectMode(RepositoryInfo repository, string? targetBranch, Lazy<IGitVersionConfiguration> configuration)
    {
        if (!repository.UsePartialClone || string.IsNullOrWhiteSpace(repository.TargetUrl))
        {
            return DynamicCloneMode.Full;
        }

        // The bare clone fetches advertised heads. Preserve legacy preparation for
        // canonical refs outside refs/heads, such as pull request or tag refs.
        if (targetBranch is not null && targetBranch.StartsWith("refs/", StringComparison.Ordinal)
            && !targetBranch.StartsWith("refs/heads/", StringComparison.Ordinal))
        {
            return DynamicCloneMode.Full;
        }

        return configuration.Value.Ignore.Paths.Count == 0 ? DynamicCloneMode.Treeless : DynamicCloneMode.Blobless;
    }
}

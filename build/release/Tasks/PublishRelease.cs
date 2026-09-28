using Common.Utilities;

namespace Release.Tasks;


public class PublishRelease;

public class PublishReleaseInternal
{
    public bool ShouldRun(BuildContext context)
    {
        var shouldRun = true;
        shouldRun &= context.ShouldRun(context.IsGitHubActionsBuild, $"{nameof(PublishRelease)} works only on GitHub Actions.");
        shouldRun &= context.ShouldRun(context.IsTaggedRelease || context.IsTaggedPreRelease, $"{nameof(PublishRelease)} works only for tagged releases.");

        return shouldRun;
    }

    public void Run(BuildContext context)
    {
        var token = context.Credentials?.GitHub?.Token;
        if (string.IsNullOrEmpty(token))
        {
            throw new InvalidOperationException("Could not resolve GitHub Token.");
        }

        var archives = context.GetFiles(Paths.Native + "/*.{tar.gz,zip}").Select(x => x.ToString()).ToList();
        context.Information("Archives count: " + archives.Count);

        var assets = string.Join(",", archives);

        var milestone = context.Version?.Milestone;

        if (milestone is null)
        {
            return;
        }

        context.GitReleaseManagerCreate(token, Constants.RepoOwner, Constants.Repository, new GitReleaseManagerCreateSettings
        {
            Milestone = milestone,
            Name = milestone,
            Prerelease = false,
            TargetCommitish = Constants.DefaultBranch
        });

        context.GitReleaseManagerAddAssets(token, Constants.RepoOwner, Constants.Repository, milestone, assets);
        context.GitReleaseManagerClose(token, Constants.RepoOwner, Constants.Repository, milestone);
    }
}

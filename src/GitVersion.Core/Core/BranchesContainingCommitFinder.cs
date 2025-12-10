using GitVersion.Common;
using GitVersion.Extensions;
using GitVersion.Git;
using Microsoft.Extensions.Logging;

namespace GitVersion;

internal class BranchesContainingCommitFinder(IRepositoryStore repositoryStore, ILogger logger)
{
    private readonly ILogger logger = logger.NotNull();
    private readonly IRepositoryStore repositoryStore = repositoryStore.NotNull();

    public IEnumerable<IBranch> GetBranchesContainingCommit(ICommit commit, IEnumerable<IBranch>? branches = null, bool onlyTrackedBranches = false)
    {
        commit.NotNull();
        branches ??= [.. this.repositoryStore.Branches];

        // TODO Should we cache this?
        // Yielding part is split from the main part of the method to avoid having the exception check performed lazily.
        // Details at https://github.com/GitTools/GitVersion/issues/2755
        return InnerGetBranchesContainingCommit(commit, branches, onlyTrackedBranches);
    }

    private IEnumerable<IBranch> InnerGetBranchesContainingCommit(ICommit commit, IEnumerable<IBranch> branches, bool onlyTrackedBranches)
    {
        using (logger.BeginTimedOperation($"Getting branches containing the commit '{commit.Id}'"))
        {
            var directBranchHasBeenFound = false;
            logger.LogInformation("Trying to find direct branches.");
            // TODO: It looks wasteful looping through the branches twice. Can't these loops be merged somehow? @asbjornu
            var branchList = branches.ToList();
            foreach (var branch in branchList.Where(branch => BranchTipIsNullOrCommit(branch, commit) && !IncludeTrackedBranches(branch, onlyTrackedBranches)))
            {
                directBranchHasBeenFound = true;
                logger.LogInformation("Direct branch found: '{Branch}'", branch);
                yield return branch;
            }

            if (directBranchHasBeenFound)
            {
                yield break;
            }

            logger.LogInformation("No direct branches found, searching through {BranchType} branches", onlyTrackedBranches ? "tracked" : "all");
            foreach (var branch in branchList.Where(b => IncludeTrackedBranches(b, onlyTrackedBranches)))
            {
                logger.LogInformation("Searching for commits reachable from '{Branch}'", branch);

                var commits = this.repositoryStore.GetCommitsReacheableFrom(commit, branch);

                if (!commits.Any())
                {
                    logger.LogInformation("The branch '{Branch}' has no matching commits", branch);
                    continue;
                }

                logger.LogInformation("The branch '{Branch}' has a matching commit", branch);
                yield return branch;
            }
        }
    }

    private static bool IncludeTrackedBranches(IBranch branch, bool includeOnlyTracked)
        => (includeOnlyTracked && branch.IsTracking) || !includeOnlyTracked;

    private static bool BranchTipIsNullOrCommit(IBranch branch, ICommit commit)
        => branch.Tip == null || branch.Tip.Sha == commit.Sha;
}

using GitVersion.Extensions;

namespace GitVersion.Git;

internal sealed partial class ManagedGitRepository(ILogger<ManagedGitRepository> logger, IGitCliMutator cliMutator) : IMutatingGitRepository, ILightweightGitRepository
{
    private readonly ILogger<ManagedGitRepository> logger = logger.NotNull();

    internal IGitCliMutator CliMutator { get; } = cliMutator.NotNull();

    public void Clone(string? sourceUrl, string? workdirPath, AuthenticationInfo auth) =>
        CliMutator.Clone(sourceUrl, workdirPath, auth);

    public void CloneLightweight(string? sourceUrl, string? gitDirectory, AuthenticationInfo authentication, DynamicCloneMode mode) =>
        CliMutator.CloneLightweight(sourceUrl, gitDirectory, authentication, mode);

    public void SelectBranchWithoutCheckout(string canonicalName)
    {
        var otherLocalBranches = Branches.Where(branch => !branch.IsRemote && branch.Name.Canonical != canonicalName)
            .Select(branch => branch.Name.Canonical).ToArray();
        CliMutator.CreateSymbolicReference(CliWorkingDirectory, "HEAD", canonicalName);
        foreach (var branch in otherLocalBranches)
        {
            CliMutator.DeleteReference(CliWorkingDirectory, branch);
        }
        Invalidate();
    }

    public void ValidateLightweightHistory(DynamicCloneMode mode) => CliMutator.ValidateLightweightHistory(CliWorkingDirectory, mode);

    public void Checkout(string commitOrBranchSpec)
    {
        CliMutator.Checkout(CliWorkingDirectory, commitOrBranchSpec);
        Invalidate();
    }

    public void Fetch(string remote, IEnumerable<string> refSpecs, AuthenticationInfo auth, string? logMessage)
    {
        CliMutator.Fetch(CliWorkingDirectory, remote, refSpecs, auth);
        Invalidate();
    }

    public void CreateBranchForPullRequestBranch(AuthenticationInfo auth) =>
        PullRequestBranchOperations.CreateBranchForPullRequestBranch(
            this,
            this.logger,
            remoteName => CliMutator.ListRemoteReferences(CliWorkingDirectory, remoteName, auth));
}

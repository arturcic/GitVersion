namespace GitVersion.Git;

/// <summary>Optional support for dynamic repositories without a working tree.</summary>
internal interface ILightweightGitRepository
{
    void CloneLightweight(string? sourceUrl, string? gitDirectory, AuthenticationInfo authentication, DynamicCloneMode mode);
    void SelectBranchWithoutCheckout(string canonicalName);
    void ValidateLightweightHistory(DynamicCloneMode mode);
}

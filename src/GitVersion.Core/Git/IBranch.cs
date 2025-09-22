namespace GitVersion.Git;

public interface IBranch : IEquatable<IBranch?>, IComparable<IBranch>, INamedReference, IDisposable
{
    ICommit? Tip { get; }
    bool IsRemote { get; }
    bool IsTracking { get; }
    bool IsDetachedHead { get; }
    ICommitCollection Commits { get; }
}

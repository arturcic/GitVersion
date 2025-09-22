namespace GitVersion.Git;

public interface ITag : IEquatable<ITag?>, IComparable<ITag>, INamedReference, IDisposable
{
    string TargetSha { get; }

    ICommit Commit { get; }
}

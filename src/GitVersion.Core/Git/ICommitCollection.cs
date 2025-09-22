namespace GitVersion.Git;

public interface ICommitCollection : IEnumerable<ICommit>, IDisposable
{
    IEnumerable<ICommit> GetCommitsPriorTo(DateTimeOffset olderThan);
    IEnumerable<ICommit> QueryBy(CommitFilter commitFilter);
}

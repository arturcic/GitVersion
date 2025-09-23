using GitVersion.Extensions;
using GitVersion.Helpers;
using LibGit2Sharp;

namespace GitVersion.Git;

internal sealed class Tag : ITag
{
    private static readonly LambdaEqualityHelper<ITag> equalityHelper = new(x => x.Name.Canonical);
    private static readonly LambdaKeyComparer<ITag, string> comparerHelper = new(x => x.Name.Canonical);
    private readonly IRepository repositoryInstance;
    private readonly LibGit2Sharp.Tag innerTag;
    private readonly Diff diff;
    private readonly Lazy<ICommit?> commitLazy;

    internal Tag(IRepository repositoryInstance, LibGit2Sharp.Tag tag, Diff diff)
    {
        this.repositoryInstance = repositoryInstance;
        this.innerTag = tag.NotNull();
        this.commitLazy = new(PeeledTargetCommit);
        this.diff = diff.NotNull();
        Name = new(this.innerTag.CanonicalName);
    }

    public ReferenceName Name { get; }
    public int CompareTo(ITag? other) => comparerHelper.Compare(this, other);
    public bool Equals(ITag? other) => equalityHelper.Equals(this, other);
    public string TargetSha => this.innerTag.Target.Sha;
    public ICommit Commit => this.commitLazy.Value.NotNull();

    private Commit? PeeledTargetCommit()
    {
        var target = this.innerTag.Target;

        while (target is TagAnnotation annotation)
        {
            target = annotation.Target;
        }

        return target is LibGit2Sharp.Commit commit ? new Commit(this.repositoryInstance, commit, this.diff) : null;
    }

    public override bool Equals(object? obj) => Equals(obj as ITag);
    public override int GetHashCode() => equalityHelper.GetHashCode(this);
    public override string ToString() => Name.ToString();

    public void Dispose() => this.commitLazy.Value?.Dispose();
}

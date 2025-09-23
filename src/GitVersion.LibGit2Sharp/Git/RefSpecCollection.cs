using GitVersion.Extensions;
using LibGit2Sharp;

namespace GitVersion.Git;

internal sealed class RefSpecCollection : IRefSpecCollection
{
    private readonly Lazy<IReadOnlyCollection<IRefSpec>> refSpecs;

    internal RefSpecCollection(IRepository repositoryInstance, LibGit2Sharp.RefSpecCollection collection)
    {
        collection = collection.NotNull();
        this.refSpecs = new Lazy<IReadOnlyCollection<IRefSpec>>(() => [.. collection.Select(tag => new RefSpec(repositoryInstance, tag))]);
    }

    public IEnumerator<IRefSpec> GetEnumerator() => this.refSpecs.Value.GetEnumerator();
    IEnumerator IEnumerable.GetEnumerator() => GetEnumerator();
}

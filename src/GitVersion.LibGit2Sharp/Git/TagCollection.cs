using GitVersion.Extensions;
using LibGit2Sharp;

namespace GitVersion.Git;

internal sealed class TagCollection : ITagCollection
{
    private readonly Lazy<IReadOnlyCollection<ITag>> tags;

    internal TagCollection(IRepository repositoryInstance, LibGit2Sharp.TagCollection collection, Diff diff)
    {
        collection = collection.NotNull();
        this.tags = new Lazy<IReadOnlyCollection<ITag>>(() => [.. collection.Select(tag => new Tag(repositoryInstance, tag, diff))]);
    }

    public IEnumerator<ITag> GetEnumerator()
        => this.tags.Value.GetEnumerator();

    IEnumerator IEnumerable.GetEnumerator() => GetEnumerator();
}

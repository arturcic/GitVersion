using GitVersion.Extensions;
using LibGit2Sharp;

namespace GitVersion.Git;

internal sealed class RemoteCollection : IRemoteCollection
{
    private readonly IRepository repositoryInstance;
    private readonly LibGit2Sharp.RemoteCollection innerCollection;
    private IReadOnlyCollection<IRemote>? remotes;

    internal RemoteCollection(IRepository repositoryInstance, LibGit2Sharp.RemoteCollection collection)
    {
        this.repositoryInstance = repositoryInstance.NotNull();
        this.innerCollection = collection.NotNull();
    }

    public IEnumerator<IRemote> GetEnumerator()
    {
        this.remotes ??= [.. this.innerCollection.Select(reference => new Remote(this.repositoryInstance, reference))];
        return this.remotes.GetEnumerator();
    }

    IEnumerator IEnumerable.GetEnumerator() => GetEnumerator();

    public IRemote? this[string name]
    {
        get
        {
            var remote = this.innerCollection[name];
            return remote is null ? null : new Remote(this.repositoryInstance, remote);
        }
    }

    public void Remove(string remoteName)
    {
        this.innerCollection.Remove(remoteName);
        this.remotes = null;
    }

    public void Update(string remoteName, string refSpec)
    {
        this.innerCollection.Update(remoteName, r => r.FetchRefSpecs.Add(refSpec));
        this.remotes = null;
    }

    public void Dispose()
    {
        if (this.remotes == null) return;
        foreach (var remote in this.remotes)
        {
            remote.Dispose();
        }
    }
}

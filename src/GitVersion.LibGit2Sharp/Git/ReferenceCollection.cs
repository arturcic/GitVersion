using GitVersion.Extensions;
using LibGit2Sharp;

namespace GitVersion.Git;

internal sealed class ReferenceCollection : IReferenceCollection
{
    private readonly IRepository repositoryInstance;
    private readonly LibGit2Sharp.ReferenceCollection innerCollection;
    private IReadOnlyCollection<IReference>? references;

    internal ReferenceCollection(IRepository repositoryInstance, LibGit2Sharp.ReferenceCollection collection)
    {
        this.repositoryInstance = repositoryInstance.NotNull();
        this.innerCollection = collection.NotNull();
    }

    public IEnumerator<IReference> GetEnumerator()
    {
        this.references ??= [.. this.innerCollection.Select(reference => new Reference(this.repositoryInstance, reference))];
        return this.references.GetEnumerator();
    }

    public void Add(string name, string canonicalRefNameOrObject, bool allowOverwrite = false) => this.innerCollection.Add(name, canonicalRefNameOrObject, allowOverwrite);

    public void UpdateTarget(IReference directRef, IObjectId targetId)
    {
        RepositoryExtensions.RunSafe(() => this.innerCollection.UpdateTarget((Reference)directRef, (ObjectId)targetId));
        this.references = null;
    }

    IEnumerator IEnumerable.GetEnumerator() => GetEnumerator();

    public IReference? this[string name]
    {
        get
        {
            var reference = this.innerCollection[name];
            return reference is null ? null : new Reference(this.repositoryInstance, reference);
        }
    }

    public IReference? this[ReferenceName referenceName] => this[referenceName.Canonical];

    public IReference? Head => this["HEAD"];

    public IEnumerable<IReference> FromGlob(string prefix) => this.innerCollection.FromGlob(prefix).Select(reference => new Reference(this.repositoryInstance, reference));

    public void Dispose()
    {
        if (this.references == null) return;
        foreach (var reference in this.references)
        {
            reference.Dispose();
        }
    }
}

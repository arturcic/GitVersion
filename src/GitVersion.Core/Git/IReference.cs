namespace GitVersion.Git;

public interface IReference : IEquatable<IReference?>, IComparable<IReference>, INamedReference, IDisposable
{
    string TargetIdentifier { get; }
    IObjectId? ReferenceTargetId { get; }
}

using System.IO.Abstractions;
using System.Reflection;
using System.Runtime.Loader;
using System.Text.Json;
using GitVersion;
using GitVersion.Git;
using Microsoft.Extensions.Logging.Abstractions;
using Microsoft.Extensions.Options;

if (args.Length != 4)
    throw new ArgumentException("usage: DualBackend METHOD REPOSITORY FIXTURE_ROOT BASELINE");

var (method, path, fixture, published) = (args[0], args[1], args[2], args[3]);
AssemblyLoadContext.Default.Resolving += (_, name) =>
{
    var candidate = Path.Combine(published, name.Name + ".dll");
    return File.Exists(candidate) ? AssemblyLoadContext.Default.LoadFromAssemblyPath(candidate) : null;
};

var managedAssembly = Assembly.Load("GitVersion.Git.Managed");
var libAssembly = Assembly.Load("GitVersion.LibGit2Sharp");
var managedType = managedAssembly.GetType("GitVersion.Git.ManagedGitRepository", throwOnError: true)!;
var libType = libAssembly.GetType("GitVersion.Git.GitRepository", throwOnError: true)!;

object expected;
object actual;
if (method == "BareRepositoryHasNullProjectRootDirectoryOnBothBackends")
{
    expected = BareInfo(libAssembly, "GitRepositoryInfo");
    actual = BareInfo(managedAssembly, "ManagedGitRepositoryInfo");
}
else
{
    var executor = Logged(managedAssembly.GetType("GitVersion.Git.GitCliExecutor", true)!);
    var mutator = Logged(managedAssembly.GetType("GitVersion.Git.GitCliMutator", true)!, executor);
    using var managed = (IGitRepository)Logged(managedType, mutator);
    using var lib = (IGitRepository)Logged(libType);
    managed.DiscoverRepository(path);
    lib.DiscoverRepository(path);
    expected = Observe(lib);
    actual = Observe(managed);
}

var expectedJson = JsonSerializer.Serialize(expected);
var actualJson = JsonSerializer.Serialize(actual);
if (expectedJson != actualJson)
    throw new InvalidOperationException($"Pinned adapters differ for {method}: libgit2={expectedJson}, managed={actualJson}");
Console.WriteLine(expectedJson);

static object Logged(Type type, params object[] dependencies)
{
    var loggerType = typeof(NullLogger<>).MakeGenericType(type);
    var logger = loggerType.GetField("Instance")!.GetValue(null)!;
    return Activator.CreateInstance(type, [logger, .. dependencies])!;
}

object BareInfo(Assembly assembly, string type)
{
    var info = (IGitRepositoryInfo)Activator.CreateInstance(
        assembly.GetType("GitVersion.Git." + type, true)!, new FileSystem(),
        Options.Create(new GitVersionOptions { WorkingDirectory = path }))!;
    return new { root = Location(info.ProjectRootDirectory), git = Location(info.DotGitDirectory) };
}

string? Location(string? value) => value is null ? null
    : Path.GetRelativePath(fixture, Path.TrimEndingDirectorySeparator(value)).Replace(Path.DirectorySeparatorChar, '/');

static object? Head(IBranch b) => new { name = b.Name.Canonical, tip = b.Tip?.Sha };
static object Branch(IBranch b) => new { name = b.Name.Canonical, tip = b.Tip?.Sha, remote = b.IsRemote, tracking = b.IsTracking, detached = b.IsDetachedHead };
static object[] Refs(IGitRepository r) => r.References.Select(x => new { name = x.Name.Canonical, target = x.TargetIdentifier, id = x.ReferenceTargetId?.Sha }).ToArray();
static object[] Tags(IGitRepository r) => r.Tags.Select(t => new { name = t.Name.Canonical, target = t.TargetSha, commit = t.Commit.Sha }).ToArray();
static string[] IDs(IEnumerable<ICommit> commits) => commits.Select(c => c.Sha).ToArray();
static object[] Log(IGitRepository r) => r.Commits.Select(c => new { id = c.Sha, merge = c.IsMergeCommit, parents = IDs(c.Parents) }).ToArray();

object Paths(IGitRepository r) => new { git = Location(r.Path), root = Location(r.WorkingDirectory) };
static Dictionary<string, ICommit> All(IGitRepository r) => r.Branches.SelectMany(b => b.Commits)
    .DistinctBy(c => c.Sha).OrderBy(c => c.Sha, StringComparer.Ordinal)
    .ToDictionary(c => c.Sha, StringComparer.Ordinal);
static object Diffs(IGitRepository r) => All(r).ToDictionary(x => x.Key, x => x.Value.DiffPaths.ToArray());
static object Bases(IGitRepository r)
{
    var all = All(r);
    return all.Values.SelectMany(a => all.Values.Select(b => new { a = a.Sha, b = b.Sha, result = r.FindMergeBase(a, b)?.Sha })).ToArray();
}
static object Queries(IGitRepository r)
{
    var all = All(r);
    int[] sorts = [0, 2, 1, 3, 6, 7];
    return new
    {
        ids = all.Keys.ToArray(),
        queries = (from sort in sorts
                   from first in new[] { false, true }
                   from exclude in new string?[] { null }.Concat(all.Keys.Take(3))
                   select new
                   {
                       sort, first, exclude,
                       ids = IDs(r.Commits.QueryBy(new CommitFilter
                       {
                           IncludeReachableFrom = r.Branches["main"],
                           ExcludeReachableFrom = exclude is null ? null : all[exclude],
                           FirstParentOnly = first,
                           SortBy = (CommitSortStrategies)sort,
                       })),
                   }).ToArray(),
    };
}

object Observe(IGitRepository r) => method switch
{
    "ReferencesAreIdenticalOnBothBackends" => new
    {
        refs = Refs(r),
        head = new { name = r.References.Head!.Name.Canonical, target = r.References.Head!.TargetIdentifier },
        glob = r.References.FromGlob("refs/heads/*").Select(x => new { name = x.Name.Canonical, target = x.TargetIdentifier }).ToArray(),
    },
    "BranchesAreIdenticalOnBothBackends" => new
    {
        branches = r.Branches.Select(Branch).ToArray(),
        lookups = new[] { "main", "develop", "refs/heads/develop", "feature/complex", "missing" }
            .Select(name => r.Branches[name] is { } b ? Head(b) : null).ToArray(),
        head = Head(r.Head), detached = r.IsHeadDetached, shallow = r.IsShallow,
    },
    "TagsAreIdenticalOnBothBackends" or "LightweightAndAnnotatedTagOnTheSameCommitBehaveIdenticallyOnBothBackends" => Tags(r),
    "CommitLogsAreIdenticalOnBothBackends" => new
    {
        log = r.Commits.Select(c => new { id = c.Sha, when = c.When.ToUnixTimeSeconds(), offset = (int)c.When.Offset.TotalMinutes,
            merge = c.IsMergeCommit, parents = IDs(c.Parents), message = c.Message }).ToArray(),
        branches = new[] { "main", "develop", "feature/complex" }.Select(name => IDs(r.Branches[name]!.Commits)).ToArray(),
    },
    "QueryBySequencesAreIdenticalOnBothBackends" => Queries(r),
    "MergeBasesAreIdenticalOnBothBackendsForAllCommitPairs" => Bases(r),
    "DiffPathsAreIdenticalOnBothBackendsForAllCommits" => Diffs(r),
    "UncommittedChangesCountIsIdenticalOnBothBackends" or "IndexVersionFourBehavesIdenticallyOnBothBackends" => r.UncommittedChangesCount(),
    "EmptyRepositoryBehavesIdenticallyOnBothBackends" => new { head = Head(r.Head), detached = r.IsHeadDetached, changes = r.UncommittedChangesCount() },
    "DetachedHeadBehavesIdenticallyOnBothBackends" => new { head = Head(r.Head), detached = r.IsHeadDetached, headDetached = r.Head.IsDetachedHead },
    "RemotesAndRemoteBranchesAreIdenticalOnBothBackends" => new
    {
        remotes = r.Remotes.Select(remote => new
        {
            name = remote.Name, url = Location(remote.Url),
            fetch = remote.FetchRefSpecs.Select(s => new { specification = s.Specification, direction = s.Direction.ToString(), source = s.Source, destination = s.Destination }).ToArray(),
            push = remote.PushRefSpecs.Select(s => s.Specification).ToArray(),
        }).ToArray(),
        origin = Location(r.Remotes["origin"]!.Url),
        branches = r.Branches.Select(Branch).ToArray(),
    },
    "RepositoryPathsAreIdenticalOnBothBackends" => Paths(r),
    "OctopusMergeBehavesIdenticallyOnBothBackends" => new { parents = r.Head.Tip!.Parents.Count, log = Log(r), bases = Bases(r), diffs = Diffs(r) },
    "PackedAndLooseReferencesBehaveIdenticallyOnBothBackends" => new { refs = Refs(r), tags = Tags(r), branches = r.Branches.Select(Head).ToArray() },
    "ShallowCloneBehavesIdenticallyOnBothBackends" => new { shallow = r.IsShallow, log = Log(r) },
    "WorktreeBehavesIdenticallyOnBothBackends" => new { paths = Paths(r), head = Head(r.Head), changes = r.UncommittedChangesCount(), refs = Refs(r), log = Log(r) },
    _ => throw new ArgumentException("Unknown original test " + method),
};

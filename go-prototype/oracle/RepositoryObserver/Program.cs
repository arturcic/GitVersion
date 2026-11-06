using System.Reflection;
using System.Runtime.Loader;
using System.Text.Json;
using GitVersion.Git;

// This probe invokes the already-pinned managed assembly. It does not rebuild,
// patch or copy repository algorithms from the reference implementation.
internal static class Program
{
    private static int Main(string[] args)
    {
        try
        {
            if (args.Length != 2) throw new ArgumentException("usage: RepositoryObserver BASELINE_DIRECTORY REPOSITORY");
            var baseline = Path.GetFullPath(args[0]);
            AssemblyLoadContext.Default.Resolving += (_, name) => AssemblyLoadContext.Default.LoadFromAssemblyPath(Path.Combine(baseline, name.Name + ".dll"));
            Observe(baseline, Path.GetFullPath(args[1]));
            return 0;
        }
        catch (Exception ex) { Console.Error.WriteLine(ex); return 1; }
    }

    private static void Observe(string baseline, string path)
    {
        var assembly = AssemblyLoadContext.Default.LoadFromAssemblyPath(Path.Combine(baseline, "GitVersion.Git.Managed.dll"));
        var type = assembly.GetType("GitVersion.Git.ManagedGitRepository", throwOnError: true)!;
        var logging = AssemblyLoadContext.Default.LoadFromAssemblyPath(Path.Combine(baseline, "Microsoft.Extensions.Logging.Abstractions.dll"));
        var loggerType = logging.GetType("Microsoft.Extensions.Logging.Abstractions.NullLogger`1", true)!.MakeGenericType(type);
        var logger = loggerType.GetField("Instance", BindingFlags.Public | BindingFlags.Static)?.GetValue(null)
            ?? throw new InvalidOperationException("Pinned NullLogger<T>.Instance field was not found");
        // An observation must never launch native Git or mutate the repository.
        var mutator = DispatchProxy.Create(assembly.GetType("GitVersion.Git.IGitCliMutator", true)!, typeof(RejectMutationProxy));
        using var repo = (IGitRepository)Activator.CreateInstance(type, logger, mutator)!;
        repo.DiscoverRepository(path);
        if (repo.IsHeadDetached || repo.IsShallow || repo.Head.Tip is null || repo.UncommittedChangesCount() != 0)
            throw new InvalidOperationException("unsupported repository state");
        var branches = repo.Branches.ToArray();
        var tags = repo.Tags.ToArray();
        var tips = branches.Select(b => b.Tip!).Concat(tags.Select(t => t.Commit)).Append(repo.Head.Tip)
            .DistinctBy(c => c.Sha).OrderBy(c => c.Sha, StringComparer.Ordinal).ToArray();
        var commits = new SortedDictionary<string, object>(StringComparer.Ordinal);
        var walks = new SortedDictionary<string, string[]>(StringComparer.Ordinal);
        var bases = new SortedDictionary<string, string?>(StringComparer.Ordinal);
        foreach (var tip in tips)
            foreach (var c in repo.Commits.QueryBy(new CommitFilter { IncludeReachableFrom = tip, SortBy = CommitSortStrategies.Time }))
                commits[c.Sha] = new { parents = c.Parents.Select(p => p.Sha).ToArray(), message = c.Message, when = c.When.ToUnixTimeSeconds(), diff_paths = c.DiffPaths.ToArray() };
        for (var sort = 0; sort < 8; sort++)
            foreach (var first in new[] { false, true })
            {
                var filter = new CommitFilter { IncludeReachableFrom = repo.Head.Tip, SortBy = (CommitSortStrategies)sort, FirstParentOnly = first };
                var key = $"{sort}/{first.ToString().ToLowerInvariant()}";
                walks["head/" + key] = repo.Commits.QueryBy(filter).Select(c => c.Sha).ToArray();
                foreach (var branch in branches)
                    walks[$"exclude/{branch.Name.Canonical}/{key}"] = repo.Commits.QueryBy(filter with { ExcludeReachableFrom = branch }).Select(c => c.Sha).ToArray();
            }
        foreach (var branch in branches) walks["branch/" + branch.Name.Canonical] = branch.Commits.Select(c => c.Sha).ToArray();
        foreach (var a in tips)
            foreach (var b in tips) bases[a.Sha + "/" + b.Sha] = repo.FindMergeBase(a, b)?.Sha;
        Console.WriteLine(JsonSerializer.Serialize(new {
            schema = 1, head = repo.Head.Tip.Sha, branch = repo.Head.Name.Canonical,
            references = repo.References.Select(r => new { name = r.Name.Canonical, target = r.TargetIdentifier, sha = r.ReferenceTargetId?.Sha }).ToArray(),
            tags = tags.Select(t => new { name = t.Name.Canonical, target = t.TargetSha, commit = t.Commit.Sha }).ToArray(),
            commits, walks, merge_bases = bases, uncommitted = repo.UncommittedChangesCount()
        }));
    }
}

public class RejectMutationProxy : DispatchProxy
{
    protected override object? Invoke(MethodInfo? method, object?[]? args) =>
        throw new InvalidOperationException("Repository observer attempted mutation: " + method?.Name);
}

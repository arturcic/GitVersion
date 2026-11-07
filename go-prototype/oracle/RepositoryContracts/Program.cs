using System.Text.Json;
using LibGit2Sharp;

if (args.Length < 2)
    throw new ArgumentException("usage: RepositoryContracts status|diff|walk|merge-base REPOSITORY [OPTIONS]");

using var repository = new Repository(args[1]);

// These are the reference expressions in the pinned NUnit tests.
object result = args[0] switch
{
    "status" => repository.Head.Tip is null
        ? EmptyRepositoryCount(repository)
        : repository.Diff.Compare<TreeChanges>(repository.Head.Tip.Tree, DiffTargets.Index | DiffTargets.WorkingDirectory).Count,
    "diff" => repository.Commits.ToDictionary(commit => commit.Message.TrimEnd(),
        commit => repository.Diff.Compare<TreeChanges>(commit.Tree, commit.Parents.FirstOrDefault()?.Tree)
            .Select(change => change.Path).ToArray(), StringComparer.Ordinal),
    "walk" => Walk(repository, args),
    "merge-base" => repository.ObjectDatabase.FindMergeBase(
        repository.Lookup<Commit>(args[2]), repository.Lookup<Commit>(args[3])).Sha,
    _ => throw new ArgumentException("unknown contract")
};
Console.WriteLine(JsonSerializer.Serialize(result));

static int EmptyRepositoryCount(Repository repository)
{
    var status = repository.RetrieveStatus();
    return status.Untracked.Count() + status.Staged.Count();
}


static string[] Walk(Repository repository, string[] args)
{
    var filter = new CommitFilter
    {
        SortBy = Enum.Parse<CommitSortStrategies>(args[2]),
        FirstParentOnly = bool.Parse(args[3]),
        IncludeReachableFrom = args.Length == 6 ? args[5] : args[5..]
    };
    if (args[4] != "")
        filter.ExcludeReachableFrom = args[4];

    return repository.Commits.QueryBy(filter).Select(commit => commit.Sha).ToArray();
}

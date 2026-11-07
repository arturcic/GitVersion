using System.Text.Json;
using LibGit2Sharp;

if (args.Length != 2)
    throw new ArgumentException("usage: RepositoryContracts status|diff REPOSITORY");

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
    _ => throw new ArgumentException("unknown contract")
};
Console.WriteLine(JsonSerializer.Serialize(result));

static int EmptyRepositoryCount(Repository repository)
{
    var status = repository.RetrieveStatus();
    return status.Untracked.Count() + status.Staged.Count();
}

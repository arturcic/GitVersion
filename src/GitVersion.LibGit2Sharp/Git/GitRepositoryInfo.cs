using GitVersion.Extensions;
using GitVersion.Helpers;
using LibGit2Sharp;
using Microsoft.Extensions.Options;

namespace GitVersion.Git;

internal class GitRepositoryInfo : IGitRepositoryInfo
{
    private readonly IOptions<GitVersionOptions> options;
    private GitVersionOptions gitVersionOptions => this.options.Value;

    private readonly Lazy<string?> dynamicGitRepositoryPath;
    private readonly Lazy<string?> dotGitDirectory;
    private readonly Lazy<string?> gitRootPath;
    private readonly Lazy<string?> projectRootDirectory;

    public GitRepositoryInfo(IOptions<GitVersionOptions> options)
    {
        this.options = options.NotNull();

        this.dynamicGitRepositoryPath = new(GetDynamicGitRepositoryPath);
        this.dotGitDirectory = new(GetDotGitDirectory);
        this.gitRootPath = new(GetGitRootPath);
        this.projectRootDirectory = new(GetProjectRootDirectory);
    }

    public string? DynamicGitRepositoryPath => this.dynamicGitRepositoryPath.Value;
    public string? DotGitDirectory => this.dotGitDirectory.Value;
    public string? GitRootPath => this.gitRootPath.Value;
    public string? ProjectRootDirectory => this.projectRootDirectory.Value;

    private string? GetDynamicGitRepositoryPath()
    {
        var repositoryInfo = gitVersionOptions.RepositoryInfo;
        if (repositoryInfo.TargetUrl.IsNullOrWhiteSpace()) return null;

        var targetUrl = repositoryInfo.TargetUrl;
        var clonePath = repositoryInfo.ClonePath;

        var userTemp = clonePath ?? Path.GetTempPath();
        var repositoryName = targetUrl.Split('/', '\\').Last().Replace(".git", string.Empty);
        var possiblePath = PathHelper.Combine(userTemp, repositoryName);

        // Verify that the existing directory is ok for us to use
        if (Directory.Exists(possiblePath) && !GitRepoHasMatchingRemote(possiblePath, targetUrl))
        {
            var i = 1;
            var originalPath = possiblePath;
            bool possiblePathExists;
            do
            {
                possiblePath = $"{originalPath}_{i++}";
                possiblePathExists = Directory.Exists(possiblePath);
            } while (possiblePathExists && !GitRepoHasMatchingRemote(possiblePath, targetUrl));
        }

        var repositoryPath = PathHelper.Combine(possiblePath, ".git");
        return repositoryPath;
    }

    private string? GetDotGitDirectory()
    {
        string gitDirectory = !DynamicGitRepositoryPath.IsNullOrWhiteSpace()
            ? DynamicGitRepositoryPath
            : Repository.Discover(gitVersionOptions.WorkingDirectory);

        gitDirectory = gitDirectory.TrimEnd('/', '\\');
        EnsureGitDirectory(gitDirectory);

        var directoryInfo = Directory.GetParent(gitDirectory) ?? throw new DirectoryNotFoundException();
        return gitDirectory.Contains(PathHelper.Combine(".git", "worktrees"))
            ? Directory.GetParent(directoryInfo.FullName)?.FullName
            : gitDirectory;
    }

    private string GetProjectRootDirectory()
    {
        if (!DynamicGitRepositoryPath.IsNullOrWhiteSpace())
        {
            return gitVersionOptions.WorkingDirectory;
        }

        var gitDirectory = Repository.Discover(gitVersionOptions.WorkingDirectory);

        EnsureGitDirectory(gitDirectory);

        return new Repository(gitDirectory).Info.WorkingDirectory;
    }

    private string? GetGitRootPath()
    {
        var isDynamicRepo = !DynamicGitRepositoryPath.IsNullOrWhiteSpace();
        var rootDirectory = isDynamicRepo ? DotGitDirectory : ProjectRootDirectory;

        return rootDirectory;
    }

    private static bool GitRepoHasMatchingRemote(string possiblePath, string targetUrl)
    {
        try
        {
            var gitRepository = new Repository(possiblePath);
            return gitRepository.Network.Remotes.Any(r => r.Url == targetUrl);
        }
        catch (Exception)
        {
            return false;
        }
    }

    private static void EnsureGitDirectory(string? gitDirectory)
    {
        if (gitDirectory.IsNullOrWhiteSpace())
            throw new DirectoryNotFoundException("Cannot find the .git directory");
    }
}

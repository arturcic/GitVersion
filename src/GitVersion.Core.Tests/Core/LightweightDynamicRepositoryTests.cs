using GitVersion.Configuration;
using GitVersion.Git;
using GitVersion.Helpers;
using GitVersion.Testing.Extensions;
using GitVersion.Testing.Git;

namespace GitVersion.Tests;

[TestFixture]
public class LightweightDynamicRepositoryTests : TestBase
{
    private static readonly AuthenticationInfo Authentication = new();

    private static GitCliExecutor Executor => new(NullLogger<GitCliExecutor>.Instance);
    private static GitCliMutator Mutator => new(NullLogger<GitCliMutator>.Instance, Executor);

    [TestCase(true)]
    [TestCase(false)]
    public void CalculatesGitFlowVersionFromRemoteDevelopWithoutCheckout(bool filterSupported)
    {
        using var source = new EmptyRepositoryFixture();
        source.Repository.MakeATaggedCommit("1.1.0");
        source.Repository.CreateBranch("develop");
        source.Checkout("develop");
        source.Repository.MakeACommit();
        source.Repository.CreateBranch("feature/version-test");
        source.Checkout("feature/version-test");
        source.Repository.MakeACommit();
        Git(source.RepositoryPath, "config", "uploadpack.allowFilter", filterSupported.ToString().ToLowerInvariant());

        using var caller = new EmptyRepositoryFixture();
        var full = Calculate(source, caller, "feature/version-test", lightweight: false);
        var result = Calculate(source, caller, "feature/version-test");

        result.Path.ShouldNotBe(full.Path);
        result.Version.ShouldBe(full.Version);
        result.Version.ShouldStartWith("1.2.0-version-test.");
        Git(result.Path, "rev-parse", "--is-bare-repository").ShouldBe("true");
        Git(result.Path, "for-each-ref", "--format=%(refname)", "refs/heads/develop").ShouldBeEmpty();
        Git(result.Path, "rev-parse", "refs/remotes/origin/develop").ShouldBe(source.Repository.Branches["develop"].ShouldNotBeNull().Tip.Sha);
        Directory.Exists(Path.Combine(result.Path, ".git")).ShouldBeFalse();
        File.Exists(Path.Combine(result.Path, "index")).ShouldBeFalse();
        result.UncommittedChanges.ShouldBe(0);
    }

    [Test]
    public void ReusesCacheAndSwitchesBranchesWithoutCheckout()
    {
        using var source = new EmptyRepositoryFixture();
        source.Repository.MakeATaggedCommit("1.1.0");
        source.Repository.CreateBranch("develop");
        source.Checkout("develop");
        source.Repository.MakeACommit();
        Git(source.RepositoryPath, "config", "uploadpack.allowFilter", "true");
        using var caller = new EmptyRepositoryFixture();

        var first = Calculate(source, caller, "main");
        var second = Calculate(source, caller, "refs/heads/develop");
        var third = Calculate(source, caller, "main");

        second.Path.ShouldBe(first.Path);
        third.Path.ShouldBe(first.Path);
        first.Version.ShouldBe("1.1.0");
        second.Version.ShouldStartWith("1.2.0-alpha.");
        third.Version.ShouldBe(first.Version);
        File.Exists(Path.Combine(first.Path, "index")).ShouldBeFalse();
    }

    [Test]
    public void UpdatesRemoteHistoryWithoutKeepingStaleLocalSourceBranches()
    {
        using var source = new EmptyRepositoryFixture();
        source.Repository.MakeATaggedCommit("1.1.0");
        source.Repository.CreateBranch("develop");
        source.Checkout("develop");
        source.Repository.MakeACommit();
        source.Repository.CreateBranch("feature/version-test");
        using var caller = new EmptyRepositoryFixture();
        Calculate(source, caller, "develop");
        source.Repository.MakeATaggedCommit("2.0.0");
        source.Repository.CreateBranch("feature/new-feature");
        source.Checkout("feature/new-feature");
        source.Repository.MakeACommit();

        var result = Calculate(source, caller, "feature/new-feature");

        result.Version.ShouldStartWith("2.1.0-new-feature.");
        Git(result.Path, "for-each-ref", "--format=%(refname)", "refs/heads/").ShouldBe("refs/heads/feature/new-feature");
        Git(result.Path, "rev-parse", "refs/remotes/origin/develop").ShouldBe(source.Repository.Branches["develop"].ShouldNotBeNull().Tip.Sha);
    }

    [Test]
    public void NoFetchReusesCachedHistoryAndFreshCloneStillDownloadsHistory()
    {
        using var source = new EmptyRepositoryFixture();
        source.Repository.MakeATaggedCommit("1.1.0");
        using var caller = new EmptyRepositoryFixture();
        var first = Calculate(source, caller, "main", noFetch: true);
        source.Repository.MakeATaggedCommit("2.0.0");

        var cached = Calculate(source, caller, "main", noFetch: true);
        var updated = Calculate(source, caller, "main");

        first.Version.ShouldBe("1.1.0");
        cached.Version.ShouldBe(first.Version);
        updated.Version.ShouldBe("2.0.0");
    }

    [Test]
    public void ContextualBranchDoesNotSelectAnotherRemoteCommit()
    {
        using var source = new EmptyRepositoryFixture();
        source.Repository.MakeATaggedCommit("1.1.0");
        source.Repository.CreateBranch("develop");
        source.Checkout("develop");
        source.Repository.MakeATaggedCommit("2.0.0");
        source.Checkout("main");
        using var caller = new EmptyRepositoryFixture();

        var result = Calculate(source, caller, null, contextBranch: "develop");

        Git(result.Path, "symbolic-ref", "HEAD").ShouldBe("refs/heads/main");
        Git(result.Path, "rev-parse", "HEAD").ShouldBe(source.Repository.Head.Tip.Sha);
    }

    [Test]
    public void ExplicitCommitCanUseRetainedRemoteHistory()
    {
        using var source = new EmptyRepositoryFixture();
        source.Repository.MakeATaggedCommit("1.1.0");
        var commit = source.Repository.Head.Tip.Sha;
        source.Repository.MakeATaggedCommit("2.0.0");
        using var caller = new EmptyRepositoryFixture();

        var result = Calculate(source, caller, "main", commitId: commit);

        result.Version.ShouldBe("1.1.0");
        Git(result.Path, "rev-parse", "HEAD").ShouldBe(source.Repository.Head.Tip.Sha);
    }

    [Test]
    public void UnknownRemoteBranchFailsInsteadOfVersioningDefaultHead()
    {
        using var source = new EmptyRepositoryFixture();
        source.Repository.MakeACommit();
        using var caller = new EmptyRepositoryFixture();

        var exception = Should.Throw<WarningException>(() => Calculate(source, caller, "missing"));

        exception.Message.ShouldContain("remote branch 'missing' was not found");
    }

    [Test]
    public void PartialCloneOmitsLargeBlobsAndRetainsTrees()
    {
        using var source = new EmptyRepositoryFixture();
        var bytes = new byte[2 * 1024 * 1024];
        new Random(2921).NextBytes(bytes);
        var path = Path.Combine(source.RepositoryPath, "large.bin");
        File.WriteAllBytes(path, bytes);
        Commands.Stage(source.Repository, path);
        source.Repository.Commit("large blob", Generate.SignatureNow(), Generate.SignatureNow());
        Git(source.RepositoryPath, "config", "uploadpack.allowFilter", "true");
        using var caller = new EmptyRepositoryFixture();
        var clone = Path.Combine(caller.RepositoryPath, "partial.git");

        Mutator.CloneLightweight(new Uri(source.RepositoryPath.TrimEnd('/', '\\')).AbsoluteUri, clone, Authentication);
        Mutator.ValidateLightweightHistory(clone);

        Git(clone, "config", "remote.origin.promisor").ShouldBe("true");
        var missing = Git(clone, "rev-list", "--objects", "--missing=print", "--all");
        missing.ShouldContain("?");
        Git(clone, "ls-tree", "HEAD").ShouldContain("large.bin");
        var objectBytes = Directory.EnumerateFiles(Path.Combine(clone, "objects"), "*", SearchOption.AllDirectories).Sum(file => new FileInfo(file).Length);
        objectBytes.ShouldBeLessThan(bytes.Length / 10);
        File.Exists(Path.Combine(clone, "large.bin")).ShouldBeFalse();
    }

    [Test]
    public void IgnoredPathCalculationUsesTreesWithoutFetchingBlobs()
    {
        using var source = new EmptyRepositoryFixture();
        source.Repository.MakeACommit("A");
        var ignored = source.Repository.MakeACommit("B");
        source.Repository.MakeACommit("C");
        source.Repository.MakeACommit("D");
        Git(source.RepositoryPath, "config", "uploadpack.allowFilter", "true");
        var ignoredPath = Git(source.RepositoryPath, "diff-tree", "--no-commit-id", "--name-only", "-r", ignored.Sha);
        var configuration = TrunkBasedConfigurationBuilder.New
            .WithIgnoreConfiguration(new IgnoreConfiguration { Paths = { ignoredPath } }).Build();
        using var caller = new EmptyRepositoryFixture();

        var full = Calculate(source, caller, "main", lightweight: false, configuration: configuration);
        var partial = Calculate(source, caller, "main", configuration: configuration);

        full.Version.ShouldBe("0.0.3");
        partial.Version.ShouldBe(full.Version);
        Git(partial.Path, "rev-list", "--objects", "--missing=print", "--all").ShouldContain("?");
    }

    [Test]
    public void LongTrackedPathsAreVersionedWithoutCreatingFiles()
    {
        using var source = new EmptyRepositoryFixture();
        File.WriteAllText(Path.Combine(source.RepositoryPath, "payload.txt"), "content");
        Commands.Stage(source.Repository, "*");
        source.Repository.Commit("payload", Generate.SignatureNow(), Generate.SignatureNow());
        var blob = Git(source.RepositoryPath, "rev-parse", "HEAD:payload.txt");
        var longPath = string.Join("/", Enumerable.Repeat(new string('a', 60), 6)) + "/file.txt";
        Git(source.RepositoryPath, "update-index", "--add", "--cacheinfo", $"100644,{blob},{longPath}");
        source.Repository.Commit("long path without worktree checkout", Generate.SignatureNow(), Generate.SignatureNow());
        Git(source.RepositoryPath, "tag", "1.1.0");
        Git(source.RepositoryPath, "config", "uploadpack.allowFilter", "true");
        using var caller = new EmptyRepositoryFixture();

        var result = Calculate(source, caller, "main");

        result.Version.ShouldBe("1.1.0");
        Git(result.Path, "ls-tree", "-r", "HEAD").ShouldContain(longPath);
        Directory.Exists(Path.Combine(result.Path, new string('a', 60))).ShouldBeFalse();
    }

    [Test]
    public void UnsupportedFilteringLogsFullObjectFallbackWithoutCheckout()
    {
        using var source = new EmptyRepositoryFixture();
        source.Repository.MakeACommit();
        Git(source.RepositoryPath, "config", "uploadpack.allowFilter", "false");
        using var caller = new EmptyRepositoryFixture();
        var messages = new List<string>();
        var mutator = new GitCliMutator(new TestLogger<GitCliMutator>(messages.Add), Executor);
        var clone = Path.Combine(caller.RepositoryPath, "fallback.git");

        mutator.CloneLightweight(new Uri(source.RepositoryPath.TrimEnd('/', '\\')).AbsoluteUri, clone, Authentication);

        messages.ShouldContain(message => message.Contains("does not support partial-clone filtering", StringComparison.Ordinal));
        Git(clone, "rev-list", "--objects", "--missing=print", "--all").ShouldNotContain("?");
        Git(clone, "rev-parse", "--is-bare-repository").ShouldBe("true");
        File.Exists(Path.Combine(clone, "index")).ShouldBeFalse();
    }

    [TestCase(401, "Unauthorized")]
    [TestCase(403, "Forbidden")]
    public void PartialCloneDoesNotTreatAuthenticationErrorsAsFilterFallback(int status, string expected)
    {
        var mutator = new GitCliMutator(NullLogger<GitCliMutator>.Instance,
            new MissingHistoryExecutor("", 128, $"fatal: The requested URL returned error: {status}"));

        var error = Should.Throw<InvalidOperationException>(() => mutator.CloneLightweight("https://example.test/repo.git", "unused", Authentication));

        error.Message.ShouldStartWith(expected);
    }

    [Test]
    public void MissingTreeIsDetectedBeforeManagedCalculation()
    {
        using var source = new EmptyRepositoryFixture();
        source.Repository.MakeACommit();
        var tree = Git(source.RepositoryPath, "rev-parse", "HEAD^{tree}");
        File.Delete(Path.Combine(source.RepositoryPath, ".git", "objects", tree[..2], tree[2..]));

        var error = Should.Throw<InvalidOperationException>(() => Mutator.ValidateLightweightHistory(source.RepositoryPath));

        error.Message.ShouldContain("missing commit or tree objects");
    }

    [TestCase("?0123456789012345678901234567890123456789\n", 0)]
    [TestCase("", 128)]
    public void MissingHistoryHasActionableError(string output, int exitCode)
    {
        var mutator = new GitCliMutator(NullLogger<GitCliMutator>.Instance, new MissingHistoryExecutor(output, exitCode));

        var error = Should.Throw<InvalidOperationException>(() => mutator.ValidateLightweightHistory("unused"));

        error.Message.ShouldContain("missing commit or tree objects");
        error.Message.ShouldContain("without --partial-clone");
    }

    private sealed class MissingHistoryExecutor(string output, int exitCode, string error = "") : IGitCliExecutor
    {
        public GitCliResult Execute(string? workingDirectory, IReadOnlyList<string> arguments) => new(exitCode, output, error);
    }

    private static (string Version, string Path, int UncommittedChanges) Calculate(EmptyRepositoryFixture source, EmptyRepositoryFixture caller, string? branch, bool lightweight = true, bool noFetch = false, string? contextBranch = null, string? commitId = null, IGitVersionConfiguration? configuration = null)
    {
        var options = Options.Create(new GitVersionOptions
        {
            WorkingDirectory = caller.RepositoryPath,
            ConfigurationInfo = { OverrideConfiguration = configuration is null ? null : new ConfigurationHelper(configuration).Dictionary },
            RepositoryInfo =
            {
                TargetUrl = new Uri(source.RepositoryPath.TrimEnd('/', '\\')).AbsoluteUri,
                TargetBranch = branch,
                ClonePath = Path.Combine(caller.RepositoryPath, "dynamic"),
                UsePartialClone = lightweight,
                CommitId = commitId
            },
            Settings = { NoCache = true, NoFetch = noFetch }
        });
        var environment = new TestEnvironment();
        if (contextBranch is not null)
        {
            environment.SetEnvironmentVariable("GIT_BRANCH", contextBranch);
        }
        using var provider = (ServiceProvider)ConfigureServices(services =>
        {
            services.AddSingleton(options);
            services.AddSingleton<IEnvironment>(environment);
        });
        provider.DiscoverRepository();
        provider.GetRequiredService<IGitPreparer>().Prepare();
        var variables = provider.GetRequiredService<IGitVersionCalculateTool>().CalculateVersionVariables();
        var repository = provider.GetRequiredService<IGitRepository>();
        return (variables.FullSemVer, repository.Path, repository.UncommittedChangesCount());
    }

    private static string Git(string directory, params string[] arguments)
    {
        var result = Executor.Execute(directory, arguments);
        result.IsSuccess.ShouldBeTrue(result.StandardError);
        return result.StandardOutput.Trim();
    }
}

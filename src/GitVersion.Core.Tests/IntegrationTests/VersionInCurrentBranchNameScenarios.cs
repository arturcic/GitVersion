using GitVersion.Configuration;
using GitVersion.Core.Tests.Helpers;
using LibGit2Sharp;

namespace GitVersion.Core.Tests.IntegrationTests;

[TestFixture]
public class VersionInCurrentBranchNameScenarios : TestBase
{
    [Test]
    public void TakesVersionFromNameOfReleaseBranch()
    {
        using var fixture = new BaseGitFlowRepositoryFixture("1.0.0");
        fixture.BranchTo("release/2.0.0");

        fixture.AssertFullSemver("2.0.0-beta.1+0");
    }

    [Test]
    public void DoesNotTakeVersionFromNameOfNonReleaseBranch()
    {
        using var fixture = new BaseGitFlowRepositoryFixture("1.0.0");
        fixture.BranchTo("feature/upgrade-power-level-to-9000.0.1");

        fixture.AssertFullSemver("1.1.0-upgrade-power-level-to-9000-0-1.1+1");
    }

    [Test]
    public void TakesVersionFromNameOfBranchThatIsReleaseByConfig()
    {
        var configuration = GitFlowConfigurationBuilder.New
            .WithBranch("support", builder => builder.WithIsReleaseBranch(true))
            .Build();

        using var fixture = new BaseGitFlowRepositoryFixture("1.0.0");
        fixture.BranchTo("support/2.0.0");

        fixture.AssertFullSemver("2.0.0-1", configuration);
    }

    [Test]
    public void TakesVersionFromNameOfRemoteReleaseBranchInOrigin()
    {
        using var fixture = new EmptyRepositoryFixture();

        fixture.Repository.MakeCommits(5);

        using var localRepositoryFixture = fixture.CloneRepository();
        fixture.BranchTo("release/2.0.0");
        fixture.MakeACommit();
        Commands.Fetch(localRepositoryFixture.Repository, localRepositoryFixture.Repository.Network.Remotes.First().Name, [], new(), null);

        localRepositoryFixture.Checkout("origin/release/2.0.0");

        localRepositoryFixture.AssertFullSemver("2.0.0-beta.1+1");
    }

    [Test]
    public void DoesNotTakeVersionFromNameOfRemoteReleaseBranchInCustomRemote()
    {
        using var fixture = new EmptyRepositoryFixture();

        fixture.Repository.MakeCommits(5);

        using var localRepositoryFixture = fixture.CloneRepository();
        localRepositoryFixture.Repository.Network.Remotes.Rename("origin", "upstream");
        fixture.BranchTo("release/2.0.0");
        fixture.MakeACommit();
        Commands.Fetch(localRepositoryFixture.Repository, localRepositoryFixture.Repository.Network.Remotes.First().Name, [], new(), null);

        localRepositoryFixture.Checkout("upstream/release/2.0.0");

        localRepositoryFixture.AssertFullSemver("0.0.1-upstream-release-2-0-0.1+6");
    }
}

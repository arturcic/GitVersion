using GitVersion.Core.Tests.Helpers;
using GitVersion.OutputVariables;
using GitVersion.VersionCalculation;
using Microsoft.Extensions.DependencyInjection;

namespace GitVersion.Agents.Tests;

[TestFixture]
public class BuildServerBaseTests : TestBase
{
    private IVariableProvider buildServer;
    private IServiceProvider sp;

    [SetUp]
    public void SetUp()
    {
        this.sp = ConfigureServices(services => services.AddSingleton<ICurrentBuildAgent, BuildAgent>());
        this.buildServer = this.sp.GetRequiredService<IVariableProvider>();
    }

    [Test]
    public void BuildNumberIsFullSemVer()
    {
        var writes = new List<string?>();
        var semanticVersion = new SemanticVersion
        {
            Major = 1,
            Minor = 2,
            Patch = 3,
            PreReleaseTag = "beta1",
            BuildMetaData = new SemanticVersionBuildMetaData("5")
            {
                Sha = "commitSha",
                CommitDate = DateTimeOffset.Parse("2014-03-06 23:59:59Z")
            }
        };

        var configuration = new TestEffectiveConfiguration();

        var variables = this.buildServer.GetVariablesFor(semanticVersion, configuration, null);
        var buildAgent = this.sp.GetRequiredService<ICurrentBuildAgent>();
        buildAgent.WriteIntegration(writes.Add, variables);

        writes[1].ShouldBe("1.2.3-beta.1+5");

        writes = new List<string?>();
        buildAgent.WriteIntegration(writes.Add, variables, false);
        writes.ShouldNotContain(x => x != null && x.StartsWith("Executing GenerateSetVersionMessage for "));
    }

    private class BuildAgent : ICurrentBuildAgent
    {
        public string EnvironmentVariable => throw new NotImplementedException();

        public bool CanApplyToCurrentContext() => throw new NotImplementedException();

        public string GenerateSetVersionMessage(GitVersionVariables variables) => variables.FullSemVer;

        public string[] GenerateSetParameterMessage(string name, string? value) => Array.Empty<string>();
    }
}

using GitVersion.Agents;
using GitVersion.Core.Tests.Helpers;
using GitVersion.OutputVariables;
using Microsoft.Extensions.DependencyInjection;

namespace GitVersion.App.Tests;

[TestFixture]
public class ArgumentParserOnBuildServerTests : TestBase
{
    private IArgumentParser argumentParser;

    [SetUp]
    public void SetUp()
    {
        var sp = ConfigureServices(services =>
        {
            services.AddSingleton<IArgumentParser, ArgumentParser>();
            services.AddSingleton<IGlobbingResolver, GlobbingResolver>();
            services.AddSingleton<ICurrentBuildAgent, BuildAgent>();
        });
        this.argumentParser = sp.GetRequiredService<IArgumentParser>();
    }

    [Test]
    public void EmptyOnFetchDisabledBuildServerMeansNoFetchIsTrue()
    {
        var arguments = this.argumentParser.ParseArguments("");
        arguments.NoFetch.ShouldBe(true);
    }

    private class BuildAgent : ICurrentBuildAgent
    {
        public string EnvironmentVariable => throw new NotImplementedException();

        public bool CanApplyToCurrentContext() => throw new NotImplementedException();

        public string GenerateSetVersionMessage(GitVersionVariables variables) => variables.FullSemVer;

        public string[] GenerateSetParameterMessage(string name, string? value) => Array.Empty<string>();
    }
}

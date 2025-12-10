using System.IO.Abstractions;
using GitVersion.Agents;
using GitVersion.Logging;
using Microsoft.Extensions.DependencyInjection;

namespace GitVersion;

public class GitVersionCommonModule : IGitVersionModule
{
    public void RegisterTypes(IServiceCollection services)
    {
        // Logging is configured externally via Microsoft.Extensions.Logging
        services.AddSingleton<IFileSystem, FileSystem>();
        services.AddSingleton<IEnvironment, Environment>();
        services.AddSingleton<IConsole, ConsoleAdapter>();

        services.AddSingleton<IBuildAgent, LocalBuild>();
        services.AddSingleton<IBuildAgentResolver, BuildAgentResolver>();
        services.AddSingleton(sp => sp.GetRequiredService<IBuildAgentResolver>().Resolve());
    }
}

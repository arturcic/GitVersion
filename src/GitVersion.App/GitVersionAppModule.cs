using GitVersion.FileSystemGlobbing;
using Microsoft.Extensions.DependencyInjection;

namespace GitVersion;

internal class GitVersionAppModule : IGitVersionModule
{
    public void RegisterTypes(IServiceCollection services)
    {
        services.AddSingleton<IGlobbingResolver, GlobbingResolver>();

        services.AddSingleton<IVersionWriter, VersionWriter>();
        services.AddSingleton<IGitVersionExecutor, GitVersionExecutor>();
    }
}

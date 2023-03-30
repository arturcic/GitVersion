using Microsoft.Extensions.DependencyInjection;

namespace GitVersion.Agents;

public class GitVersionBuildAgentsModule : IGitVersionModule
{
    public void RegisterTypes(IServiceCollection services)
    {
        var buildAgents = IGitVersionModule.FindAllDerivedTypes<ICurrentBuildAgent>(Assembly.GetAssembly(GetType()));

        foreach (var buildAgent in buildAgents)
        {
            services.AddSingleton(typeof(IBuildAgent), buildAgent);
        }
    }
}

using GitVersion.Extensions;
using Microsoft.Extensions.Logging;

namespace GitVersion.Agents;

internal class BuildAgentResolver(IEnumerable<IBuildAgent> buildAgents, ILogger<BuildAgentResolver> logger) : IBuildAgentResolver
{
    private readonly ILogger logger = logger.NotNull();

    public ICurrentBuildAgent Resolve() => new Lazy<ICurrentBuildAgent>(ResolveInternal).Value;

    private ICurrentBuildAgent ResolveInternal()
    {
        var instance = (ICurrentBuildAgent)buildAgents.Single(x => x.IsDefault);

        foreach (var buildAgent in buildAgents.Where(x => !x.IsDefault))
        {
            try
            {
                if (!buildAgent.CanApplyToCurrentContext()) continue;
                instance = (ICurrentBuildAgent)buildAgent;
            }
            catch (Exception ex)
            {
                var agentName = buildAgent.GetType().Name;
                this.logger.LogWarning(ex, "Failed to check build agent '{AgentName}'", agentName);
            }
        }

        this.logger.LogInformation("Applicable build agent found: '{AgentName}'", instance.GetType().Name);
        return instance;
    }
}

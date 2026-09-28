using Common.Utilities;

namespace Docs.Tasks;

public sealed class Clean
{
    public void Run(BuildContext context)
    {
        context.Information("Cleaning directories...");

        context.EnsureDirectoryExists(Paths.ArtifactsDocs.Combine("_published"));
    }
}

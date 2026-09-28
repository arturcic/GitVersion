using Docs.Utilities;

namespace Docs.Tasks;

public sealed class GenerateMermaidSources
{
    public void Run(BuildContext context) => context.GenerateMermaidSources(check: false);
}

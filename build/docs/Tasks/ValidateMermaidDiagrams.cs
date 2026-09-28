using Docs.Utilities;

namespace Docs.Tasks;

public sealed class ValidateMermaidDiagrams
{
    public void Run(BuildContext context)
    {
        context.GenerateMermaidSources(check: true);
        context.ValidateMermaidSyntax();
    }
}

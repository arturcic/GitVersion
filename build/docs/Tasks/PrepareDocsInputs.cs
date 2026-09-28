using Docs.Utilities;

namespace Docs.Tasks;

public sealed class PrepareDocsInputs
{
    public void Run(BuildContext context) => context.DocumentationInputs = new DocsInputs(
        context.Environment.WorkingDirectory.FullPath, message => context.Information(message)).Prepare().GetAwaiter().GetResult();
}

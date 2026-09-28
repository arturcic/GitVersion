using Common.Utilities;
using Docs.Utilities;

namespace Docs.Tasks;

public sealed class BuildDocs
{
    public bool ShouldRun(BuildContext context)
    {
        var shouldRun = true;
        shouldRun &= context.ShouldRun(context.DirectoryExists(Paths.Docs), "Wyam documentation directory is missing");

        return shouldRun;
    }

    public void Run(BuildContext context) => VersionedDocs.Build(context);
}

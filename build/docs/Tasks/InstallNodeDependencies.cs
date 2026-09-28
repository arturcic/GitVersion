using Docs.Utilities;

namespace Docs.Tasks;

public sealed class InstallNodeDependencies
{
    public void Run(BuildContext context) => context.InstallNodeDependencies();
}

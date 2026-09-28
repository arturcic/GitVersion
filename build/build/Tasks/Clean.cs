using Common.Utilities;

namespace Build.Tasks;

public sealed class Clean
{
    public void Run(BuildContext context)
    {
        context.Information("Cleaning directories...");

        context.CleanDirectories(Paths.Src + "/**/bin/" + context.MsBuildConfiguration);
        context.CleanDirectories(Paths.Src + "/**/obj");
        context.CleanDirectory(Paths.TestOutput);
        context.CleanDirectory(Paths.Nuget);
        context.CleanDirectory(Paths.Native);
        context.CleanDirectory(Paths.Packages);
        context.CleanDirectory(Paths.Artifacts);
    }
}

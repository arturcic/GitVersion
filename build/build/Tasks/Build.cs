namespace Build.Tasks;

public sealed class Build
{
    public void Run(BuildContext context)
    {
        context.Information("Builds solution...");
        const string sln = "./src/GitVersion.slnx";

        context.DotNetRestore(sln, new DotNetRestoreSettings
        {
            Verbosity = DotNetVerbosity.Minimal,
            Sources = [Constants.NugetOrgUrl],
            MSBuildSettings = context.MsBuildSettings
        });

        context.DotNetBuild(sln, new DotNetBuildSettings
        {
            Verbosity = DotNetVerbosity.Minimal,
            Configuration = context.MsBuildConfiguration,
            NoRestore = true,
            MSBuildSettings = context.MsBuildSettings
        });
    }
}

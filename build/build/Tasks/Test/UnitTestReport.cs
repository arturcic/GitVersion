using Common.Utilities;

namespace Build.Tasks;

[TaskName(nameof(UnitTestReport))]
[TaskDescription("Run the BuildAgents GitHub reporting pilot")]
[TaskArgument(Arguments.TestResults)]
public sealed class UnitTestReport : FrostingTask<BuildContext>
{
    public override void Run(BuildContext context)
    {
        // Only this project references the GitHub reporter during the pilot.
        const string projectName = "GitVersion.BuildAgents.Tests";
        const string framework = "net10.0";
        var projectDirectory = Paths.Src.Combine(projectName);
        var resultsDirectory = context.MakeAbsolute(new DirectoryPath(context.Argument(Arguments.TestResults,
            Paths.TestOutput.Combine($"pilot/{projectName}/{framework}").FullPath)));
        context.EnsureDirectoryExists(resultsDirectory);

        var settings = new DotNetBuildSettings
        {
            Framework = framework,
            Configuration = context.MsBuildConfiguration,
            MSBuildSettings = new()
        };
        // Preserve real source paths for snapshots and failure annotations.
        settings.MSBuildSettings.SetContinuousIntegrationBuild(false);
        context.DotNetBuild(projectDirectory.CombineWithFilePath($"{projectName}.csproj").FullPath, settings);

        var args = TestReporting.AppendArguments(new ProcessArgumentBuilder(), resultsDirectory)
            .Append("--report-gh")
            .Append("--report-gh-annotations on")
            .Append("--report-gh-groups off")
            .Append("--report-gh-step-summary on-failure")
            .Append("--report-gh-step-summary-sections test-results")
            .Append("--report-gh-failure-details on")
            .Append("--report-gh-slow-test-notices off");

        // SDK 10.0.401's dotnet test output omitted annotation commands in the
        // pilot. Direct execution preserves them and Cake fails on a nonzero exit.
        var assembly = projectDirectory.Combine($"bin/{context.MsBuildConfiguration}/{framework}")
            .CombineWithFilePath($"{projectName}.dll");
        context.DotNetExecute(assembly, args);
    }
}

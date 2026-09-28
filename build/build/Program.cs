using Common.Utilities;

Context.Environment.WorkingDirectory = Extensions.GetRootDirectory();
var context = new Build.BuildContext(Context);
var lifetime = new Build.BuildLifetime();

Setup(info =>
{
    InstallTool($"nuget:?package={Common.Utilities.Tools.CodecovUploaderCmd}&version={Common.Utilities.Tools.Versions[Common.Utilities.Tools.CodecovUploaderCmd]}");
    lifetime.Setup(context, info);
});
Teardown(info => lifetime.Teardown(context, info));
TaskSetup(info => context.StartGroup($"Task: {info.Task.Name}"));
TaskTeardown(_ => context.EndGroup());

Task("Default")
    .Description("Shows this output")
    .Does(() => TaskHelp.Show(context, Tasks));

Task(nameof(Build.Tasks.Test))
    .Description("(CI only) Run the tests and publish the results")
    .IsDependentOn(nameof(Build.Tasks.PublishCoverage));

Task(nameof(Build.Tasks.Build))
    .Description("Builds the solution")
    .IsDependentOn(nameof(Build.Tasks.Clean))
    .Does(() => new Build.Tasks.Build().Run(context));

Task(nameof(Build.Tasks.Package))
    .Description("Creates the packages (nuget, chocolatey or tar.gz)")
    .IsDependentOn(nameof(Build.Tasks.PackageChocolatey))
    .IsDependentOn(nameof(Build.Tasks.PackageNuget))
    .IsDependentOn(nameof(Build.Tasks.PackageArchive));

Task(nameof(Build.Tasks.Clean))
    .Description("Cleans build artifacts")
    .Does(() => new Build.Tasks.Clean().Run(context));

Task(nameof(Build.Tasks.CodeFormat))
    .Description("Formats the code")
    .Does(() => new Build.Tasks.CodeFormat().Run(context));

Task(nameof(Build.Tasks.ValidateVersion))
    .Description("Validates built assembly version")
    .IsDependentOn(nameof(Build.Tasks.Build))
    .Does(() => new Build.Tasks.ValidateVersion().Run(context));

Task(nameof(Build.Tasks.BuildPrepare))
    .Description("Builds the solution")
    .IsDependentOn(nameof(Build.Tasks.Clean))
    .Does(() => new Build.Tasks.BuildPrepare().Run(context));

Task(nameof(Build.Tasks.UnitTest))
    .Description("Run the unit tests")
    .IsDependentOn(nameof(Build.Tasks.Build))
    .WithCriteria(() => new Build.Tasks.UnitTest().ShouldRun(context))
    .Does(() => new Build.Tasks.UnitTest().Run(context));

Task(nameof(Build.Tasks.PublishCoverage))
    .Description("Publishes the test coverage")
    .IsDependentOn(nameof(Build.Tasks.UnitTest))
    .WithCriteria(() => new Build.Tasks.PublishCoverage().ShouldRun(context))
    .Does(() => new Build.Tasks.PublishCoverage().Run(context));

Task(nameof(Build.Tasks.PackageChocolatey))
    .Description("Creates the chocolatey packages")
    .IsDependentOn(nameof(Build.Tasks.PackagePrepare))
    .WithCriteria(() => new Build.Tasks.PackageChocolatey().ShouldRun(context))
    .Does(() => new Build.Tasks.PackageChocolatey().Run(context));

Task(nameof(Build.Tasks.PackageArchive))
    .Description("Creates the tar.gz or zip packages")
    .IsDependentOn(nameof(Build.Tasks.PackagePrepare))
    .Does(() => new Build.Tasks.PackageArchive().Run(context));

Task(nameof(Build.Tasks.PackagePrepare))
    .Description("Prepares for packaging")
    .IsDependentOn(nameof(Build.Tasks.ValidateVersion))
    .Does(() => new Build.Tasks.PackagePrepare().Run(context));

Task(nameof(Build.Tasks.PackageNuget))
    .Description("Creates the nuget packages")
    .Does(() => new Build.Tasks.PackageNuget().Run(context));

return BuildRunner.Run(context, () => RunTarget(Argument("target", "Default")));

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

Task("Test")
    .Description("(CI only) Run the tests and publish the results")
    .IsDependentOn("PublishCoverage");

Task("Build")
    .Description("Builds the solution")
    .IsDependentOn("Clean")
    .Does(() => new Build.Tasks.Build().Run(context));

Task("Package")
    .Description("Creates the packages (nuget, chocolatey or tar.gz)")
    .IsDependentOn("PackageChocolatey")
    .IsDependentOn("PackageNuget")
    .IsDependentOn("PackageArchive");

Task("Clean")
    .Description("Cleans build artifacts")
    .Does(() => new Build.Tasks.Clean().Run(context));

Task("CodeFormat")
    .Description("Formats the code")
    .Does(() => new Build.Tasks.CodeFormat().Run(context));

Task("ValidateVersion")
    .Description("Validates built assembly version")
    .IsDependentOn("Build")
    .Does(() => new Build.Tasks.ValidateVersion().Run(context));

Task("BuildPrepare")
    .Description("Builds the solution")
    .IsDependentOn("Clean")
    .Does(() => new Build.Tasks.BuildPrepare().Run(context));

Task("UnitTest")
    .Description("Run the unit tests")
    .IsDependentOn("Build")
    .WithCriteria(() => new Build.Tasks.UnitTest().ShouldRun(context))
    .Does(() => new Build.Tasks.UnitTest().Run(context));

Task("PublishCoverage")
    .Description("Publishes the test coverage")
    .IsDependentOn("UnitTest")
    .WithCriteria(() => new Build.Tasks.PublishCoverage().ShouldRun(context))
    .Does(() => new Build.Tasks.PublishCoverage().Run(context));

Task("PackageChocolatey")
    .Description("Creates the chocolatey packages")
    .IsDependentOn("PackagePrepare")
    .WithCriteria(() => new Build.Tasks.PackageChocolatey().ShouldRun(context))
    .Does(() => new Build.Tasks.PackageChocolatey().Run(context));

Task("PackageArchive")
    .Description("Creates the tar.gz or zip packages")
    .IsDependentOn("PackagePrepare")
    .Does(() => new Build.Tasks.PackageArchive().Run(context));

Task("PackagePrepare")
    .Description("Prepares for packaging")
    .IsDependentOn("ValidateVersion")
    .Does(() => new Build.Tasks.PackagePrepare().Run(context));

Task("PackageNuget")
    .Description("Creates the nuget packages")
    .Does(() => new Build.Tasks.PackageNuget().Run(context));

return BuildRunner.Run(context, () => RunTarget(Argument("target", "Default")));

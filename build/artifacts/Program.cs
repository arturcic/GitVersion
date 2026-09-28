using Common.Utilities;

Context.Environment.WorkingDirectory = Extensions.GetRootDirectory();
var context = new Artifacts.BuildContext(Context);
var lifetime = new Artifacts.BuildLifetime();

Setup(info => lifetime.Setup(context, info));
Teardown(info => lifetime.Teardown(context, info));
TaskSetup(info => context.StartGroup($"Task: {info.Task.Name}"));
TaskTeardown(_ => context.EndGroup());

Task("Default")
    .Description("Shows this output")
    .Does(() => TaskHelp.Show(context, Tasks));

Task("ArtifactsExecutableTest")
    .Description("Tests the cmdline and portable packages on windows")
    .WithCriteria(() => new Artifacts.Tasks.ArtifactsExecutableTest().ShouldRun(context))
    .Does(() => new Artifacts.Tasks.ArtifactsExecutableTest().Run(context));

Task("ArtifactsDotnetToolTest")
    .Description("Tests the dotnet global tool in docker container")
    .IsDependentOn("ArtifactsPrepare")
    .WithCriteria(() => new Artifacts.Tasks.ArtifactsDotnetToolTest().ShouldRun(context))
    .Does(() => new Artifacts.Tasks.ArtifactsDotnetToolTest().Run(context));

Task("ArtifactsTest")
    .Description("Tests packages in docker container")
    .IsDependentOn("ArtifactsNativeTest")
    .IsDependentOn("ArtifactsDotnetToolTest")
    .IsDependentOn("ArtifactsMsBuildCoreTest")
    .WithCriteria(() => new Artifacts.Tasks.ArtifactsTest().ShouldRun(context));

Task("ArtifactsMsBuildCoreTest")
    .Description("Tests the msbuild package in docker container")
    .IsDependentOn("ArtifactsPrepare")
    .WithCriteria(() => new Artifacts.Tasks.ArtifactsMsBuildCoreTest().ShouldRun(context))
    .Does(() => new Artifacts.Tasks.ArtifactsMsBuildCoreTest().Run(context));

Task("ArtifactsNativeTest")
    .Description("Tests the native executables in docker container")
    .IsDependentOn("ArtifactsPrepare")
    .WithCriteria(() => new Artifacts.Tasks.ArtifactsNativeTest().ShouldRun(context))
    .Does(() => new Artifacts.Tasks.ArtifactsNativeTest().Run(context));

Task("ArtifactsPrepare")
    .Description("Pulls the docker images needed for testing the artifacts")
    .WithCriteria(() => new Artifacts.Tasks.ArtifactsPrepare().ShouldRun(context))
    .Does(() => new Artifacts.Tasks.ArtifactsPrepare().Run(context));

Task("ArtifactsMsBuildFullTest")
    .Description("Tests the msbuild package on windows")
    .WithCriteria(() => new Artifacts.Tasks.ArtifactsMsBuildFullTest().ShouldRun(context))
    .Does(() => new Artifacts.Tasks.ArtifactsMsBuildFullTest().Run(context));

return BuildRunner.Run(context, () => RunTarget(Argument("target", "Default")));

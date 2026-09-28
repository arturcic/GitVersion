using Common.Utilities;

Context.Environment.WorkingDirectory = Extensions.GetRootDirectory();
var context = new Publish.BuildContext(Context);
var lifetime = new Publish.BuildLifetime();

Setup(info => lifetime.Setup(context, info));
Teardown(info => lifetime.Teardown(context, info));
TaskSetup(info => context.StartGroup($"Task: {info.Task.Name}"));
TaskTeardown(_ => context.EndGroup());

Task("Default")
    .Description("Shows this output")
    .Does(() => TaskHelp.Show(context, Tasks));

Task("PublishChocolatey")
    .Description("Publish chocolatey packages")
    .IsDependentOn("PublishChocolateyInternal");

Task("PublishChocolateyInternal")
    .Description("Publish chocolatey packages")
    .WithCriteria(() => new Publish.Tasks.PublishChocolateyInternal().ShouldRun(context))
    .Does(() => new Publish.Tasks.PublishChocolateyInternal().RunAsync(context));

Task("PublishNuget")
    .Description("Publish nuget packages")
    .IsDependentOn("PublishNugetInternal");

Task("PublishNugetInternal")
    .Description("Publish nuget packages")
    .WithCriteria(() => new Publish.Tasks.PublishNugetInternal().ShouldRun(context))
    .Does(() => new Publish.Tasks.PublishNugetInternal().RunAsync(context));

return BuildRunner.Run(context, () => RunTarget(Argument("target", "Default")));

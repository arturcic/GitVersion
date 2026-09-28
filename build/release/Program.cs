using Common.Utilities;

Context.Environment.WorkingDirectory = Extensions.GetRootDirectory();
var context = new Release.BuildContext(Context);
var lifetime = new Release.BuildLifetime();

Setup(info =>
{
    foreach (var tool in ToolManifest.GetToolUris())
    {
        InstallTool(tool);
    }
    lifetime.Setup(context, info);
});
Teardown(info => lifetime.Teardown(context, info));
TaskSetup(info => context.StartGroup($"Task: {info.Task.Name}"));
TaskTeardown(_ => context.EndGroup());

Task("Default")
    .Description("Shows this output")
    .Does(() => TaskHelp.Show(context, Tasks));

Task("PublishRelease");

Task("PublishReleaseInternal")
    .Description("Publish release")
    .WithCriteria(() => new Release.Tasks.PublishReleaseInternal().ShouldRun(context))
    .Does(() => new Release.Tasks.PublishReleaseInternal().Run(context));

return BuildRunner.Run(context, () => RunTarget(Argument("target", "Default")));

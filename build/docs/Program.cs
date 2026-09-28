using Common.Utilities;

Context.Environment.WorkingDirectory = Extensions.GetRootDirectory();
var context = new Docs.BuildContext(Context);
var lifetime = new Docs.BuildLifetime();

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

Task(nameof(Docs.Tasks.GenerateMermaidSources))
    .Description("Generates Mermaid documentation sources from integration tests")
    .Does(() => new Docs.Tasks.GenerateMermaidSources().Run(context));

Task(nameof(Docs.Tasks.BuildDocs))
    .Description("Builds the docs to local path")
    .IsDependentOn(nameof(Docs.Tasks.Clean))
    .IsDependentOn(nameof(Docs.Tasks.PrepareDocsInputs))
    .IsDependentOn(nameof(Docs.Tasks.ValidateMermaidDiagrams))
    .WithCriteria(() => new Docs.Tasks.BuildDocs().ShouldRun(context))
    .Does(() => new Docs.Tasks.BuildDocs().Run(context));

Task(nameof(Docs.Tasks.InstallNodeDependencies))
    .Description("Installs the pinned Node.js dependencies used by the documentation build")
    .Does(() => new Docs.Tasks.InstallNodeDependencies().Run(context));

Task(nameof(Docs.Tasks.PrepareDocsInputs))
    .Description("Resolve documentation trains and cache their release inputs")
    .Does(() => new Docs.Tasks.PrepareDocsInputs().Run(context));

Task(nameof(Docs.Tasks.GenerateSchemas))
    .Description("Generate schemas")
    .Does(() => new Docs.Tasks.GenerateSchemas().Run(context));

Task(nameof(Docs.Tasks.PreviewDocs))
    .Description("Run a local server with docs in preview")
    .IsDependentOn(nameof(Docs.Tasks.BuildDocs))
    .WithCriteria(() => new Docs.Tasks.PreviewDocs().ShouldRun(context))
    .Does(() => new Docs.Tasks.PreviewDocs().Run(context));

Task(nameof(Docs.Tasks.Clean))
    .Description("Cleans the temporary publish location")
    .Does(() => new Docs.Tasks.Clean().Run(context));

Task(nameof(Docs.Tasks.ValidateMermaidDiagrams))
    .Description("Verifies generated Mermaid sources and validates their syntax")
    .IsDependentOn(nameof(Docs.Tasks.InstallNodeDependencies))
    .Does(() => new Docs.Tasks.ValidateMermaidDiagrams().Run(context));

Task(nameof(Docs.Tasks.PublishDocs))
    .Description("Published the docs changes to docs specific branch")
    .IsDependentOn(nameof(Docs.Tasks.PublishDocsInternal))
    .WithCriteria(() => new Docs.Tasks.PublishDocs().ShouldRun(context));

Task(nameof(Docs.Tasks.PublishDocsInternal))
    .Description("Published the docs changes to docs specific branch")
    .IsDependentOn(nameof(Docs.Tasks.BuildDocs))
    .WithCriteria(() => new Docs.Tasks.PublishDocsInternal().ShouldRun(context))
    .Does(() => new Docs.Tasks.PublishDocsInternal().Run(context));

return BuildRunner.Run(context, () => RunTarget(Argument("target", "Default")));

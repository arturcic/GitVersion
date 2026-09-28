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

Task("GenerateMermaidSources")
    .Description("Generates Mermaid documentation sources from integration tests")
    .Does(() => new Docs.Tasks.GenerateMermaidSources().Run(context));

Task("BuildDocs")
    .Description("Builds the docs to local path")
    .IsDependentOn("Clean")
    .IsDependentOn("PrepareDocsInputs")
    .IsDependentOn("ValidateMermaidDiagrams")
    .WithCriteria(() => new Docs.Tasks.BuildDocs().ShouldRun(context))
    .Does(() => new Docs.Tasks.BuildDocs().Run(context));

Task("InstallNodeDependencies")
    .Description("Installs the pinned Node.js dependencies used by the documentation build")
    .Does(() => new Docs.Tasks.InstallNodeDependencies().Run(context));

Task("PrepareDocsInputs")
    .Description("Resolve documentation trains and cache their release inputs")
    .Does(() => new Docs.Tasks.PrepareDocsInputs().Run(context));

Task("GenerateSchemas")
    .Description("Generate schemas")
    .Does(() => new Docs.Tasks.GenerateSchemas().Run(context));

Task("PreviewDocs")
    .Description("Run a local server with docs in preview")
    .IsDependentOn("BuildDocs")
    .WithCriteria(() => new Docs.Tasks.PreviewDocs().ShouldRun(context))
    .Does(() => new Docs.Tasks.PreviewDocs().Run(context));

Task("Clean")
    .Description("Cleans the temporary publish location")
    .Does(() => new Docs.Tasks.Clean().Run(context));

Task("ValidateMermaidDiagrams")
    .Description("Verifies generated Mermaid sources and validates their syntax")
    .IsDependentOn("InstallNodeDependencies")
    .Does(() => new Docs.Tasks.ValidateMermaidDiagrams().Run(context));

Task("PublishDocs")
    .Description("Published the docs changes to docs specific branch")
    .IsDependentOn("PublishDocsInternal")
    .WithCriteria(() => new Docs.Tasks.PublishDocs().ShouldRun(context));

Task("PublishDocsInternal")
    .Description("Published the docs changes to docs specific branch")
    .IsDependentOn("BuildDocs")
    .WithCriteria(() => new Docs.Tasks.PublishDocsInternal().ShouldRun(context))
    .Does(() => new Docs.Tasks.PublishDocsInternal().Run(context));

return BuildRunner.Run(context, () => RunTarget(Argument("target", "Default")));

using Common.Utilities;

Context.Environment.WorkingDirectory = Extensions.GetRootDirectory();
var context = new Docker.BuildContext(Context);
var lifetime = new Docker.BuildLifetime();

Setup(info => lifetime.Setup(context, info));
Teardown(info => lifetime.Teardown(context, info));
TaskSetup(info => context.StartGroup($"Task: {info.Task.Name}"));
TaskTeardown(_ => context.EndGroup());

Task("Default")
    .Description("Shows this output")
    .Does(() => TaskHelp.Show(context, Tasks));

Task("DockerTest")
    .Description("Test the docker images containing the GitVersion Tool")
    .IsDependentOn("DockerBuild")
    .WithCriteria(() => new Docker.Tasks.DockerTest().ShouldRun(context))
    .Does(() => new Docker.Tasks.DockerTest().Run(context));

Task("DockerManifest")
    .Description("Publish the docker manifest containing the images for amd64 and arm64")
    .IsDependentOn("DockerManifestInternal")
    .WithCriteria(() => new Docker.Tasks.DockerManifest().ShouldRun(context));

Task("DockerManifestInternal")
    .Description("Publish the docker manifest containing the images for amd64 and arm64")
    .WithCriteria(() => new Docker.Tasks.DockerManifestInternal().ShouldRun(context))
    .Does(() => new Docker.Tasks.DockerManifestInternal().Run(context));

Task("DockerBuild")
    .Description("Build the docker images containing the GitVersion Tool")
    .WithCriteria(() => new Docker.Tasks.DockerBuild().ShouldRun(context))
    .Does(() => new Docker.Tasks.DockerBuild().Run(context));

Task("DockerHubReadmePublish")
    .Description("Publish the DockerHub updated README.md")
    .IsDependentOn("DockerHubReadmePublishInternal");

Task("DockerHubReadmePublishInternal")
    .Description("Publish the DockerHub updated README.md")
    .WithCriteria(() => new Docker.Tasks.DockerHubReadmePublishInternal().ShouldRun(context))
    .Does(() => new Docker.Tasks.DockerHubReadmePublishInternal().RunAsync(context));

Task("DockerPublish")
    .Description("Publish the docker images containing the GitVersion Tool")
    .IsDependentOn("DockerPublishInternal")
    .WithCriteria(() => new Docker.Tasks.DockerPublish().ShouldRun(context));

Task("DockerPublishInternal")
    .Description("Publish the docker images containing the GitVersion Tool")
    .IsDependentOn("DockerTest")
    .WithCriteria(() => new Docker.Tasks.DockerPublishInternal().ShouldRun(context))
    .Does(() => new Docker.Tasks.DockerPublishInternal().Run(context));

return BuildRunner.Run(context, () => RunTarget(Argument("target", "Default")));

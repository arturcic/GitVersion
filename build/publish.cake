singleStageRun = !IsEnabled(Context, "ENABLED_MULTI_STAGE_BUILD", false);

#region Publish
Task("Publish-AppVeyor")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.IsRunningOnWindows,  "Publish-AppVeyor works only on Windows agents.")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.IsRunningOnAppVeyor, "Publish-AppVeyor works only on AppVeyor.")
    .IsDependentOnWhen("UnitTest", singleStageRun)
    .IsDependentOnWhen("Pack", singleStageRun)
    .Does<BuildParameters>((parameters) =>
{
    foreach(var artifact in parameters.Artifacts.All)
    {
        if (FileExists(artifact.ArtifactPath)) { AppVeyor.UploadArtifact(artifact.ArtifactPath); }
    }

    foreach(var package in parameters.Packages.All)
    {
        if (FileExists(package.PackagePath)) { AppVeyor.UploadArtifact(package.PackagePath); }
    }
})
.OnError(exception =>
{
    Information("Publish-AppVeyor Task failed, but continuing with next Task...");
    Error(exception.Dump());
    publishingError = true;
});

Task("Publish-AzurePipeline")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.IsRunningOnWindows, "Publish-AzurePipeline works only on Windows agents.")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.IsReleasingCI,      "Publish-AzurePipeline works only on Releasing CI.")
    .WithCriteria<BuildParameters>((context, parameters) => !parameters.IsPullRequest,     "Publish-AzurePipeline works only for non-PR commits.")
    .IsDependentOnWhen("UnitTest", singleStageRun)
    .IsDependentOnWhen("Pack", singleStageRun)
    .Does<BuildParameters>((parameters) =>
{
    foreach(var artifact in parameters.Artifacts.All)
    {
        if (FileExists(artifact.ArtifactPath)) { TFBuild.Commands.UploadArtifact("", artifact.ArtifactPath, "artifacts"); }
    }
    foreach(var package in parameters.Packages.All)
    {
        if (FileExists(package.PackagePath)) { TFBuild.Commands.UploadArtifact("", package.PackagePath, "packages"); }
    }
})
.OnError(exception =>
{
    Information("Publish-AzurePipeline Task failed, but continuing with next Task...");
    Error(exception.Dump());
    publishingError = true;
});

Task("Publish-Gem-Internal")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.EnabledPublishGem,  "Publish-Gem was disabled.")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.IsRunningOnWindows, "Publish-Gem works only on Windows agents.")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.IsReleasingCI,      "Publish-Gem works only on Releasing CI.")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.IsStableRelease() || parameters.IsPreRelease(), "Publish-Gem works only for releases.")
    .IsDependentOnWhen("Pack-Gem", singleStageRun)
    .Does<BuildParameters>((parameters) =>
{
    var apiKey = parameters.Credentials.RubyGem.ApiKey;
    if(string.IsNullOrEmpty(apiKey)) {
        throw new InvalidOperationException("Could not resolve Ruby Gem Api key.");
    }

    SetRubyGemPushApiKey(apiKey);

    var toolPath = Context.FindToolInPath(IsRunningOnWindows() ? "gem.cmd" : "gem");
    Information("Publish-Gem-Internal done");
    // GemPush(parameters.Paths.Files.GemOutputFilePath, new Cake.Gem.Push.GemPushSettings()
    // {
    //     ToolPath = toolPath,
    // });
})
.OnError(exception =>
{
    Information("Publish-Gem Task failed, but continuing with next Task...");
    Error(exception.Dump());
    publishingError = true;
});

Task("Publish-NuGet-Internal")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.EnabledPublishNuget, "Publish-NuGet was disabled.")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.IsRunningOnWindows,  "Publish-NuGet works only on Windows agents.")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.IsReleasingCI,       "Publish-NuGet works only on Releasing CI.")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.IsStableRelease() || parameters.IsPreRelease(), "Publish-NuGet works only for releases.")
    .IsDependentOnWhen("Pack-NuGet", singleStageRun)
    .Does<BuildParameters>((parameters) =>
{
    var apiKey = parameters.Credentials.Nuget.ApiKey;
    if(string.IsNullOrEmpty(apiKey)) {
        throw new InvalidOperationException("Could not resolve NuGet API key.");
    }

    var apiUrl = parameters.Credentials.Nuget.ApiUrl;
    if(string.IsNullOrEmpty(apiUrl)) {
        throw new InvalidOperationException("Could not resolve NuGet API url.");
    }

    foreach(var package in parameters.Packages.Nuget)
    {
        if (FileExists(package.PackagePath))
        {
            // Push the package.
            Information("Publish-NuGet-Internal done");
            // NuGetPush(package.PackagePath, new NuGetPushSettings
            // {
            //     ApiKey = apiKey,
            //     Source = apiUrl
            // });
        }
    }
})
.OnError(exception =>
{
    Information("Publish-NuGet Task failed, but continuing with next Task...");
    Error(exception.Dump());
    publishingError = true;
});

Task("Publish-Chocolatey-Internal")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.EnabledPublishChocolatey, "Publish-Chocolatey was disabled.")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.IsRunningOnWindows,       "Publish-Chocolatey works only on Windows agents.")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.IsReleasingCI,            "Publish-Chocolatey works only on Releasing CI.")
    .WithCriteria<BuildParameters>((context, parameters) => parameters.IsStableRelease() || parameters.IsPreRelease(), "Publish-Chocolatey works only for releases.")
    .IsDependentOnWhen("Pack-Chocolatey", singleStageRun)
    .Does<BuildParameters>((parameters) =>
{
    var apiKey = parameters.Credentials.Chocolatey.ApiKey;
    if(string.IsNullOrEmpty(apiKey)) {
        throw new InvalidOperationException("Could not resolve Chocolatey API key.");
    }

    var apiUrl = parameters.Credentials.Chocolatey.ApiUrl;
    if(string.IsNullOrEmpty(apiUrl)) {
        throw new InvalidOperationException("Could not resolve Chocolatey API url.");
    }

    foreach(var package in parameters.Packages.Chocolatey)
    {
        if (FileExists(package.PackagePath))
        {
            // Push the package.
            Information("Publish-Chocolatey-Internal done");
            // ChocolateyPush(package.PackagePath, new ChocolateyPushSettings
            // {
            //     ApiKey = apiKey,
            //     Source = apiUrl,
            //     Force = true
            // });
        }
    }
})
.OnError(exception =>
{
    Information("Publish-Chocolatey Task failed, but continuing with next Task...");
    Error(exception.Dump());
    publishingError = true;
});
#endregion

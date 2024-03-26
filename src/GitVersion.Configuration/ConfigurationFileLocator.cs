using GitVersion.Extensions;
using GitVersion.Helpers;
using GitVersion.Logging;
using Microsoft.Extensions.Options;

namespace GitVersion.Configuration;

internal class ConfigurationFileLocator(
    IFileSystem fileSystem,
    ILog log,
    IOptions<GitVersionOptions> options)
    : IConfigurationFileLocator
{
    public const string DefaultFileName = "GitVersion.yml";
    public const string DefaultAlternativeFileName = "GitVersion.yaml";

    private readonly IFileSystem fileSystem = fileSystem.NotNull();
    private readonly ILog log = log.NotNull();
    private readonly IOptions<GitVersionOptions> options = options.NotNull();

    private string? ConfigurationFile => options.Value.ConfigurationInfo.ConfigurationFile;

    public bool TryGetConfigurationFile(string? workingDirectory, string? projectRootDirectory, out string? configFilePath)
    {
        configFilePath = GetConfigurationFile(workingDirectory) ?? GetConfigurationFile(projectRootDirectory);
        return configFilePath != null;
    }

    public void Verify(string? workingDirectory, string? projectRootDirectory)
    {
        if (Path.IsPathRooted(this.ConfigurationFile)) return;
        if (PathHelper.Equal(workingDirectory, projectRootDirectory)) return;
        WarnAboutAmbiguousConfigFileSelection(workingDirectory, projectRootDirectory);
    }

    public string? GetConfigurationFile(string? directory)
    {
        if (directory is null) return null;
        string?[] candidates = [this.ConfigurationFile, DefaultFileName, DefaultAlternativeFileName];
        var candidatePaths =
            from candidate in candidates
            where !candidate.IsNullOrWhiteSpace()
            select PathHelper.Combine(directory, candidate);

        foreach (var candidatePath in candidatePaths)
        {
            this.log.Debug($"Trying to find configuration file at '{candidatePath}'");
            if (fileSystem.Exists(candidatePath))
            {
                this.log.Info($"Found configuration file at '{candidatePath}'");
                return candidatePath;
            }
            this.log.Debug($"Configuration file not found at '{candidatePath}'");
        }

        return null;
    }

    private void WarnAboutAmbiguousConfigFileSelection(string? workingDirectory, string? projectRootDirectory)
    {
        var workingConfigFile = GetConfigurationFile(workingDirectory);
        var projectRootConfigFile = GetConfigurationFile(projectRootDirectory);

        var hasConfigInWorkingDirectory = workingConfigFile != null;
        var hasConfigInProjectRootDirectory = projectRootConfigFile != null;

        if (hasConfigInProjectRootDirectory && hasConfigInWorkingDirectory)
        {
            throw new WarningException($"Ambiguous configuration file selection from '{workingConfigFile}' and '{projectRootConfigFile}'");
        }

        if (!hasConfigInProjectRootDirectory && !hasConfigInWorkingDirectory)
        {
            if (this.ConfigurationFile is not (DefaultFileName or DefaultAlternativeFileName))
            {
                workingConfigFile = PathHelper.Combine(workingDirectory, this.ConfigurationFile);
                projectRootConfigFile = PathHelper.Combine(projectRootDirectory, this.ConfigurationFile);
                throw new WarningException($"The configuration file was not found at '{workingConfigFile}' or '{projectRootConfigFile}'");
            }
        }
    }
}

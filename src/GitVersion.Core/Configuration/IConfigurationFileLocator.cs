namespace GitVersion.Configuration;

public interface IConfigurationFileLocator
{
    bool TryGetConfigurationFile(string? workingDirectory, string? projectRootDirectory, out string? configFilePath);
    void Verify(string? workingDirectory, string? projectRootDirectory);
    string? GetConfigurationFile(string? directory);
}

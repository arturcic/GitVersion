using GitVersion.Extensions;
using GitVersion.Helpers;

namespace GitVersion.Configuration.Tests.Configuration;

public static class Extensions
{
    public static IDisposable<string> SetupConfigFile(this IFileSystem fileSystem, string? path = null, string fileName = ConfigurationFileLocator.DefaultFileName, string text = "")
    {
        if (path.IsNullOrEmpty())
        {
            path = PathHelper.GetRepositoryTempPath();
        }

        var fullPath = PathHelper.Combine(path, fileName);
        var directory = fileSystem.Directory.CreateDirectory(Path.GetDirectoryName(fullPath)!);
        fileSystem.File.WriteAllText(fullPath, text);

        return Disposable.Create(fullPath, () =>
        {
            fileSystem.File.Delete(fullPath);
            directory.Delete(true);
        });
    }
}

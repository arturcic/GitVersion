using GitVersion.Extensions;
using GitVersion.Helpers;

namespace GitVersion.Logging;

internal class FileAppender : ILogAppender
{
    private readonly IFileSystem fileSystem;
    private readonly string filePath;

    public FileAppender(IFileSystem fileSystem, string filePath)
    {
        this.fileSystem = fileSystem.NotNull();
        this.filePath = filePath;

        var directory = this.fileSystem.Path.GetDirectoryName(filePath);
        if (!this.fileSystem.Directory.Exists(directory))
        {
            this.fileSystem.Directory.CreateDirectory(directory!);
        }

        using var file = this.fileSystem.File.Create(filePath);
    }

    public void WriteTo(LogLevel level, string message)
    {
        try
        {
            var contents = $"{DateTime.Now:yyyy-MM-dd HH:mm:ss}\t\t{message}{PathHelper.NewLine}";
            this.fileSystem.File.AppendAllText(this.filePath, contents);
        }
        catch
        {
            //
        }
    }
}

using GitVersion.Helpers;

namespace GitVersion;

internal class FileSystem : IFileSystem
{
    public void FileCopy(string from, string to, bool overwrite) => File.Copy(from, to, overwrite);

    public void FileMove(string from, string to) => File.Move(from, to);

    public bool FileExists(string file) => File.Exists(file);

    public void FileDelete(string path) => File.Delete(path);

    public string FileReadAllText(string path) => File.ReadAllText(path);

    public void FileWriteAllText(string? file, string fileContents)
    {
        // Opinionated decision to use UTF8 with BOM when creating new files or when the existing
        // encoding was not easily detected due to the file not having an encoding preamble.
        var encoding = EncodingHelper.DetectEncoding(file) ?? Encoding.UTF8;
        FileWriteAllText(file, fileContents, encoding);
    }

    public void FileWriteAllText(string? file, string fileContents, Encoding encoding)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(file);

        File.WriteAllText(file, fileContents, encoding);
    }

    public Stream FileOpenWrite(string path) => File.OpenWrite(path);

    public Stream FileOpenRead(string path) => File.OpenRead(path);

    public void DirectoryCreateDirectory(string path) => Directory.CreateDirectory(path);

    public bool DirectoryExists(string path) => Directory.Exists(path);

    public string[] DirectoryGetFiles(string path) => Directory.GetFiles(path);

    public string[] DirectoryGetDirectories(string path) => Directory.GetDirectories(path);

    public IEnumerable<string> DirectoryEnumerateFiles(string? directory, string searchPattern, SearchOption searchOption)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(directory);

        return Directory.EnumerateFiles(directory, searchPattern, searchOption);
    }

    public long FileGetLastWriteTime(string path) => File.GetLastWriteTime(path).Ticks;

    public long GetLastDirectoryWrite(string path) => new DirectoryInfo(path)
        .GetDirectories("*.*", SearchOption.AllDirectories)
        .Select(d => d.LastWriteTimeUtc)
        .DefaultIfEmpty()
        .Max()
        .Ticks;
}

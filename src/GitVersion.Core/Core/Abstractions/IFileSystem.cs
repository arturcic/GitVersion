namespace GitVersion;

public interface IFileSystem
{
    void FileCopy(string from, string to, bool overwrite);
    void FileMove(string from, string to);
    bool FileExists(string file);
    void FileDelete(string path);
    string FileReadAllText(string path);
    void FileWriteAllText(string? file, string fileContents);
    void FileWriteAllText(string? file, string fileContents, Encoding encoding);
    Stream FileOpenWrite(string path);
    Stream FileOpenRead(string path);
    void DirectoryCreateDirectory(string path);
    bool DirectoryExists(string path);
    string[] DirectoryGetFiles(string path);
    string[] DirectoryGetDirectories(string path);
    IEnumerable<string> DirectoryEnumerateFiles(string? directory, string searchPattern, SearchOption searchOption);
    long GetLastDirectoryWrite(string path);
    long FileGetLastWriteTime(string path);
}

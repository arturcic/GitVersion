using System.Text.Json;

namespace Common.Utilities;

public static class ToolManifest
{
    public static IEnumerable<string> GetToolUris()
    {
        var manifest = Extensions.GetRootDirectory().CombineWithFilePath(".config/dotnet-tools.json");
        using var document = JsonDocument.Parse(File.ReadAllText(manifest.FullPath));
        foreach (var tool in document.RootElement.GetProperty("tools").EnumerateObject())
        {
            var version = tool.Value.GetProperty("version").GetString();
            yield return $"dotnet:?package={Uri.EscapeDataString(tool.Name)}&version={Uri.EscapeDataString(version!)}";
        }
    }
}

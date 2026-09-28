using Common.Utilities;

namespace Docs.Tasks;

public sealed class GenerateSchemas
{
    public void Run(BuildContext context)
    {
        ArgumentNullException.ThrowIfNull(context.Version);
        var schemaTool = context.GetSchemaDotnetToolLocation();
        var gitVersion = context.Version.GitVersion;
        var version = $"{gitVersion.Major}.{gitVersion.Minor}";
        var schemaTargetDir = context.MakeAbsolute(Paths.Root.Combine("schemas"));
        context.EnsureDirectoryExists(schemaTargetDir);
        context.Information($"Schema tool: {schemaTool}");
        context.Information($"Schema target dir: {schemaTargetDir}");
        context.Information($"Schema version: {version}");
        context.DotNetExecute(schemaTool, $"--Version {version} --OutputDirectory {schemaTargetDir}");
    }
}

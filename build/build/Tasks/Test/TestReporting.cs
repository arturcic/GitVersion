namespace Build.Tasks;

internal static class TestReporting
{
    internal static ProcessArgumentBuilder AppendArguments(ProcessArgumentBuilder args, DirectoryPath resultsDirectory) => args
        .Append("--report-spekt-junit")
        .Append("--report-spekt-junit-filename").AppendQuoted(resultsDirectory.CombineWithFilePath("results.xml").FullPath)
        .Append("--results-directory").AppendQuoted(resultsDirectory.FullPath)
        .Append("--coverlet")
        .Append("--coverlet-output-format").AppendQuoted("cobertura")
        .Append("--coverlet-exclude").AppendQuoted("[GitVersion*.Tests]*")
        .Append("--coverlet-exclude").AppendQuoted("[GitVersion.Testing]*");
}

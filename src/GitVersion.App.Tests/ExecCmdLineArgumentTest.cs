using GitVersion.App.Tests.Helpers;
using GitVersion.Helpers;

namespace GitVersion.App.Tests;

[TestFixture]
[Parallelizable(ParallelScope.None)]
public class ExecCmdLineArgumentTest
{
    [Test]
    public void InvalidArgumentsExitCodeShouldNotBeZero()
    {
        using var fixture = new EmptyRepositoryFixture();
        var result = GitVersionHelper.ExecuteIn(fixture.RepositoryPath, arguments: " /invalid-argument");

        result.ExitCode.ShouldNotBe(0);
        result.Output.ShouldNotBeNull();
        result.Output.ShouldContain("Could not parse command line parameter '/invalid-argument'");
    }

    [Test]
    public void LogPathContainsForwardSlash()
    {
        using var fixture = new EmptyRepositoryFixture();
        fixture.MakeATaggedCommit("1.2.3");
        fixture.MakeACommit();

        var result = GitVersionHelper.ExecuteIn(fixture.RepositoryPath,
            """ /l "/tmp/path" """, false);

        result.ExitCode.ShouldBe(0);
        result.Output.ShouldNotBeNull();
        result.Output.ShouldContain(
            """
                    "MajorMinorPatch": "1.2.4"
                    """);
    }

    [Theory]
    [TestCase("", "INFO [")]
    [TestCase("-verbosity NORMAL", "INFO [")]
    [TestCase("-verbosity quiet", "")]
    public void CheckBuildServerVerbosityConsole(string verbosityArg, string expectedOutput)
    {
        using var fixture = new EmptyRepositoryFixture();
        fixture.MakeATaggedCommit("1.2.3");
        fixture.MakeACommit();

        var result = GitVersionHelper.ExecuteIn(fixture.RepositoryPath,
            $""" {verbosityArg} -output buildserver /l "/tmp/path" """, false);

        result.ExitCode.ShouldBe(0);
        result.Output.ShouldNotBeNull();
        result.Output.ShouldContain(expectedOutput);
    }

    [Test]
    public void WorkingDirectoryWithoutGitFolderFailsWithInformativeMessage()
    {
        var workingDirectory = FileSystemHelper.Path.GetTempPathLegacy();
        var result = GitVersionHelper.ExecuteIn(workingDirectory, null, false);

        result.ExitCode.ShouldNotBe(0);
        result.Output.ShouldNotBeNull();
        result.Output.ShouldContain("Cannot find the .git directory");
    }

    [TestCase(" -help")]
    [TestCase(" -version")]
    public void WorkingDirectoryWithoutGitFolderDoesNotFailForVersionAndHelp(string argument)
    {
        var result = GitVersionHelper.ExecuteIn(workingDirectory: null, arguments: argument);

        result.ExitCode.ShouldBe(0);
        result.Output.ShouldNotBeNull();
    }

    [Test]
    public void WorkingDirectoryWithoutCommitsFailsWithInformativeMessage()
    {
        using var fixture = new EmptyRepositoryFixture();

        var result = GitVersionHelper.ExecuteIn(fixture.RepositoryPath, null, false);

        result.ExitCode.ShouldNotBe(0);
        result.Output.ShouldNotBeNull();
        result.Output.ShouldContain("No commits found on the current branch.");
    }

    [Test]
    public void WorkingDirectoryDoesNotExistFailsWithInformativeMessage()
    {
        var workingDirectory = FileSystemHelper.Path.Combine(FileSystemHelper.Path.GetCurrentDirectory(), Guid.NewGuid().ToString("N"));
        var executable = ExecutableHelper.GetDotNetExecutable();

        var output = new StringBuilder();
        var args = ExecutableHelper.GetExecutableArgs($" /targetpath {workingDirectory} ");

        var exitCode = ProcessHelper.Run(
            s => output.AppendLine(s),
            s => output.AppendLine(s),
            null,
            executable,
            args,
            FileSystemHelper.Path.GetCurrentDirectory());

        exitCode.ShouldNotBe(0);
        var outputString = output.ToString();
        outputString.ShouldContain($"The working directory '{workingDirectory}' does not exist.", Case.Insensitive, outputString);
    }
}

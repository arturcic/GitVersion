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
        // Spectre.Console.Cli will show help for unknown options.
        // To test a specific error, we'd need a more specific invalid scenario
        // or to check for the help output. For now, let's use an intentionally broken syntax.
        var result = GitVersionHelper.ExecuteIn(fixture.RepositoryPath, arguments: " --invalid-option");

        result.ExitCode.ShouldNotBe(0);
        result.Output.ShouldNotBeNull();
        // The error message from Spectre.Console.Cli might be different.
        // It usually lists available commands or indicates an unknown option.
        // Example: "Error: Unknown option 'invalid-option'"
        // For now, checking for "Error:" is a general way.
        result.Output.ShouldContain("Error:");
    }

    [Test]
    public void LogPathContainsForwardSlash()
    {
        using var fixture = new EmptyRepositoryFixture();
        fixture.MakeATaggedCommit("1.2.3");
        fixture.MakeACommit();

        var result = GitVersionHelper.ExecuteIn(fixture.RepositoryPath,
            """ --log-file "/tmp/path" """, false);

        result.ExitCode.ShouldBe(0);
        result.Output.ShouldNotBeNull();
        result.Output.ShouldContain(
            """
                    "MajorMinorPatch": "1.2.4"
                    """);
    }

    [Theory]
    [TestCase("", "INFO [")] // Default verbosity is Normal (Info level)
    [TestCase("--verbosity Normal", "INFO [")]
    [TestCase("--verbosity None", "")] // Assuming None means completely quiet for build server output
    public void CheckBuildServerVerbosityConsole(string verbosityArg, string expectedOutput)
    {
        using var fixture = new EmptyRepositoryFixture();
        fixture.MakeATaggedCommit("1.2.3");
        fixture.MakeACommit();

        // Added --output buildserver to ensure build server messages are tested
        var result = GitVersionHelper.ExecuteIn(fixture.RepositoryPath,
            $""" {verbosityArg} --output buildserver --log-file "/tmp/path" """, false);

        result.ExitCode.ShouldBe(0);
        result.Output.ShouldNotBeNull();
        if (!string.IsNullOrEmpty(expectedOutput))
        {
            result.Output.ShouldContain(expectedOutput);
        }
        else
        {
            // If expectedOutput is empty, we might want to assert what *shouldn't* be there,
            // e.g., that "INFO [" is not present.
            // For Verbosity.None, typically no log prefixes like "INFO", "WARN", "ERROR" should appear.
            result.Output.ShouldNotContain("INFO [");
            result.Output.ShouldNotContain("WARN [");
            result.Output.ShouldNotContain("ERROR [");
        }
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

    [TestCase(" --help")]
    [TestCase(" --version")]
    public void WorkingDirectoryWithoutGitFolderDoesNotFailForVersionAndHelp(string argument)
    {
        var result = GitVersionHelper.ExecuteIn(workingDirectory: null, arguments: argument);

        result.ExitCode.ShouldBe(0); // help and version should exit with 0
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
        // Using the positional argument for targetPath
        var args = ExecutableHelper.GetExecutableArgs($" {workingDirectory} ");

        var exitCode = ProcessHelper.Run(
            s => output.AppendLine(s),
            s => output.AppendLine(s),
            null,
            executable,
            args,
            FileSystemHelper.Path.GetCurrentDirectory());

        exitCode.ShouldNotBe(0);
        var outputString = output.ToString();
        // The error message might come from GitVersion's core logic if Spectre allows non-existent paths
        // or from Spectre if it has built-in validation for CommandArgument.
        // For now, let's assume the core logic error message is relevant.
        // If Spectre validates and fails first, this message might need adjustment.
        outputString.ShouldContain($"The working directory '{workingDirectory}' does not exist.", Case.Insensitive, outputString);
    }
}

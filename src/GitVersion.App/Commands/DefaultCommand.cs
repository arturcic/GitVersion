using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using GitVersion.Logging;
using GitVersion.Services;
using GitVersion.Settings;
using Spectre.Console.Cli;

namespace GitVersion.Commands;

public class DefaultCommand : AsyncCommand<GitVersionSettings>
{
    private readonly IGitVersionExecutor _gitVersionExecutor;

    public DefaultCommand(IGitVersionExecutor gitVersionExecutor)
    {
        _gitVersionExecutor = gitVersionExecutor ?? throw new ArgumentNullException(nameof(gitVersionExecutor));
    }

    public override async Task<int> ExecuteAsync(CommandContext context, GitVersionSettings settings)
    {
        var gitVersionOptions = ToGitVersionOptions(settings);
        return await _gitVersionExecutor.ExecuteAsync(gitVersionOptions);
    }

    private static GitVersionOptions ToGitVersionOptions(GitVersionSettings settings)
    {
        var overrideConfiguration = ParseOverrideConfiguration(settings.OverrideConfiguration);
        var verbosity = MapVerbosity(settings.Verbosity);

        var gitVersionOptions = new GitVersionOptions
        {
            AssemblyInfo =
            {
                UpdateProjectFiles = settings.UpdateProjectFiles,
                UpdateAssemblyInfo = settings.UpdateAssemblyInfo,
                EnsureAssemblyInfo = settings.EnsureAssemblyInfo,
                Files = settings.UpdateAssemblyInfoFileName?.ToHashSet() ?? new HashSet<string>()
            },
            Authentication =
            {
                Username = settings.Username,
                Password = settings.Password,
                // Token is not available in GitVersionSettings
            },
            ConfigurationInfo =
            {
                ConfigurationFile = settings.ConfigurationFile,
                OverrideConfiguration = overrideConfiguration,
                ShowConfiguration = settings.ShowConfiguration
            },
            RepositoryInfo =
            {
                TargetUrl = settings.TargetUrl,
                TargetBranch = settings.TargetBranch,
                CommitId = settings.CommitId,
                ClonePath = settings.ClonePath,
                // DynamicRepositoryLocation is mapped to ClonePath
            },
            Settings =
            {
                NoFetch = settings.NoFetch,
                NoCache = settings.NoCache,
                NoNormalize = settings.NoNormalize,
            },
            WixInfo =
            {
                UpdateWixVersionFile = settings.UpdateWixVersionFile
            },
            Diag = settings.Diag,
            IsVersion = settings.IsVersion,
            // IsHelp is handled by Spectre.Console.Cli

            LogFilePath = settings.LogFilePath,
            ShowVariable = settings.ShowVariable,
            Format = settings.Format,
            Verbosity = verbosity,
            Output = settings.Output?.Select(o => Enum.Parse<OutputType>(o, true)).ToHashSet() ?? new HashSet<OutputType> { OutputType.Json },
            OutputFile = settings.OutputFile
        };

        if (settings.TargetPath != null)
        {
            var workingDirectory = settings.TargetPath.TrimEnd('/', '\\');
            gitVersionOptions.WorkingDirectory = workingDirectory;
        }
        
        return gitVersionOptions;
    }

    private static Dictionary<object, object?> ParseOverrideConfiguration(string[]? overrideConfigStrings)
    {
        var overrideConfiguration = new Dictionary<object, object?>();
        if (overrideConfigStrings == null || !overrideConfigStrings.Any())
        {
            return overrideConfiguration;
        }

        foreach (var pair in overrideConfigStrings)
        {
            var parts = QuotedStringHelpers.SplitUnquoted(pair, '=');
            if (parts.Length == 2)
            {
                var key = parts[0];
                var value = parts[1];
                // This is still simplified. The original OverrideConfigurationOptionParser
                // had logic to convert values based on the key (e.g., to bool, int, enum).
                // For now, keeping as string, but this might need future enhancement
                // if specific types are expected by the core logic.
                overrideConfiguration[key] = value;
            }
            else
            {
                // Handle or log malformed override config pair?
                // For now, skipping if not exactly key=value
            }
        }
        return overrideConfiguration;
    }

    private static Logging.Verbosity MapVerbosity(Settings.Verbosity settingsVerbosity)
    {
        return settingsVerbosity switch
        {
            Settings.Verbosity.None => Logging.Verbosity.Quiet, // Closest match
            Settings.Verbosity.Error => Logging.Verbosity.Minimal, // Shows Errors
            Settings.Verbosity.Warn => Logging.Verbosity.Normal,  // Shows Warns, Errors
            Settings.Verbosity.Info => Logging.Verbosity.Normal,  // Shows Info, Warns, Errors
            Settings.Verbosity.Debug => Logging.Verbosity.Verbose, // Shows Debug and above
            Settings.Verbosity.Trace => Logging.Verbosity.Diagnostic, // Shows Trace and above
            _ => Logging.Verbosity.Normal,
        };
    }
}

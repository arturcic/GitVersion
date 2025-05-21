using System.ComponentModel;
using Spectre.Console.Cli;

namespace GitVersion.Settings;

public class GitVersionSettings : CommandSettings
{
    [Description("The working directory (can be a directory, git repository URL or cloned repository).")]
    [CommandArgument(0, "[TARGET_PATH]")]
    public string? TargetPath { get; init; }

    [Description("Path to the log file.")]
    [CommandOption("-l|--log-file <LOG_FILE_PATH>")]
    public string? LogFilePath { get; init; }

    [Description("Outputs the version of GitVersion.")]
    [CommandOption("--version")]
    [DefaultValue(false)]
    public bool IsVersion { get; init; }

    [Description("Specifies the verbosity level.")]
    [CommandOption("--verbosity <VERBOSITY>")]
    [DefaultValue(Verbosity.Normal)]
    public Verbosity Verbosity { get; init; }

    [Description("Skips fetching the repository.")]
    [CommandOption("--no-fetch")]
    [DefaultValue(false)]
    public bool NoFetch { get; init; }

    [Description("Skips using the GitVersion cache.")]
    [CommandOption("--no-cache")]
    [DefaultValue(false)]
    public bool NoCache { get; init; }

    [Description("Skips normalization of the git repository.")]
    [CommandOption("--no-normalize")]
    [DefaultValue(false)]
    public bool NoNormalize { get; init; }

    [Description("Outputs the effective GitVersion configuration.")]
    [CommandOption("--show-config")]
    [DefaultValue(false)]
    public bool ShowConfiguration { get; init; }

    [Description("The path to the GitVersion configuration file.")]
    [CommandOption("--config <CONFIG_FILE>")]
    public string? ConfigurationFile { get; init; }

    [Description("Overrides configuration key-value pairs (e.g., /overrideconfig tag-prefix=TAG).")]
    [CommandOption("--override-config <KEY_VALUE_PAIR>")]
    public string[]? OverrideConfiguration { get; init; }

    // Authentication settings - will be moved to a dedicated command/group later if needed
    [Description("Username for authenticated remote repository.")]
    [CommandOption("-u|--username <USERNAME>")]
    public string? Username { get; init; }

    [Description("Password for authenticated remote repository.")]
    [CommandOption("-p|--password <PASSWORD>")]
    public string? Password { get; init; }

    // Output settings
    [Description("Specifies a single GitVersion variable to output.")]
    [CommandOption("-s|--showvariable <VARIABLE_NAME>")]
    public string? ShowVariable { get; init; }

    [Description("Specifies the output format for GitVersion variables.")]
    [CommandOption("--format <OUTPUT_FORMAT>")]
    public string? Format { get; init; }

    [Description("Path to the output file for GitVersion variables.")]
    [CommandOption("--outputfile <OUTPUT_FILE_PATH>")]
    public string? OutputFile { get; init; }

    [Description("Specifies the output type(s). (Json, File, BuildServer, DotEnv)")]
    [CommandOption("--output <OUTPUT_TYPE>")]
    public string[]? Output { get; init; } // Was ISet<OutputType>, map to string[] and convert

    // Update settings
    [Description("Updates the Wix version file.")]
    [CommandOption("--update-wix-version-file")]
    [DefaultValue(false)]
    public bool UpdateWixVersionFile { get; init; }

    [Description("Updates project files with the version information.")]
    [CommandOption("--update-project-files")]
    [DefaultValue(false)]
    public bool UpdateProjectFiles { get; init; }

    [Description("Updates assembly info files with the version information.")]
    [CommandOption("--update-assemblyinfo")]
    [DefaultValue(false)]
    public bool UpdateAssemblyInfo { get; init; }
    
    [Description("Specifies the assembly info file(s) to update. Can be a globbing pattern.")]
    [CommandOption("--update-assemblyinfo-file-name <ASSEMBLY_INFO_FILE_NAME>")]
    public string[]? UpdateAssemblyInfoFileName { get; init; }

    [Description("Ensures that an assembly info file exists.")]
    [CommandOption("--ensure-assemblyinfo")]
    [DefaultValue(false)]
    public bool EnsureAssemblyInfo { get; init; }

    // Additional properties from Arguments.cs that might be needed
    [Description("The target URL for a dynamic repository.")]
    [CommandOption("--target-url <URL>")]
    public string? TargetUrl { get; init; }

    [Description("The target branch for a dynamic repository.")]
    [CommandOption("-b|--target-branch <BRANCH_NAME>")]
    public string? TargetBranch { get; init; }

    [Description("The commit id to operate on.")]
    [CommandOption("-c|--commit-id <COMMIT_ID>")]
    public string? CommitId { get; init; }
    
    [Description("Path to the cloned dynamic repository.")]
    [CommandOption("--dynamic-repository-clone-path <PATH>")]
    public string? ClonePath { get; init; }

    [Description("Activates diagnostic logging.")]
    [CommandOption("--diag")]
    [DefaultValue(false)]
    public bool Diag { get; init; }
}

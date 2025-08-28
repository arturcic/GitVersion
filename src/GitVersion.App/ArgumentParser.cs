using System.CommandLine;
using System.CommandLine.Parsing;
using System.IO.Abstractions;
using GitVersion.Agents;
using GitVersion.Extensions;
using GitVersion.FileSystemGlobbing;
using GitVersion.Helpers;
using GitVersion.Logging;
using GitVersion.OutputVariables;

namespace GitVersion;

internal class ArgumentParser(IEnvironment environment,
                              IFileSystem fileSystem,
                              ICurrentBuildAgent buildAgent,
                              IConsole console,
                              IGlobbingResolver globbingResolver)
    : IArgumentParser
{
    private readonly IEnvironment environment = environment.NotNull();
    private readonly IFileSystem fileSystem = fileSystem.NotNull();
    private readonly ICurrentBuildAgent buildAgent = buildAgent.NotNull();
    private readonly IConsole console = console.NotNull();
    private readonly IGlobbingResolver globbingResolver = globbingResolver.NotNull();

    // Option declarations with inline initialization
    private readonly Argument<string> pathArgument = new("path")
    {
        Arity = ArgumentArity.ZeroOrOne,
        Description = "The directory containing .git. If not defined current directory is used."
    };
    private readonly Option<bool> helpOption = new("-h", "Shows help");
    private readonly Option<bool> versionOption = new("-version") { Description = "Displays the version of GitVersion" };
    private readonly Option<bool> diagOption = new("--diag") { Description = "Runs GitVersion with additional diagnostic information (requires git.exe to be installed)" };
    private readonly Option<string> targetPathOption = new("-targetpath") { Description = "Same as 'path', but not positional" };
    private readonly Option<List<OutputType>> outputOption = new("-output", "Determines the output to the console. Can be either 'json', 'file', 'buildserver' or 'dotenv', will default to 'json'")
    {
        AllowMultipleArgumentsPerToken = true
    };
    private readonly Option<string> outputFileOption = new("--outputfile", "Path to output file. It is used in combination with --output 'file'");
    private readonly Option<string> showVariableOption = new("-showvariable", "Used in conjunction with --output json, will output just a particular variable");
    private readonly Option<string> formatOption = new("-format", "Used in conjunction with --output json, will output a format containing version variables");
    private readonly Option<string> logFileOption = new("-l", "Path to logfile");
    private readonly Option<string> configOption = new("-config", "Path to config file (defaults to GitVersion.yml, GitVersion.yaml, .GitVersion.yml or .GitVersion.yaml)");
    private readonly Option<bool> showConfigOption = new("-showconfig", "Outputs the effective GitVersion config in yaml format");
    private readonly Option<List<string>> overrideConfigOption = new("-overrideconfig", "Overrides GitVersion config values inline (key=value pairs)")
    {
        AllowMultipleArgumentsPerToken = true
    };
    private readonly Option<bool> noCacheOption = new("-nocache", "Bypasses the cache, result will not be written to the cache");
    private readonly Option<bool> noNormalizeOption = new("-nonormalize", "Disables normalize step on a build server");
    private readonly Option<bool> allowShallowOption = new("-allowshallow", "Allows GitVersion to run on a shallow clone");
    private readonly Option<Verbosity> verbosityOption = new("-verbosity", "Specifies the amount of information to be displayed");
    private readonly Option<List<string>> updateAssemblyInfoOption = new("-updateAssemblyInfo", "Will recursively search for all 'AssemblyInfo.cs' files in the git repo and update them")
    {
        AllowMultipleArgumentsPerToken = true
    };
    private readonly Option<List<string>> updateProjectFilesOption = new("-updateProjectFiles", "Will recursively search for all project files (.csproj/.vbproj/.fsproj) files in the git repo and update them")
    {
        AllowMultipleArgumentsPerToken = true
    };
    private readonly Option<bool> ensureAssemblyInfoOption = new("-ensureAssemblyInfo") { Description = "If the assembly info file specified with --updateassemblyinfo is not found, it will be created" };
    private readonly Option<bool> updateWixVersionFileOption = new("--updatewixversionfile") { Description = "All the GitVersion variables are written to 'GitVersion_WixVersion.wxi'" };
    private readonly Option<string> urlOption = new("-url") { Description = "Url to remote git repository" };
    private readonly Option<string> branchOption = new("-b") { Description = "Name of the branch to use on the remote repository, must be used in combination with --url" };
    private readonly Option<string> usernameOption = new("-u") { Description = "Username in case authentication is required" };
    private readonly Option<string> passwordOption = new("-p") { Description = "Password in case authentication is required" };
    private readonly Option<string> commitOption = new("--commit") { Description = "The commit id to check. If not specified, the latest available commit on the specified branch will be used" };
    private readonly Option<string> dynamicRepoLocationOption = new("-dynamicRepoLocation") { Description = "By default dynamic repositories will be cloned to %tmp%. Use this switch to override" };
    private readonly Option<bool> noFetchOption = new("-nofetch") { Description = "Disables 'git fetch' during version calculation" };

    private const string defaultOutputFileName = "GitVersion.json";
    private static readonly IEnumerable<string> availableVariables = GitVersionVariables.AvailableVariables;

    public Arguments ParseArguments(string commandLineArguments)
    {
        var arguments = QuotedStringHelpers.SplitUnquoted(commandLineArguments, ' ');
        return ParseArguments(arguments);
    }

    public Arguments ParseArguments(string[] commandLineArguments)
    {
        var rootCommand = CreateRootCommand();
        var parseResult = rootCommand.Parse(commandLineArguments);

        if (parseResult.Errors.Any())
        {
            throw new WarningException(string.Join(System.Environment.NewLine, parseResult.Errors.Select(e => e.Message)));
        }

        return CreateArgumentsFromParseResult(parseResult);
    }

    private RootCommand CreateRootCommand()
    {
        var rootCommand = new RootCommand("Use convention to derive a SemVer product version from a GitFlow or GitHub based repository.");

        // Add positional argument
        rootCommand.Add(pathArgument);

        // Add all options
        rootCommand.Add(helpOption);
        rootCommand.Add(versionOption);
        rootCommand.Add(diagOption);
        rootCommand.Add(targetPathOption);
        rootCommand.Add(outputOption);
        rootCommand.Add(outputFileOption);
        rootCommand.Add(showVariableOption);
        rootCommand.Add(formatOption);
        rootCommand.Add(logFileOption);
        rootCommand.Add(configOption);
        rootCommand.Add(showConfigOption);
        rootCommand.Add(overrideConfigOption);
        rootCommand.Add(noCacheOption);
        rootCommand.Add(noNormalizeOption);
        rootCommand.Add(allowShallowOption);
        rootCommand.Add(verbosityOption);
        rootCommand.Add(updateAssemblyInfoOption);
        rootCommand.Add(updateProjectFilesOption);
        rootCommand.Add(ensureAssemblyInfoOption);
        rootCommand.Add(updateWixVersionFileOption);
        rootCommand.Add(urlOption);
        rootCommand.Add(branchOption);
        rootCommand.Add(usernameOption);
        rootCommand.Add(passwordOption);
        rootCommand.Add(commitOption);
        rootCommand.Add(dynamicRepoLocationOption);
        rootCommand.Add(noFetchOption);

        return rootCommand;
    }

    private Arguments CreateArgumentsFromParseResult(ParseResult parseResult)
    {
        var arguments = new Arguments();

        // Handle help and version first
        if (parseResult.GetValue(helpOption))
        {
            return new Arguments { IsHelp = true };
        }

        if (parseResult.GetValue(versionOption))
        {
            return new Arguments { IsVersion = true };
        }

        // Add authentication from environment
        AddAuthentication(arguments);

        // Set default output
        arguments.Output.Add(OutputType.Json);

        // Parse positional argument (target path)
        var pathValue = parseResult.GetValue(pathArgument);
        var targetPathValue = parseResult.GetValue(targetPathOption);

        if (targetPathValue != null)
        {
            arguments.TargetPath = targetPathValue;
        }
        else if (pathValue != null)
        {
            arguments.TargetPath = pathValue;
        }
        else
        {
            arguments.TargetPath = SysEnv.CurrentDirectory;
        }

        arguments.TargetPath = arguments.TargetPath.TrimEnd('/', '\\');

        // Parse all other options
        arguments.Diag = parseResult.GetValue(diagOption);
        arguments.NoCache = parseResult.GetValue(noCacheOption);
        arguments.NoNormalize = parseResult.GetValue(noNormalizeOption);
        arguments.AllowShallow = parseResult.GetValue(allowShallowOption);
        arguments.NoFetch = parseResult.GetValue(noFetchOption) || this.buildAgent.PreventFetch();

        // Output options
        var outputs = parseResult.GetValue(outputOption);
        if (outputs != null && outputs.Any())
        {
            arguments.Output.Clear();
            foreach (var output in outputs)
            {
                arguments.Output.Add(output);
            }
        }

        arguments.OutputFile = parseResult.GetValue(outputFileOption);
        if (arguments.Output.Contains(OutputType.File) && arguments.OutputFile == null)
        {
            arguments.OutputFile = defaultOutputFileName;
        }

        // Variable and format options
        var showVariable = parseResult.GetValue(showVariableOption);
        if (showVariable != null)
        {
            arguments.ShowVariable = ValidateVariableName(showVariable);
        }

        var format = parseResult.GetValue(formatOption);
        if (format != null)
        {
            arguments.Format = ValidateFormatString(format);
        }

        arguments.LogFilePath = parseResult.GetValue(logFileOption);
        arguments.ConfigurationFile = parseResult.GetValue(configOption);
        arguments.ShowConfiguration = parseResult.GetValue(showConfigOption);

        // Override config
        var overrideConfigs = parseResult.GetValue(overrideConfigOption);
        if (overrideConfigs != null && overrideConfigs.Any())
        {
            arguments.OverrideConfiguration = ParseOverrideConfiguration(overrideConfigs);
        }

        arguments.Verbosity = parseResult.GetValue(verbosityOption);

        // Assembly info options
        var updateAssemblyInfo = parseResult.GetValue(updateAssemblyInfoOption);
        if (updateAssemblyInfo != null && updateAssemblyInfo.Any())
        {
            arguments.UpdateAssemblyInfo = true;
            foreach (var file in updateAssemblyInfo)
            {
                arguments.UpdateAssemblyInfoFileName.Add(file);
            }
        }

        var updateProjectFiles = parseResult.GetValue(updateProjectFilesOption);
        if (updateProjectFiles != null && updateProjectFiles.Any())
        {
            arguments.UpdateProjectFiles = true;
            foreach (var file in updateProjectFiles)
            {
                arguments.UpdateAssemblyInfoFileName.Add(file);
            }
        }

        arguments.EnsureAssemblyInfo = parseResult.GetValue(ensureAssemblyInfoOption);
        arguments.UpdateWixVersionFile = parseResult.GetValue(updateWixVersionFileOption);

        // Remote repository options
        arguments.TargetUrl = parseResult.GetValue(urlOption);
        arguments.TargetBranch = parseResult.GetValue(branchOption);
        arguments.Authentication.Username = parseResult.GetValue(usernameOption);
        arguments.Authentication.Password = parseResult.GetValue(passwordOption);
        arguments.CommitId = parseResult.GetValue(commitOption);
        arguments.ClonePath = parseResult.GetValue(dynamicRepoLocationOption);

        // Validate configuration file
        ValidateConfigurationFile(arguments);

        // Resolve assembly info files
        if (!arguments.EnsureAssemblyInfo)
        {
            arguments.UpdateAssemblyInfoFileName = ResolveFiles(arguments.TargetPath, arguments.UpdateAssemblyInfoFileName).ToHashSet();
        }

        // Validation checks
        if (arguments.UpdateProjectFiles && arguments.UpdateAssemblyInfo)
        {
            throw new WarningException("Cannot specify both updateprojectfiles and updateassemblyinfo in the same run. Please rerun GitVersion with only one parameter");
        }

        if (arguments.UpdateProjectFiles && arguments.EnsureAssemblyInfo)
        {
            throw new WarningException("Cannot specify --ensureassemblyinfo with updateprojectfiles: please ensure your project file exists before attempting to update it");
        }

        if (arguments.UpdateAssemblyInfoFileName.Count > 1 && arguments.EnsureAssemblyInfo)
        {
            throw new WarningException("Can't specify multiple assembly info files when using --ensureassemblyinfo switch, either use a single assembly info file or do not specify --ensureassemblyinfo and create assembly info files manually");
        }

        return arguments;
    }

    private void ValidateConfigurationFile(Arguments arguments)
    {
        if (arguments.ConfigurationFile.IsNullOrWhiteSpace()) return;

        if (FileSystemHelper.Path.IsPathRooted(arguments.ConfigurationFile))
        {
            if (!this.fileSystem.File.Exists(arguments.ConfigurationFile))
            {
                throw new WarningException($"Could not find config file at '{arguments.ConfigurationFile}'");
            }
            arguments.ConfigurationFile = FileSystemHelper.Path.GetFullPath(arguments.ConfigurationFile);
        }
        else
        {
            var configFilePath = FileSystemHelper.Path.GetFullPath(FileSystemHelper.Path.Combine(arguments.TargetPath, arguments.ConfigurationFile));
            if (!this.fileSystem.File.Exists(configFilePath))
            {
                throw new WarningException($"Could not find config file at '{configFilePath}'");
            }
            arguments.ConfigurationFile = configFilePath;
        }
    }

    private void AddAuthentication(Arguments arguments)
    {
        var username = this.environment.GetEnvironmentVariable("GITVERSION_REMOTE_USERNAME");
        if (!username.IsNullOrWhiteSpace())
        {
            arguments.Authentication.Username = username;
        }

        var password = this.environment.GetEnvironmentVariable("GITVERSION_REMOTE_PASSWORD");
        if (!password.IsNullOrWhiteSpace())
        {
            arguments.Authentication.Password = password;
        }
    }

    private IEnumerable<string> ResolveFiles(string workingDirectory, ISet<string>? assemblyInfoFiles)
    {
        if (assemblyInfoFiles == null) yield break;

        foreach (var file in assemblyInfoFiles)
        {
            foreach (var path in this.globbingResolver.Resolve(workingDirectory, file))
            {
                yield return path;
            }
        }
    }

    private string ValidateVariableName(string variableName)
    {
        var versionVariable = availableVariables.SingleOrDefault(av => av.Equals(variableName.Replace("'", ""), StringComparison.CurrentCultureIgnoreCase));
        if (versionVariable == null)
        {
            var message = $"--showvariable requires a valid version variable. Available variables are:{System.Environment.NewLine}" +
                          string.Join(", ", availableVariables.Select(x => $"'{x}'"));
            throw new WarningException(message);
        }
        return versionVariable;
    }

    private string ValidateFormatString(string format)
    {
        if (format.IsNullOrWhiteSpace())
        {
            throw new WarningException("Format requires a valid format string. Available variables are: " + string.Join(", ", availableVariables));
        }

        var foundVariable = availableVariables.Any(variable => format.Contains(variable, StringComparison.CurrentCultureIgnoreCase));
        if (!foundVariable)
        {
            throw new WarningException("Format requires a valid format string. Available variables are: " + string.Join(", ", availableVariables));
        }

        return format;
    }

    private IReadOnlyDictionary<object, object?> ParseOverrideConfiguration(List<string> overrideConfigs)
    {
        var parser = new OverrideConfigurationOptionParser();

        foreach (var keyValueOption in overrideConfigs)
        {
            var keyAndValue = QuotedStringHelpers.SplitUnquoted(keyValueOption, '=');
            if (keyAndValue.Length != 2)
            {
                throw new WarningException($"Could not parse --overrideconfig option: {keyValueOption}. Ensure it is in format 'key=value'.");
            }

            var optionKey = keyAndValue[0].ToLowerInvariant();
            if (!OverrideConfigurationOptionParser.SupportedProperties.Contains(optionKey))
            {
                throw new WarningException($"Could not parse --overrideconfig option: {keyValueOption}. Unsupported 'key'.");
            }
            parser.SetValue(optionKey, keyAndValue[1]);
        }

        return parser.GetOverrideConfiguration();
    }
}

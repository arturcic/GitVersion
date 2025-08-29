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
    private readonly Option<bool> helpOption = new Option<bool>("-h") { Description = "Shows help" };
    private readonly Option<bool> versionOption = new Option<bool>("-version") { Description = "Displays the version of GitVersion" };
    private readonly Option<bool> diagOption = new Option<bool>("--diag") { Description = "Runs GitVersion with additional diagnostic information (requires git.exe to be installed)" };
    private readonly Option<string> targetPathOption = new Option<string>("-targetpath") { Description = "Same as 'path', but not positional" };
    private readonly Option<string> targetPathLegacyOption = new Option<string>("/targetpath") { Description = "Same as 'path', but not positional" };
    private readonly Option<List<OutputType>> outputOption = new Option<List<OutputType>>("-output")
    {
        AllowMultipleArgumentsPerToken = true
    };
    private readonly Option<List<OutputType>> outputLegacyOption = new Option<List<OutputType>>("/output")
    {
        AllowMultipleArgumentsPerToken = true
    };

    private readonly Option<List<string>> overrideConfigOption = new Option<List<string>>("-overrideconfig")
    {
        AllowMultipleArgumentsPerToken = true
    };

    private readonly Option<List<string>> overrideConfigLegacyOption = new Option<List<string>>("/overrideconfig")
    {
        AllowMultipleArgumentsPerToken = true,
        Arity = ArgumentArity.ZeroOrMore
    };

    private readonly Option<List<string>> updateAssemblyInfoOption = new Option<List<string>>("-updateAssemblyInfo")
    {
        AllowMultipleArgumentsPerToken = true,
        Arity = ArgumentArity.ZeroOrMore
    };
    private readonly Option<List<string>> updateAssemblyInfoLegacyOption = new Option<List<string>>("/updateassemblyinfo")
    {
        Arity = ArgumentArity.ZeroOrMore
    };

    private readonly Option<List<string>> updateProjectFilesOption = new Option<List<string>>("-updateProjectFiles")
    {
        AllowMultipleArgumentsPerToken = true,
        Arity = ArgumentArity.ZeroOrMore
    };
    private readonly Option<List<string>> updateProjectFilesLegacyOption = new Option<List<string>>("/updateprojectfiles")
    {
        Arity = ArgumentArity.ZeroOrMore
    };
    private readonly Option<string> ensureAssemblyInfoOption = new Option<string>("-ensureassemblyinfo") { Arity = ArgumentArity.ZeroOrOne, Description = "If the assembly info file specified with --updateassemblyinfo is not found, it will be created" };
    private readonly Option<string> ensureAssemblyInfoLegacyOption = new Option<string>("/ensureassemblyinfo") { Arity = ArgumentArity.ZeroOrOne, Description = "If the assembly info file specified with --updateassemblyinfo is not found, it will be created" };
    private readonly Option<bool> updateWixVersionFileOption = new Option<bool>("--updatewixversionfile") { Description = "All the GitVersion variables are written to 'GitVersion_WixVersion.wxi'" };
    private readonly Option<bool> updateWixVersionFileLegacyOption = new Option<bool>("/updatewixversionfile") { Description = "All the GitVersion variables are written to 'GitVersion_WixVersion.wxi'" };
    private readonly Option<string> urlOption = new Option<string>("-url") { Description = "Url to remote git repository" };
    private readonly Option<string> urlLegacyOption = new Option<string>("/url") { Description = "Url to remote git repository" };
    private readonly Option<string> branchOption = new Option<string>("-b") { Description = "Name of the branch to use on the remote repository, must be used in combination with --url" };
    private readonly Option<string> branchLegacyOption = new Option<string>("/b") { Description = "Name of the branch to use on the remote repository, must be used in combination with --url" };
    private readonly Option<string> usernameOption = new Option<string>("-u") { Description = "Username in case authentication is required" };
    private readonly Option<string> usernameLegacyOption = new Option<string>("/u") { Description = "Username in case authentication is required" };
    private readonly Option<string> passwordOption = new Option<string>("-p") { Description = "Password in case authentication is required" };
    private readonly Option<string> passwordLegacyOption = new Option<string>("/p") { Description = "Password in case authentication is required" };
    private readonly Option<string> commitOption = new Option<string>("--commit") { Description = "The commit id to check. If not specified, the latest available commit on the specified branch will be used" };
    private readonly Option<string> commitLegacyOption = new Option<string>("/commit") { Description = "The commit id to check. If not specified, the latest available commit on the specified branch will be used" };
    private readonly Option<string> dynamicRepoLocationOption = new Option<string>("-dynamicRepoLocation") { Description = "By default dynamic repositories will be cloned to %tmp%. Use this switch to override" };
    private readonly Option<string> dynamicRepoLocationLegacyOption = new Option<string>("/dynamicRepoLocation") { Description = "By default dynamic repositories will be cloned to %tmp%. Use this switch to override" };
    private readonly Option<string> outputFileOption = new Option<string>("-outputfile") { Description = "Allows overriding the default output file name (default: GitVersion.json)" };
    private readonly Option<string> outputFileLegacyOption = new Option<string>("/outputfile") { Description = "Allows overriding the default output file name (default: GitVersion.json)" };
    private readonly Option<string> showVariableOption = new Option<string>("-showvariable") { Description = "Shows the value of a specific variable" };
    private readonly Option<string> showVariableLegacyOption = new Option<string>("/showvariable") { Description = "Shows the value of a specific variable" };
    private readonly Option<string> formatOption = new Option<string>("-format") { Description = "Outputs the version in the specified format (e.g. json, env, msbuild)" };
    private readonly Option<string> formatLegacyOption = new Option<string>("/format") { Description = "Outputs the version in the specified format (e.g. json, env, msbuild)" };
    private readonly Option<string> logFileOption = new Option<string>("-l") { Description = "File to log output to" };
    private readonly Option<string> logFileLegacyOption = new Option<string>("/l") { Description = "File to log output to" };
    private readonly Option<string> configOption = new Option<string>("-config") { Description = "Path to config file" };
    private readonly Option<string> configLegacyOption = new Option<string>("/config") { Description = "Path to config file" };
    private readonly Option<bool> showConfigOption = new Option<bool>("-showconfig") { Description = "Outputs the effective configuration (including defaults)" };
    private readonly Option<bool> showConfigLegacyOption = new Option<bool>("/showconfig") { Description = "Outputs the effective configuration (including defaults)" };
    private readonly Option<bool> noCacheOption = new Option<bool>("-nocache") { Description = "Disables the cache" };
    private readonly Option<bool> noCacheLegacyOption = new Option<bool>("/nocache") { Description = "Disables the cache" };
    private readonly Option<bool> noNormalizeOption = new Option<bool>("-nonormalize") { Description = "Disables normalization of the working directory path" };
    private readonly Option<bool> noNormalizeLegacyOption = new Option<bool>("/nonormalize") { Description = "Disables normalization of the working directory path" };
    private readonly Option<bool> allowShallowOption = new Option<bool>("-allowshallow") { Description = "Allows using shallow clones" };
    private readonly Option<bool> allowShallowLegacyOption = new Option<bool>("/allowshallow") { Description = "Allows using shallow clones" };
    private readonly Option<Verbosity> verbosityOption = new Option<Verbosity>("-verbosity") { Description = "Sets the verbosity level" };
    private readonly Option<Verbosity> verbosityLegacyOption = new Option<Verbosity>("/verbosity") { Description = "Sets the verbosity level" };
    private readonly Option<bool> noFetchOption = new Option<bool>("-nofetch") { Description = "Disables 'git fetch' during version calculation" };
    private readonly Option<bool> noFetchLegacyOption = new Option<bool>("/nofetch") { Description = "Disables 'git fetch' during version calculation" };

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

        // Check for help before parsing errors
        if (parseResult.GetValue(helpOption) || commandLineArguments.Contains("-h") || commandLineArguments.Contains("--help") || commandLineArguments.Contains("-help") || commandLineArguments.Contains("-?"))
        {
            var helpArgs = new Arguments { IsHelp = true };
            helpArgs.TargetPath = null;
            return helpArgs;
        }

        // Check for version before parsing errors
        if (parseResult.GetValue(versionOption) || commandLineArguments.Contains("-version") || commandLineArguments.Contains("--version"))
        {
            var versionArgs = new Arguments { IsVersion = true };
            versionArgs.TargetPath = null;
            return versionArgs;
        }

        // Check for invalid legacy options (starting with / but not valid)
        foreach (var arg in commandLineArguments)
        {
            if (arg.StartsWith("/") && !IsValidLegacyOption(arg))
            {
                throw new WarningException($"Could not parse command line parameter '{arg}'.");
            }
        }

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
        rootCommand.Add(targetPathLegacyOption);
        rootCommand.Add(outputOption);
        rootCommand.Add(outputLegacyOption);
        rootCommand.Add(outputFileOption);
        rootCommand.Add(outputFileLegacyOption);
        rootCommand.Add(showVariableOption);
        rootCommand.Add(showVariableLegacyOption);
        rootCommand.Add(formatOption);
        rootCommand.Add(formatLegacyOption);
        rootCommand.Add(logFileOption);
        rootCommand.Add(logFileLegacyOption);
        rootCommand.Add(configOption);
        rootCommand.Add(configLegacyOption);
        rootCommand.Add(showConfigOption);
        rootCommand.Add(showConfigLegacyOption);
        rootCommand.Add(overrideConfigOption);
        rootCommand.Add(overrideConfigLegacyOption);
        rootCommand.Add(noCacheOption);
        rootCommand.Add(noCacheLegacyOption);
        rootCommand.Add(noNormalizeOption);
        rootCommand.Add(noNormalizeLegacyOption);
        rootCommand.Add(allowShallowOption);
        rootCommand.Add(allowShallowLegacyOption);
        rootCommand.Add(verbosityOption);
        rootCommand.Add(verbosityLegacyOption);
        rootCommand.Add(updateAssemblyInfoOption);
        rootCommand.Add(updateAssemblyInfoLegacyOption);
        rootCommand.Add(updateProjectFilesOption);
        rootCommand.Add(updateProjectFilesLegacyOption);
        rootCommand.Add(ensureAssemblyInfoOption);
        rootCommand.Add(ensureAssemblyInfoLegacyOption);
        rootCommand.Add(updateWixVersionFileOption);
        rootCommand.Add(updateWixVersionFileLegacyOption);
        rootCommand.Add(urlOption);
        rootCommand.Add(urlLegacyOption);
        rootCommand.Add(branchOption);
        rootCommand.Add(branchLegacyOption);
        rootCommand.Add(usernameOption);
        rootCommand.Add(usernameLegacyOption);
        rootCommand.Add(passwordOption);
        rootCommand.Add(passwordLegacyOption);
        rootCommand.Add(commitOption);
        rootCommand.Add(commitLegacyOption);
        rootCommand.Add(dynamicRepoLocationOption);
        rootCommand.Add(dynamicRepoLocationLegacyOption);
        rootCommand.Add(noFetchOption);
        rootCommand.Add(noFetchLegacyOption);

        return rootCommand;
    }

    private Arguments CreateArgumentsFromParseResult(ParseResult parseResult)
    {
        var arguments = new Arguments();

        // Add authentication from environment
        AddAuthentication(arguments);

        // Set default output
        arguments.Output.Add(OutputType.Json);

        // Parse positional argument (target path)
        var pathValue = parseResult.GetValue(pathArgument);
        var targetPathValue = parseResult.GetValue(targetPathOption);
        var targetPathLegacyValue = parseResult.GetValue(targetPathLegacyOption);

        if (targetPathLegacyValue != null)
        {
            arguments.TargetPath = targetPathLegacyValue;
        }
        else if (targetPathValue != null)
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
        arguments.NoCache = parseResult.GetValue(noCacheOption) || parseResult.GetValue(noCacheLegacyOption);
        arguments.NoNormalize = parseResult.GetValue(noNormalizeOption) || parseResult.GetValue(noNormalizeLegacyOption);
        arguments.AllowShallow = parseResult.GetValue(allowShallowOption) || parseResult.GetValue(allowShallowLegacyOption);
        arguments.NoFetch = (parseResult.GetValue(noFetchOption) || parseResult.GetValue(noFetchLegacyOption)) || this.buildAgent.PreventFetch();

        // Output options
        var outputs = parseResult.GetValue(outputOption);
        var outputsLegacy = parseResult.GetValue(outputLegacyOption);
        if (outputsLegacy != null && outputsLegacy.Any())
        {
            arguments.Output.Clear();
            foreach (var output in outputsLegacy)
            {
                arguments.Output.Add(output);
            }
        }
        else if (outputs != null && outputs.Any())
        {
            arguments.Output.Clear();
            foreach (var output in outputs)
            {
                arguments.Output.Add(output);
            }
        }

        arguments.OutputFile = parseResult.GetValue(outputFileOption) ?? parseResult.GetValue(outputFileLegacyOption);
        if (arguments.Output.Contains(OutputType.File) && arguments.OutputFile == null)
        {
            arguments.OutputFile = defaultOutputFileName;
        }

        // Variable and format options
        var showVariable = parseResult.GetValue(showVariableOption) ?? parseResult.GetValue(showVariableLegacyOption);
        if (showVariable != null)
        {
            arguments.ShowVariable = ValidateVariableName(showVariable);
        }

        var format = parseResult.GetValue(formatOption) ?? parseResult.GetValue(formatLegacyOption);
        if (format != null)
        {
            arguments.Format = ValidateFormatString(format);
        }

        arguments.LogFilePath = parseResult.GetValue(logFileOption) ?? parseResult.GetValue(logFileLegacyOption);
        arguments.ConfigurationFile = parseResult.GetValue(configOption) ?? parseResult.GetValue(configLegacyOption);
        arguments.ShowConfiguration = parseResult.GetValue(showConfigOption) || parseResult.GetValue(showConfigLegacyOption);

        // Override config
        var overrideConfigs = parseResult.GetValue(overrideConfigOption);
        var overrideConfigsLegacy = parseResult.GetValue(overrideConfigLegacyOption);

        // Combine values from both options
        var allOverrideConfigs = new List<string>();
        if (overrideConfigs != null && overrideConfigs.Any())
        {
            allOverrideConfigs.AddRange(overrideConfigs);
        }
        if (overrideConfigsLegacy != null && overrideConfigsLegacy.Any())
        {
            allOverrideConfigs.AddRange(overrideConfigsLegacy);
        }

        if (allOverrideConfigs.Any())
        {
            arguments.OverrideConfiguration = ParseOverrideConfiguration(allOverrideConfigs, overrideConfigsLegacy != null && overrideConfigsLegacy.Any() ? "/overrideconfig" : "--overrideconfig");
        }

        var verbosityValue = parseResult.GetValue(verbosityOption);
        var verbosityLegacyValue = parseResult.GetValue(verbosityLegacyOption);
        arguments.Verbosity = verbosityLegacyValue != default(Verbosity) ? verbosityLegacyValue : verbosityValue;

        // Assembly info options
        var updateAssemblyInfo = parseResult.GetValue(updateAssemblyInfoOption);
        var updateAssemblyInfoLegacy = parseResult.GetValue(updateAssemblyInfoLegacyOption);

        // Check if either option was specified (even if empty) by checking tokens
        var hasUpdateAssemblyInfo = parseResult.Tokens.Any(t => t.Value == "-updateAssemblyInfo" || t.Value == "/updateassemblyinfo");

        // Use legacy option if it has values, otherwise use new option
        var finalUpdateAssemblyInfo = (updateAssemblyInfoLegacy != null && updateAssemblyInfoLegacy.Any())
            ? updateAssemblyInfoLegacy
            : updateAssemblyInfo;

        if (hasUpdateAssemblyInfo)
        {
            if (finalUpdateAssemblyInfo != null && finalUpdateAssemblyInfo.Any())
            {
                // Check if the first argument is a boolean false value
                var firstArg = finalUpdateAssemblyInfo.First();
                if (string.Equals(firstArg, "false", StringComparison.OrdinalIgnoreCase) ||
                    string.Equals(firstArg, "0", StringComparison.OrdinalIgnoreCase))
                {
                    arguments.UpdateAssemblyInfo = false;
                }
                else
                {
                    arguments.UpdateAssemblyInfo = true;
                    arguments.UpdateAssemblyInfoFileName.AddRange(finalUpdateAssemblyInfo);
                }
            }
            else
            {
                // Option specified without arguments - treat as true
                arguments.UpdateAssemblyInfo = true;
            }
        }

        var updateProjectFiles = parseResult.GetValue(updateProjectFilesOption);
        var updateProjectFilesLegacy = parseResult.GetValue(updateProjectFilesLegacyOption);

        // Check if either option was specified (even if empty) by checking tokens
        var hasUpdateProjectFiles = parseResult.Tokens.Any(t => t.Value == "-updateProjectFiles" || t.Value == "/updateprojectfiles");

        // Use legacy option if it has values, otherwise use new option
        var finalUpdateProjectFiles = (updateProjectFilesLegacy != null && updateProjectFilesLegacy.Any())
            ? updateProjectFilesLegacy
            : updateProjectFiles;

        if (hasUpdateProjectFiles)
        {
            if (finalUpdateProjectFiles != null && finalUpdateProjectFiles.Any())
            {
                arguments.UpdateProjectFiles = true;
                arguments.UpdateAssemblyInfoFileName.AddRange(finalUpdateProjectFiles);
            }
            else
            {
                // Option specified without arguments - treat as true
                arguments.UpdateProjectFiles = true;
            }
        }

        // Parse EnsureAssemblyInfo option
        var ensureAssemblyInfoValue = parseResult.GetValue(ensureAssemblyInfoOption) ?? parseResult.GetValue(ensureAssemblyInfoLegacyOption);
        if (ensureAssemblyInfoValue != null)
        {
            // If a value is provided, parse it as boolean
            if (string.Equals(ensureAssemblyInfoValue, "false", StringComparison.OrdinalIgnoreCase) ||
                string.Equals(ensureAssemblyInfoValue, "0", StringComparison.OrdinalIgnoreCase))
            {
                arguments.EnsureAssemblyInfo = false;
            }
            else
            {
                arguments.EnsureAssemblyInfo = true;
            }
        }
        else
        {
            // Check if the option was specified without a value (should default to true)
            var hasEnsureAssemblyInfo = parseResult.Tokens.Any(t => t.Value == "-ensureassemblyinfo" || t.Value == "/ensureassemblyinfo");
            arguments.EnsureAssemblyInfo = hasEnsureAssemblyInfo;
        }
        arguments.UpdateWixVersionFile = parseResult.GetValue(updateWixVersionFileOption) || parseResult.GetValue(updateWixVersionFileLegacyOption);

        // Remote repository options
        arguments.TargetUrl = parseResult.GetValue(urlOption) ?? parseResult.GetValue(urlLegacyOption);
        arguments.TargetBranch = parseResult.GetValue(branchOption) ?? parseResult.GetValue(branchLegacyOption);

        // Only set authentication from command line if explicitly provided
        var usernameValue = parseResult.GetValue(usernameOption) ?? parseResult.GetValue(usernameLegacyOption);
        if (usernameValue != null)
        {
            arguments.Authentication.Username = usernameValue;
        }

        var passwordValue = parseResult.GetValue(passwordOption) ?? parseResult.GetValue(passwordLegacyOption);
        if (passwordValue != null)
        {
            arguments.Authentication.Password = passwordValue;
        }

        arguments.CommitId = parseResult.GetValue(commitOption) ?? parseResult.GetValue(commitLegacyOption);
        arguments.ClonePath = parseResult.GetValue(dynamicRepoLocationOption) ?? parseResult.GetValue(dynamicRepoLocationLegacyOption);

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

    private IReadOnlyDictionary<object, object?> ParseOverrideConfiguration(List<string> overrideConfigs, string optionName = "--overrideconfig")
    {
        var parser = new OverrideConfigurationOptionParser();

        foreach (var keyValueOption in overrideConfigs)
        {
            var keyAndValue = QuotedStringHelpers.SplitUnquoted(keyValueOption, '=');
            if (keyAndValue.Length != 2)
            {
                throw new WarningException($"Could not parse {optionName} option: {keyValueOption}. Ensure it is in format 'key=value'.");
            }

            var optionKey = keyAndValue[0].ToLowerInvariant();
            if (!OverrideConfigurationOptionParser.SupportedProperties.Contains(optionKey))
            {
                throw new WarningException($"Could not parse {optionName} option: {keyValueOption}. Unsupported 'key'.");
            }
            parser.SetValue(optionKey, keyAndValue[1]);
        }

        return parser.GetOverrideConfiguration();
    }

    private bool IsValidLegacyOption(string arg)
    {
        // Only check arguments that look like legacy options (start with / followed by option name)
        if (!arg.StartsWith("/"))
        {
            return true; // Not a legacy option, so it's valid
        }

        // Skip validation for arguments that look like absolute file paths
        // Check if it looks like a Unix absolute path with multiple components
        if (arg.Contains('/') && arg.Length > 1)
        {
            var pathParts = arg.Split('/', StringSplitOptions.RemoveEmptyEntries);
            // If it has multiple path components or looks like a typical Unix path, treat as file path
            if (pathParts.Length > 1 || arg.Contains('.') || arg.Length > 20)
            {
                return true; // Looks like a file path, not a legacy option
            }
        }

        // Extract the option name without the leading /
        var optionName = arg.Split('=')[0].TrimStart('/').ToLowerInvariant();

        // List of valid legacy options
        var validLegacyOptions = new[]
        {
            "targetpath", "output", "overrideconfig", "updateassemblyinfo", "updateprojectfiles",
            "ensureassemblyinfo", "updatewixversionfile", "url", "b", "u", "p", "commit",
            "dynamicrepolocation", "outputfile", "showvariable", "format", "l", "config",
            "showconfig", "nocache", "nonormalize", "allowshallow", "verbosity", "nofetch"
        };

        return validLegacyOptions.Contains(optionName);
    }
}

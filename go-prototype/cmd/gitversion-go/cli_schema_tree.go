package main

import "github.com/GitTools/GitVersion/go-prototype/internal/output"

func schemaArgumentPointer(a cliSchemaArgument) *cliSchemaArgument { return &a }

func recursiveSchemaOption(option cliSchemaOption) cliSchemaOption {
	option.Recursive = true
	return option
}

func booleanSchemaArgument(name string) *cliSchemaArgument {
	return schemaArgumentPointer(schemaArgument(name, "", 0, 1, false, false).withChoices([]string{"true", "false"}, true, false))
}

// The compatibility CLI tree describes syntax and documentation without executing a command.
func compatibilityCLITree() cliSchemaCommand {
	logFileOption := schemaOption("--log-file", "Path to logfile; specify 'console' to emit to stderr", []string{"-l"}, schemaArgumentPointer(schemaArgument("log-file", "", 1, 1, false, false)), false)
	configOption := schemaOption("--config", "Path to config file (defaults to GitVersion.yml, GitVersion.yaml, .GitVersion.yml or .GitVersion.yaml)", []string{"-c"}, schemaArgumentPointer(schemaArgument("config", "", 1, 1, false, false)), false)
	overrideConfigOption := schemaOption("--override-config", "Overrides GitVersion config values inline (key=value pairs e.g. --override-config workflow=GitHubFlow/v1)", nil, schemaArgumentPointer(schemaArgument("override-config", "", 0, schemaUnbounded, false, false)), false)
	targetPathOption := schemaOption("--target-path", "Same as 'path', but not positional", nil, schemaArgumentPointer(schemaArgument("target-path", "", 1, 1, false, false)), false)
	verbosityOption := schemaOption("--verbosity", "Specifies the amount of information to be displayed (Quiet, Minimal, Normal, Verbose, Diagnostic). Default is Normal", nil, schemaArgumentPointer(schemaArgument("verbosity", "", 1, 1, false, false).withChoices([]string{"Quiet", "Minimal", "Normal", "Verbose", "Diagnostic"}, true, true)), false)
	root := schemaCommand("gitversion", "Use convention to derive a SemVer product version from a GitFlow or GitHub based repository.")
	root.Arguments = []cliSchemaArgument{
		schemaArgument("path", "The directory containing .git. If not defined current directory is used. (Must be first argument)", 0, 1, false, false),
	}
	root.Options = []cliSchemaOption{
		recursiveSchemaOption(schemaOption("--help", "Show help and usage information", []string{"-?", "-h", "/?", "/h"}, nil, false)),
		schemaOption("--version", "Show version information", nil, nil, false),
		schemaOption("--diagnose", "Runs GitVersion with additional diagnostic information.\nAlso needs the '--log-file' argument to specify a logfile or stdout (requires git.exe to be installed)", []string{"-d"}, booleanSchemaArgument("diagnose"), false),
		logFileOption,
		schemaOption("--output", "Determines the output to the console. Can be either 'json', 'file', 'buildserver' or 'dotenv', will default to 'json'", []string{"-o"}, schemaArgumentPointer(schemaArgument("output", "", 0, schemaUnbounded, false, false).withChoices([]string{"BuildServer", "Json", "File", "DotEnv"}, true, true)), true),
		schemaOption("--output-file", "Path to output file. It is used in combination with --output 'file'", nil, schemaArgumentPointer(schemaArgument("output-file", "", 1, 1, false, false)), false),
		schemaOption("--show-variable", "Used in conjunction with --output json, will output just a particular variable.\nE.g. --output json --show-variable SemVer - will output `1.2.3+beta.4`", []string{"-v"}, schemaArgumentPointer(schemaArgument("show-variable", "", 1, 1, false, false).withChoices(output.VariableNames, true, false)), false),
		schemaOption("--format", "Used in conjunction with --output json, will output a format containing version variables.\nSupports C# format strings - see [Format Strings](/docs/reference/custom-formatting) for details.\nE.g. --output json --format {SemVer} - will output `1.2.3+beta.4`\n     --output json --format {Major}.{Minor} - will output `1.2`", []string{"-f"}, schemaArgumentPointer(schemaArgument("format", "", 1, 1, false, false)), false),
		configOption,
		schemaOption("--show-config", "Outputs the effective GitVersion config in yaml format (defaults + custom from GitVersion.yml + runtime overrides with v7 configuration)", nil, booleanSchemaArgument("show-config"), false),
		overrideConfigOption,
		targetPathOption,
		schemaOption("--no-fetch", "Disables 'git fetch' during version calculation. Might cause GitVersion to not calculate your version as expected", nil, booleanSchemaArgument("no-fetch"), false),
		schemaOption("--no-cache", "Bypasses the cache, result will not be written to the cache", nil, booleanSchemaArgument("no-cache"), false),
		schemaOption("--no-normalize", "Disables normalize step on a build server", nil, booleanSchemaArgument("no-normalize"), false),
		schemaOption("--allow-shallow", "Allows GitVersion to run on a shallow clone.\nThis is not recommended, but can be used if you are sure that the shallow clone contains all the information needed to calculate the version.", nil, booleanSchemaArgument("allow-shallow"), false),
		verbosityOption,
		schemaOption("--update-assembly-info", "Will recursively search for all 'AssemblyInfo.cs' files in the git repo and update them", nil, schemaArgumentPointer(schemaArgument("update-assembly-info", "", 0, schemaUnbounded, false, false)), true),
		schemaOption("--update-project-files", "Will recursively search for all project files (.csproj/.vbproj/.fsproj/.sqlproj) in the git repo and update them (only compatible with Sdk projects)", nil, schemaArgumentPointer(schemaArgument("update-project-files", "", 0, schemaUnbounded, false, false)), true),
		schemaOption("--ensure-assembly-info", "If the assembly info file specified with --update-assembly-info is not found, it will be created with AssemblyFileVersion, AssemblyVersion and AssemblyInformationalVersion.\nSupports C#, F#, VB", nil, booleanSchemaArgument("ensure-assembly-info"), false),
		schemaOption("--update-wix-version-file", "All the GitVersion variables are written to 'GitVersion_WixVersion.wxi'", nil, booleanSchemaArgument("update-wix-version-file"), false),
		schemaOption("--url", "Url to remote git repository", nil, schemaArgumentPointer(schemaArgument("url", "", 1, 1, false, false)), false),
		schemaOption("--branch", "Target branch to version (local or with --url). Takes precedence over GIT_BRANCH/Git_Branch, which only supply branch context at HEAD", []string{"-b"}, schemaArgumentPointer(schemaArgument("branch", "", 1, 1, false, false)), false),
		schemaOption("--username", "Username in case authentication is required", []string{"-u"}, schemaArgumentPointer(schemaArgument("username", "", 1, 1, false, false)), false),
		schemaOption("--password", "Password in case authentication is required", []string{"-p"}, schemaArgumentPointer(schemaArgument("password", "", 1, 1, false, false)), false),
		schemaOption("--commit", "The commit id to check. If not specified, the latest available commit on the specified branch will be used", nil, schemaArgumentPointer(schemaArgument("commit", "", 1, 1, false, false)), false),
		schemaOption("--dynamic-repo-location", "By default dynamic repositories will be cloned to %tmp%. Use this option to override", nil, schemaArgumentPointer(schemaArgument("dynamic-repo-location", "", 1, 1, false, false)), false),
		recursiveSchemaOption(schemaOption("--cli-schema", "Writes the complete OpenCLI 0.1 command schema as JSON and exits.", nil, nil, false)),
		recursiveSchemaOption(schemaOption("--telemetry-opt-out", "Disables optional sampled usage telemetry.", nil, booleanSchemaArgument("telemetry-opt-out"), false)),
	}
	config := schemaCommand("config", "Manages GitVersion configuration.")
	migrate := schemaCommand("migrate", "Migrates a v6 configuration document to the v7 structure.")
	migrate.Options = []cliSchemaOption{
		schemaOption("--config", "Path to the configuration file to migrate.", []string{"-c"}, schemaArgumentPointer(schemaArgument("config", "", 1, 1, false, false)), false),
		schemaOption("--output", "Path to write the migrated configuration document.", nil, schemaArgumentPointer(schemaArgument("output", "", 1, 1, false, false)), false),
		schemaOption("--in-place", "Replace the input configuration file.", nil, booleanSchemaArgument("in-place"), false),
		schemaOption("--force", "Allow --output to replace an existing file.", nil, booleanSchemaArgument("force"), false),
	}
	get := schemaCommand("get", "Queries a scalar property of the effective v7 configuration as JSON.")
	get.Arguments = []cliSchemaArgument{
		schemaArgument("property-path", "Dot-separated public v7 property path, e.g. calculation.tag-prefix. Maps and collections are not supported.", 1, 1, false, false),
	}
	get.Options = []cliSchemaOption{
		configOption,
		overrideConfigOption,
		targetPathOption,
		logFileOption,
		verbosityOption,
	}
	validate := schemaCommand("validate", "Validates a configuration file without calculating a version or accessing a Git backend.")
	validate.Options = []cliSchemaOption{
		configOption,
		overrideConfigOption,
		targetPathOption,
		schemaOption("--format", "Emit one versioned JSON validation result (json). Default: diagnostics on stderr and empty stdout.", nil, schemaArgumentPointer(schemaArgument("format", "", 1, 1, false, false).withChoices([]string{"json"}, false, false)), false),
	}
	config.Commands = []cliSchemaCommand{migrate, get, validate}
	root.Commands = []cliSchemaCommand{config}
	return root
}

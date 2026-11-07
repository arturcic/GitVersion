package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/commandline"
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/output"
)

type cliArguments struct {
	TargetPath, LogFilePath, OutputFile, ConfigurationFile, ShowVariable, Format                        string
	TargetURL, TargetBranch, Username, Password, CommitID, ClonePath                                    string
	Verbosity                                                                                           string
	Output                                                                                              map[string]bool
	UpdateAssemblyInfoFileName                                                                          []string
	OverrideConfiguration                                                                               []byte
	IsHelp, IsVersion, NoFetch, NoNormalize, NoCache, AllowShallow, Diag                                bool
	ShowConfiguration, UpdateAssemblyInfo, UpdateProjectFiles, EnsureAssemblyInfo, UpdateWixVersionFile bool
}

var posixOptions = map[string]string{
	"-h": "help", "--help": "help", "-?": "help", "--version": "version",
	"-l": "log-file", "-o": "output", "-d": "diagnose", "-b": "branch",
	"-u": "username", "-p": "password", "-c": "config", "-v": "show-variable", "-f": "format",
}
var argumentOptions = map[string]byte{
	"help": 'b', "version": 'b', "diagnose": 'b', "show-config": 'b',
	"no-fetch": 'b', "no-normalize": 'b', "no-cache": 'b', "allow-shallow": 'b',
	"update-wix-version-file": 'b', "ensure-assembly-info": 'b', "telemetry-opt-out": 'b',
	"log-file": 's', "output-file": 's', "config": 's', "target-path": 's',
	"branch": 's', "username": 's', "password": 's', "commit": 's', "url": 's',
	"dynamic-repo-location": 's', "verbosity": 's', "show-variable": 's', "format": 's',
	"output": 'l', "update-project-files": 'l', "update-assembly-info": 'l', "override-config": 'o',
}

func canonicalOption(token string) string {
	if name := posixOptions[token]; name != "" {
		return name
	}
	if strings.HasPrefix(token, "--") && argumentOptions[token[2:]] != 0 {
		return token[2:]
	}
	return ""
}

func optionToken(token string) bool {
	return strings.HasPrefix(token, "-") && len(token) > 1
}

func argumentError(token string) error {
	return fmt.Errorf("Could not parse command line parameter '%s'.", token)
}

func parseVerbosity(input string) (string, error) {
	for _, option := range []string{"Quiet", "Minimal", "Normal", "Verbose", "Diagnostic"} {
		if strings.EqualFold(input, option) {
			return option, nil
		}
	}
	return "", fmt.Errorf("Could not parse Verbosity value '%s'", input)
}

func optionBoolean(value string) (bool, bool) {
	switch strings.ToLower(value) {
	case "1", "true":
		return true, true
	case "0", "false":
		return false, true
	default:
		return false, false
	}
}

func parseOptionTokens(args []string) (map[string][]string, string, error) {
	options := map[string][]string{}
	positional := ""
	for i := 0; i < len(args); {
		token := commandline.UnquoteText(args[i])
		name, inlineValue, inline := strings.Cut(token, "=")
		key := canonicalOption(name)
		if key == "" {
			if i != 0 || optionToken(token) {
				return nil, "", argumentError(token)
			}
			positional = token
			i++
			continue
		}
		i++
		kind := argumentOptions[key]
		values := []string{}
		if inline {
			values = append(values, inlineValue)
		} else if kind == 'b' {
			if i < len(args) {
				if _, ok := optionBoolean(args[i]); ok {
					values, i = append(values, args[i]), i+1
				}
			}
		} else {
			limit := len(args)
			if kind == 's' || kind == 'o' {
				limit = i + 1
			}
			for i < len(args) && i < limit && !optionToken(args[i]) {
				values, i = append(values, args[i]), i+1
			}
			if kind == 's' && len(values) != 1 {
				return nil, "", argumentError(token)
			}
		}
		if kind == 'b' && len(values) == 0 {
			values = append(values, "true")
		}
		options[key] = append(options[key], values...)
	}

	return options, positional, nil
}

func parseCLIArguments(args []string, cwd string, lookup func(string) string) (cliArguments, error) {
	var result cliArguments
	options, positional, err := parseOptionTokens(args)
	if err != nil {
		return result, err
	}

	last := func(name string) string {
		values := options[name]
		if len(values) == 0 {
			return ""
		}
		return commandline.UnquoteText(values[len(values)-1])
	}
	enabled := func(name string) bool {
		on, _ := optionBoolean(last(name))
		return on
	}
	if enabled("help") || enabled("version") {
		result.IsHelp, result.IsVersion = enabled("help"), enabled("version")
		return result, nil
	}
	result.TargetPath = last("target-path")
	if result.TargetPath == "" {
		result.TargetPath = positional
	}
	if result.TargetPath == "" {
		result.TargetPath = cwd
	}
	if trimmed := strings.TrimRight(result.TargetPath, "/\\"); trimmed != "" {
		result.TargetPath = trimmed
	}
	result.LogFilePath, result.OutputFile, result.ConfigurationFile = last("log-file"), last("output-file"), last("config")
	result.TargetURL, result.TargetBranch, result.CommitID, result.ClonePath = last("url"), last("branch"), last("commit"), last("dynamic-repo-location")
	result.Username, result.Password = last("username"), last("password")
	if _, specified := options["username"]; !specified {
		result.Username = lookup("GITVERSION_REMOTE_USERNAME")
	}
	if _, specified := options["password"]; !specified {
		result.Password = lookup("GITVERSION_REMOTE_PASSWORD")
	}
	result.NoFetch, result.NoNormalize, result.NoCache = enabled("no-fetch"), enabled("no-normalize"), enabled("no-cache")
	result.AllowShallow, result.Diag, result.ShowConfiguration = enabled("allow-shallow"), enabled("diagnose"), enabled("show-config")
	result.EnsureAssemblyInfo, result.UpdateWixVersionFile = enabled("ensure-assembly-info"), enabled("update-wix-version-file")
	if value := last("verbosity"); value != "" {
		var err error
		result.Verbosity, err = parseVerbosity(value)
		if err != nil {
			return result, err
		}
	}
	result.Output = map[string]bool{}
	for _, value := range options["output"] {
		value = strings.ToLower(commandline.UnquoteText(value))
		switch value {
		case "json", "file", "buildserver", "dotenv":
			result.Output[value] = true
		default:
			return result, fmt.Errorf("Value '%s' cannot be parsed as output type, please use 'json', 'file', 'buildserver' or 'dotenv'", value)
		}
	}
	if len(result.Output) == 0 {
		result.Output["json"] = true
	}
	if result.Output["file"] && result.OutputFile == "" {
		result.OutputFile = "GitVersion.json"
	}
	if value := strings.ReplaceAll(last("show-variable"), "'", ""); value != "" {
		for _, variable := range output.VariableNames {
			if strings.EqualFold(variable, value) {
				result.ShowVariable = variable
				break
			}
		}
		if result.ShowVariable == "" {
			return result, fmt.Errorf("--show-variable requires a valid version variable")
		}
	}
	result.Format = last("format")
	if result.Format != "" && !slices.ContainsFunc(output.VariableNames, func(name string) bool {
		return strings.Contains(strings.ToLower(result.Format), strings.ToLower(name))
	}) {
		return result, fmt.Errorf("Format requires a valid format string. Available variables are: %s", strings.Join(output.VariableNames, ", "))
	}
	if err := mapAssemblyArguments(&result, options); err != nil {
		return result, err
	}
	versionValue := lookup("GITVERSION_CONFIGURATION_VERSION")
	if values := options["override-config"]; len(values) > 0 {
		version, err := config.SelectConfigurationVersion(&versionValue)
		if err != nil {
			return result, err
		}
		result.OverrideConfiguration, err = config.ParseCLIOverrides(values, version, "--override-config")
		if err != nil {
			return result, err
		}
	}
	if result.ConfigurationFile != "" {
		path, err := filepath.Abs(resolveMigrationPath(result.TargetPath, result.ConfigurationFile))
		if err != nil {
			return result, err
		}
		if !(config.OSConfigurationFS{}).FileExists(path) {
			return result, fmt.Errorf("Could not find config file at '%s'", path)
		}
		result.ConfigurationFile = path
	}
	return result, nil
}

func directoryExists(path string) bool {
	return (config.OSConfigurationFS{}).DirectoryExists(path)
}

func mapAssemblyArguments(result *cliArguments, options map[string][]string) error {
	assembly, assemblySet := options["update-assembly-info"]
	project, projectSet := options["update-project-files"]
	result.UpdateAssemblyInfo, result.UpdateProjectFiles = assemblySet, projectSet
	if len(assembly) == 1 {
		if on, ok := optionBoolean(assembly[0]); ok && !on {
			result.UpdateAssemblyInfo = false
			assembly = nil
		}
	}
	if assemblySet && projectSet {
		return fmt.Errorf("Cannot specify both --update-assembly-info and --update-project-files in the same run. Please rerun GitVersion with only one parameter")
	}
	if projectSet && result.EnsureAssemblyInfo {
		return fmt.Errorf("Cannot specify --ensure-assembly-info with --update-project-files: please ensure your project file exists before attempting to update it")
	}
	var patterns []string
	for _, value := range append(slices.Clone(assembly), project...) {
		if on, valid := optionBoolean(value); !valid || !on {
			name := commandline.UnquoteText(value)
			if !slices.Contains(patterns, name) {
				patterns = append(patterns, name)
			}
		}
	}
	if result.EnsureAssemblyInfo {
		if len(patterns) > 1 {
			return fmt.Errorf("Can't specify multiple assembly info files when using --ensure-assembly-info, either use a single assembly info file or do not specify --ensure-assembly-info and create assembly info files manually")
		}
		result.UpdateAssemblyInfoFileName = patterns
		return nil
	}
	for _, pattern := range patterns {
		matches, err := resolveAssemblyFiles(result.TargetPath, pattern)
		if err != nil {
			return err
		}
		for _, path := range matches {
			if !slices.Contains(result.UpdateAssemblyInfoFileName, path) {
				result.UpdateAssemblyInfoFileName = append(result.UpdateAssemblyInfoFileName, path)
			}
		}
	}
	return nil
}

func resolveAssemblyFiles(dir, pattern string) ([]string, error) {
	pattern = filepath.FromSlash(strings.ReplaceAll(pattern, "\\", "/"))
	if !strings.Contains(pattern, "**") {
		matches, err := filepath.Glob(resolveMigrationPath(dir, pattern))
		if err != nil {
			return nil, err
		}
		return slices.DeleteFunc(matches, func(path string) bool { return !(config.OSConfigurationFS{}).FileExists(path) }), nil
	}
	root := dir
	if filepath.IsAbs(pattern) {
		root, pattern, _ = strings.Cut(pattern, "**")
		pattern = "**" + pattern
	}
	if !directoryExists(root) {
		return nil, nil
	}
	patternParts := strings.Split(filepath.ToSlash(pattern), "/")
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if matchAssemblyGlob(patternParts, strings.Split(filepath.ToSlash(relative), "/")) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

func matchAssemblyGlob(pattern, path []string) bool {
	if len(pattern) == 0 {
		return len(path) == 0
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(path); i++ {
			if matchAssemblyGlob(pattern[1:], path[i:]) {
				return true
			}
		}
		return false
	}
	if len(path) == 0 {
		return false
	}
	match, _ := filepath.Match(pattern[0], path[0])
	return match && matchAssemblyGlob(pattern[1:], path[1:])
}

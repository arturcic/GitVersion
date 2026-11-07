package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/buildagent"
	"github.com/GitTools/GitVersion/go-prototype/internal/calculate"
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/formatting"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/output"
)

func selectedArgumentParser() (string, error) {
	if _, retired := os.LookupEnv("GITVERSION_USE_V6_ARGUMENT_PARSER"); retired {
		return "", fmt.Errorf("GITVERSION_USE_V6_ARGUMENT_PARSER has been removed. Unset it and use GITVERSION_ARGUMENT_PARSER_VERSION=v6 for the temporary legacy parser, or GITVERSION_ARGUMENT_PARSER_VERSION=v7 (the default).")
	}
	parser := strings.ToLower(strings.TrimSpace(os.Getenv("GITVERSION_ARGUMENT_PARSER_VERSION")))
	if parser != "" && parser != "v6" && parser != "v7" {
		return "", fmt.Errorf("Unrecognized GITVERSION_ARGUMENT_PARSER_VERSION value '%s'. Valid values are 'v6' and 'v7'.", parser)
	}
	return parser, nil
}

func runRootCLI(args []string, out, diagnostics io.Writer) error {
	parser, err := selectedArgumentParser()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	options, err := parseCLIArguments(args, parser == "v6", cwd, os.Getenv)
	if err != nil {
		return err
	}
	if options.IsHelp {
		_, err := fmt.Fprintln(out, "Usage: gitversion-go [<path>] [--config PATH] [--output json|file] [--show-variable NAME] [--format TEMPLATE]\n       gitversion-go [<path>] config get|validate|migrate")
		return err
	}
	if options.IsVersion {
		_, err := fmt.Fprintln(out, "gitversion-go prototype")
		return err
	}
	if options.ShowConfiguration {
		version, err := selectedConfigurationVersion()
		if err != nil {
			return err
		}
		locator := config.FileLocator{}
		if options.ConfigurationFile != "" {
			locator.Custom = &options.ConfigurationFile
		}
		provider := config.Provider{Locator: locator, Version: version}
		directory, err := filepath.Abs(options.TargetPath)
		if err != nil {
			return err
		}
		return outputConfigurationQuery(version, directory, options.TargetURL, nil, options.OverrideConfiguration, &provider, out)
	}
	if options.TargetURL != "" || options.TargetBranch != "" || options.ClonePath != "" {
		return fmt.Errorf("remote and branch targeting are pending")
	}
	if options.Diag {
		return fmt.Errorf("diagnostic mode is pending")
	}
	repoOptions := gitrepo.OpenOptions{AllowShallow: options.AllowShallow, AllowDirty: true}
	if !options.NoNormalize {
		agent := buildagent.Resolve(options.TargetPath, os.LookupEnv)
		repoOptions.NormalizeDetached = agent.Name != "LocalBuild"
		if branch := agent.CurrentBranch(); branch != nil {
			repoOptions.BuildBranch = *branch
		}
		if tag := agent.CurrentTag(); tag != nil {
			repoOptions.BuildTag = *tag
		}
	}
	return runRootCalculation(options, repoOptions, out, diagnostics)
}

type versionStages struct {
	discover  func(string) (string, error)
	calculate func(string) (calculationExecution, error)
	output    func(calculationExecution) error
	assembly  func(map[string]any) error
	wix       func(map[string]any) error
}

func runVersionStages(directory string, stages versionStages) error {
	root, err := stages.discover(directory)
	if err != nil {
		return calculationFailure{err}
	}
	result, err := stages.calculate(root)
	if err != nil {
		return calculationFailure{err}
	}
	if err := stages.output(result); err != nil {
		return err
	}
	if err := stages.assembly(result.values); err != nil {
		return err
	}
	return stages.wix(result.values)
}

type calculationFailure struct{ error }

func (failure calculationFailure) Unwrap() error { return failure.error }

func runRootCalculation(options cliArguments, repoOptions gitrepo.OpenOptions, out, diagnostics io.Writer) error {
	rawVersion := os.Getenv("GITVERSION_CONFIGURATION_VERSION")
	version, err := resolveConfigurationVersion(rawVersion)
	if err != nil {
		return err
	}
	return runVersionStages(options.TargetPath, versionStages{
		discover: discoverCalculationRoot,
		calculate: func(root string) (calculationExecution, error) {
			return calculateDiscovered(options, root, rawVersion, version, diagnostics, repoOptions, !options.NoCache, config.LoadCalculationConfig)
		},
		output: func(result calculationExecution) error {
			if err := writeCalculationLog(options, result, diagnostics); err != nil {
				return err
			}
			return writeRootOutput(options, result.values, result.updateBuildNumber, out)
		},
		assembly: func(values map[string]any) error {
			switch {
			case options.UpdateProjectFiles:
				return updateProjectFiles(options.TargetPath, options.UpdateAssemblyInfoFileName, values, diagnostics)
			case options.UpdateAssemblyInfo:
				return updateAssemblyInfo(options.TargetPath, options.UpdateAssemblyInfoFileName, options.EnsureAssemblyInfo, values, diagnostics)
			default:
				return nil
			}
		},
		wix: func(values map[string]any) error {
			if options.UpdateWixVersionFile {
				return updateWixVersionFile(options.TargetPath, values)
			}
			return nil
		},
	})
}

func writeRootOutput(options cliArguments, values map[string]any, updateBuildNumber bool, out io.Writer) error {
	if options.Output["buildserver"] {
		agent := buildagent.Resolve(options.TargetPath, os.LookupEnv)
		if err := agent.WriteIntegration(context.Background(), out, values, updateBuildNumber); err != nil {
			return err
		}
	}
	if options.Output["dotenv"] {
		return buildagent.WriteDotEnv(out, values)
	}
	if options.Output["file"] {
		data, err := output.MarshalVariables(values)
		if err != nil {
			return err
		}
		path := resolveMigrationPath(options.TargetPath, options.OutputFile)
		info, err := os.Lstat(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := writeFileAtomically(path, data, info, true); err != nil {
			return err
		}
	}
	if !options.Output["json"] {
		return nil
	}
	if options.ShowVariable != "" {
		value := values[options.ShowVariable]
		if value == nil {
			value = ""
		}
		_, err := fmt.Fprintln(out, value)
		return err
	}
	if options.Format != "" {
		text, err := formatting.FormatWith(options.Format, values, os.LookupEnv)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, text)
		return err
	}
	data, err := output.MarshalVariables(values)
	if err != nil {
		return err
	}
	_, err = out.Write(append(data, '\n'))
	return err
}

type calculationExecution struct {
	values            map[string]any
	trace             calculate.Trace
	updateBuildNumber bool
	cacheMessages     []string
	cacheHit          bool
}

func calculateCLI(path, custom, logFile, commit string, overrides []byte, diagnostics io.Writer, repoOptions gitrepo.OpenOptions) (map[string]any, calculate.Trace, error) {
	options := cliArguments{
		TargetPath: path, ConfigurationFile: custom, LogFilePath: logFile,
		CommitID: commit, OverrideConfiguration: overrides,
	}
	result, err := executeCalculation(options, diagnostics, repoOptions, false, config.LoadCalculationConfig)
	return result.values, result.trace, err
}

func executeCalculation(options cliArguments, diagnostics io.Writer, repoOptions gitrepo.OpenOptions, cache bool,
	loadConfig func(string, string, []byte, string, func(string)) (config.Config, []byte, error),
) (calculationExecution, error) {
	var result calculationExecution
	rawVersion := os.Getenv("GITVERSION_CONFIGURATION_VERSION")
	version, err := resolveConfigurationVersion(rawVersion)
	if err != nil {
		return result, err
	}
	root, err := discoverCalculationRoot(options.TargetPath)
	if err != nil {
		return result, err
	}
	return calculateDiscovered(options, root, rawVersion, version, diagnostics, repoOptions, cache, loadConfig)
}

func discoverCalculationRoot(path string) (string, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) || (err == nil && !info.IsDir()) {
		return "", fmt.Errorf("The working directory '%s' does not exist.", path)
	}
	if err != nil {
		return "", err
	}
	return gitrepo.WorkingTreeRoot(path)
}

func calculateDiscovered(options cliArguments, root, rawVersion, version string, diagnostics io.Writer, repoOptions gitrepo.OpenOptions, cache bool,
	loadConfig func(string, string, []byte, string, func(string)) (config.Config, []byte, error),
) (calculationExecution, error) {
	var result calculationExecution
	path, custom := options.TargetPath, options.ConfigurationFile
	configDirectory := path
	if custom == "" {
		locator := config.FileLocator{}
		file, err := locator.Get(&path)
		if err != nil {
			return result, err
		}
		if file == nil {
			configDirectory = root
		}
	}
	var warning string
	cfg, configuration, err := loadConfig(configDirectory, custom, options.OverrideConfiguration, rawVersion,
		func(message string) { warning = message })
	if err != nil {
		if strings.Contains(err.Error(), "uses the legacy v6 configuration structure") {
			err = fmt.Errorf("An error occurred:\n%w", err)
		}
		return result, err
	}
	if warning != "" {
		fmt.Fprintln(diagnostics, warning)
	}
	result.updateBuildNumber = cfg.UpdateBuildNumber
	repo, err := gitrepo.OpenForCalculation(root, repoOptions)
	if err != nil {
		return result, err
	}
	var cachePath string
	if cache && len(options.OverrideConfiguration) == 0 {
		cachePath, err = calculationCachePath(repo, version, options.CommitID, configuration)
		if err == nil {
			result.cacheMessages = append(result.cacheMessages, "Loading version variables from disk cache file "+cachePath)
			result.values, err = loadCalculationCache(cachePath)
			switch {
			case err == nil:
				result.cacheHit = true
			case os.IsNotExist(err):
				result.cacheMessages = append(result.cacheMessages, "Cache file "+cachePath+" not found.")
			default:
				result.cacheMessages = append(result.cacheMessages, "Unable to read cache file "+cachePath+".")
			}
		}
	}
	if !result.cacheHit {
		result.values, result.trace, err = calculate.CalculatePrepared(repo, cfg, options.CommitID)
		if err != nil {
			return result, err
		}
		if cachePath != "" {
			// Publish only while the repository reference view still matches the key.
			current, err := calculationCachePath(repo, version, options.CommitID, configuration)
			if err == nil && current == cachePath {
				if err := storeCalculationCache(cachePath, result.values); err != nil {
					result.cacheMessages = append(result.cacheMessages, "Unable to write cache file "+cachePath+".")
				}
			}
		}
	}
	logFile := options.LogFilePath
	if warning != "" && logFile != "" && logFile != "console" {
		file, err := os.OpenFile(resolveMigrationPath(path, logFile), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			return result, err
		}
		_, err = fmt.Fprintln(file, warning)
		if err := errors.Join(err, file.Close()); err != nil {
			return result, err
		}
	}
	return result, nil
}

func updateBuildNumberSetting(directory, custom string, overrides []byte) (bool, error) {
	version, err := selectedConfigurationVersion()
	if err != nil {
		return false, err
	}
	locator := config.FileLocator{}
	if custom != "" {
		locator.Custom = &custom
	}
	provider := config.Provider{Locator: locator, Version: version}
	resolved, err := provider.Provide(&directory, overrides)
	if err != nil {
		return false, err
	}
	value, err := resolved.SerializeProperty("output.update-build-number", "v7")
	if err != nil {
		return false, err
	}
	return string(value) == "true", nil
}

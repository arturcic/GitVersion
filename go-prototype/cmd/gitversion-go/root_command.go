package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/buildagent"
	"github.com/GitTools/GitVersion/go-prototype/internal/calculate"
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/formatting"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/output"
)

func runRootCLI(args []string, out, diagnostics io.Writer) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	options, err := parseCLIArguments(args, cwd, os.Getenv)
	if err != nil {
		return fmt.Errorf("An error occurred:\n%w", err)
	}
	if options.IsHelp {
		return writeCLIHelp(out, []cliSchemaCommand{compatibilityCLITree()})
	}
	if options.IsVersion {
		_, err := fmt.Fprintln(out, cliInformationalVersion())
		return err
	}
	version, err := selectedConfigurationVersion()
	if err != nil {
		return err
	}
	agent := buildagent.Resolve(options.TargetPath, os.LookupEnv)
	log := newCLILog(options.LogFilePath, options.Verbosity, options.Output["buildserver"], diagnostics)
	defer log.close()
	log.initialize(options.TargetPath, version, agent, options.Diag)
	if options.ShowConfiguration {
		locator := config.FileLocator{Log: log.configLookup}
		if options.ConfigurationFile != "" {
			locator.Custom = &options.ConfigurationFile
		}
		provider := config.Provider{Locator: locator, Version: os.Getenv("GITVERSION_CONFIGURATION_VERSION")}
		directory, err := filepath.Abs(options.TargetPath)
		if err != nil {
			return err
		}
		return outputConfigurationQuery(version, directory, options.TargetURL, nil, options.OverrideConfiguration, &provider, out)
	}
	options.NoCache = options.NoCache || options.Diag
	target, contextBranch, err := branchSelection(options.TargetBranch, os.Getenv)
	if err != nil {
		return calculationFailure{err}
	}
	repoOptions := gitrepo.OpenOptions{
		AllowShallow: options.AllowShallow, AllowDirty: true,
		TargetBranch: target, ContextBranch: contextBranch,
	}
	if options.TargetURL != "" || !options.NoNormalize {
		repoOptions.NormalizeDetached = agent.Name != "LocalBuild"
		if branch := agent.CurrentBranch(); branch != nil {
			repoOptions.BuildBranch = *branch
		}
		if tag := agent.CurrentTag(); tag != nil {
			repoOptions.BuildTag = *tag
		}
	}
	if options.TargetURL != "" && target == "" && contextBranch == "" {
		repoOptions.TargetBranch = repoOptions.BuildBranch
		if strings.TrimSpace(repoOptions.TargetBranch) == "" {
			return calculationFailure{fmt.Errorf("Dynamic Git repositories must have a target branch (--branch)")}
		}
	}
	return runRootCalculation(options, repoOptions, agent, version, out, diagnostics, log)
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

type renderedCommandError struct{ error }

func (failure renderedCommandError) Unwrap() error { return failure.error }

func runRootCalculation(options cliArguments, repoOptions gitrepo.OpenOptions, agent *buildagent.Agent, version string, out, diagnostics io.Writer, log *cliLog) error {
	rawVersion := os.Getenv("GITVERSION_CONFIGURATION_VERSION")
	credentials := gitrepo.Credentials{Username: options.Username, Password: options.Password}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	repoOptions.PreparationContext, repoOptions.RemoteCredentials = ctx, credentials
	var discovered bool
	err := runVersionStages(options.TargetPath, versionStages{
		discover: func(directory string) (string, error) {
			if options.TargetURL != "" {
				return gitrepo.PrepareClone(ctx, options.TargetURL, options.ClonePath, repoOptions.ContextBranch != "", credentials)
			}
			if !(config.OSConfigurationFS{}).DirectoryExists(directory) {
				return "", errors.New("Cannot find the .git directory")
			}
			return discoverCalculationRoot(directory)
		},
		calculate: func(root string) (calculationExecution, error) {
			discovered = true
			project, dotGit, err := gitrepo.CalculationPaths(root)
			if err != nil {
				return calculationExecution{}, err
			}
			log.write(logInfo, "Project root is: "+project+string(filepath.Separator))
			log.write(logInfo, "DotGit directory is: "+dotGit)
			branch, contextual := repoOptions.TargetBranch, repoOptions.ContextBranch
			if branch == "" {
				branch = "null"
			}
			if contextual == "" {
				contextual = "null"
			}
			log.write(logInfo, fmt.Sprintf("Branch input: %s; contextual override: %s", branch, contextual))
			if options.TargetURL != "" || (!options.NoNormalize && agent.Name != "LocalBuild") {
				fetch := !options.NoFetch && !agent.PreventFetch()
				cleanup := agent.ShouldCleanUpRemotes() && options.TargetURL == ""
				remote, err := gitrepo.PrepareRemote(ctx, root, fetch, cleanup, credentials)
				if err != nil {
					return calculationExecution{}, err
				}
				if cleanup {
					repoOptions.RetainRemote = remote
				}
				if repoOptions.ContextBranch == "" {
					repoOptions.NormalizationRemote = remote
				} else if fetch {
					repoOptions.FetchedRemote = remote
				}
			}
			locator := config.FileLocator{Log: log.configLookup}
			return calculateDiscovered(options, root, rawVersion, version, repoOptions, !options.NoCache, locator.LoadCalculationConfig, log)
		},
		output: func(result calculationExecution) error {
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
	if err != nil {
		var failure calculationFailure
		if !discovered && options.TargetURL == "" && errors.As(err, &failure) {
			return failure.error
		}
		message := strings.TrimPrefix(err.Error(), "An error occurred:\n")
		message = logURLPassword.ReplaceAllString(message, "${1}*******@")
		if options.Password != "" {
			message = strings.ReplaceAll(message, options.Password, "*******")
		}
		if errors.Is(err, gitrepo.ErrNoCommits) {
			log.write(logError, "An unexpected error occurred:\n"+message)
			log.write(logInfo, gitGraphHint)
		} else {
			if errors.As(err, &failure) || strings.HasPrefix(err.Error(), "An error occurred:\n") {
				fmt.Fprintln(diagnostics, "An error occurred:")
			}
			fmt.Fprintln(diagnostics, message)
			log.write(logError, "An error occurred:\n"+message)
		}
		return renderedCommandError{err}
	}
	return err
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
	log := newCLILog(options.LogFilePath, options.Verbosity, false, diagnostics)
	defer log.close()
	return calculateDiscovered(options, root, rawVersion, version, repoOptions, cache, loadConfig, log)
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

func calculateDiscovered(options cliArguments, root, rawVersion, version string, repoOptions gitrepo.OpenOptions, cache bool,
	loadConfig func(string, string, []byte, string, func(string)) (config.Config, []byte, error), log *cliLog,
) (calculationExecution, error) {
	var result calculationExecution
	path, custom := options.TargetPath, options.ConfigurationFile
	configDirectory := path
	if custom == "" && options.TargetURL == "" {
		locator := config.FileLocator{}
		if log != nil {
			locator.Log = log.configLookup
		}
		file, err := locator.Get(&path)
		if err != nil {
			return result, err
		}
		if file == nil {
			configDirectory = root
		}
	}
	cfg, configuration, err := loadConfig(configDirectory, custom, options.OverrideConfiguration, rawVersion, func(message string) {
		log.write(logWarning, message)
	})
	if err != nil {
		if strings.Contains(err.Error(), "uses the legacy v6 configuration structure") {
			err = fmt.Errorf("An error occurred:\nThe configuration file uses the legacy v6 configuration structure, but GITVERSION_CONFIGURATION_VERSION=v7 is selected. Run 'gitversion config migrate' or set GITVERSION_CONFIGURATION_VERSION=v6 temporarily.")
		}
		return result, err
	}
	result.updateBuildNumber = cfg.UpdateBuildNumber
	repoOptions.IgnoreBranch = cfg.Ignore.BranchIgnored
	repo, err := gitrepo.OpenForCalculation(root, repoOptions)
	if err != nil {
		if errors.Is(err, gitrepo.ErrNoCommits) {
			log.write(logInfo, "Using latest commit on specified branch")
		}
		return result, err
	}
	var cachePath string
	if cache && len(options.OverrideConfiguration) == 0 {
		cachePath, err = calculationCachePath(repo, version, options.CommitID, configuration)
		if err == nil {
			started := time.Now()
			title := "Loading version variables from disk cache file " + cachePath
			log.write(logInfo, "┌─── Begin: "+title)
			result.values, err = loadCalculationCache(cachePath)
			switch {
			case err == nil:
				result.cacheHit = true
			case os.IsNotExist(err):
				log.write(logInfo, "│   Cache file "+cachePath+" not found.")
			default:
				log.write(logInfo, "│   Unable to read cache file "+cachePath+".")
			}
			log.write(logInfo, fmt.Sprintf("└─── End: %s (Took: %.2fms)", title, float64(time.Since(started))/float64(time.Millisecond)))
		}
	}
	if !result.cacheHit {
		events := calculate.CalculationLog{Warn: func(message string) { log.write(logWarning, message) }}
		if log.enabled(logInfo) {
			events.Info = func(message string) { log.write(logInfo, message) }
			events.TaggedVersionLookup = log.taggedVersionLookup
		}
		result.values, result.trace, err = calculate.CalculatePreparedWithLog(repo, cfg, options.CommitID, events)
		if err != nil {
			return result, err
		}
		if cachePath != "" {
			// Publish only while the repository reference view still matches the key.
			current, err := calculationCachePath(repo, version, options.CommitID, configuration)
			if err == nil && current == cachePath {
				if err := storeCalculationCache(cachePath, result.values); err != nil {
					log.write(logInfo, "Unable to write cache file "+cachePath+".")
				}
			}
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

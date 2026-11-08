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
	"github.com/GitTools/GitVersion/go-prototype/internal/logscope"
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
	normalize := options.TargetURL != "" || (!options.NoNormalize && agent.Name != "LocalBuild")
	branch := repoOptions.TargetBranch
	if branch == "" && normalize && repoOptions.ContextBranch == "" {
		branch = repoOptions.BuildBranch
	}
	logRepository := func(project, dotGit string) {
		log.write(logInfo, "Project root is: "+project)
		log.write(logInfo, "DotGit directory is: "+dotGit)
		target, contextual := branch, strings.TrimPrefix(repoOptions.ContextBranch, "refs/heads/")
		if target == "" {
			target = "null"
		}
		if contextual == "" {
			contextual = "null"
		}
		log.write(logInfo, fmt.Sprintf("Branch input: %s; contextual override: %s", target, contextual))
	}
	err := runVersionStages(options.TargetPath, versionStages{
		discover: func(directory string) (string, error) {
			if options.TargetURL != "" {
				return gitrepo.PrepareCloneWithLog(ctx, options.TargetURL, options.ClonePath, repoOptions.ContextBranch != "", credentials,
					func(source, destination string, existing bool) func(error) {
						logRepository(directory, destination)
						info, end := logscope.Begin(log.info(), fmt.Sprintf("Creating dynamic repository at '%s'", destination))
						if info == nil {
							return func(error) {}
						}
						if existing {
							info("Git repository already exists")
							return func(error) { end() }
						}
						cloneInfo, endClone := logscope.Begin(info, fmt.Sprintf("Cloning repository from url '%s'", source))
						return func(err error) {
							if err == nil {
								cloneInfo(fmt.Sprintf("Cloned repository '%s' into '%s'", source, destination))
							}
							endClone()
							end()
						}
					})
			}
			if !(config.OSConfigurationFS{}).DirectoryExists(directory) {
				return "", errors.New("Cannot find the .git directory")
			}
			return discoverCalculationRoot(directory)
		},
		calculate: func(root string) (calculationExecution, error) {
			discovered = true
			if options.TargetURL == "" && log.enabled(logInfo) {
				project, dotGit, err := gitrepo.CalculationPaths(root)
				if err != nil {
					return calculationExecution{}, err
				}
				logRepository(project+string(filepath.Separator), dotGit)
			}
			var preparation repositoryPreparation
			if normalize {
				preparation = repositoryPreparation{
					branch: branch,
					prepare: func(repoOptions *gitrepo.OpenOptions, info func(string)) error {
						fetch := !options.NoFetch && !agent.PreventFetch()
						cleanup := agent.ShouldCleanUpRemotes() && options.TargetURL == ""
						remote, err := gitrepo.PrepareRemoteWithLog(ctx, root, fetch, cleanup, credentials, info)
						if err != nil {
							return err
						}
						if cleanup {
							repoOptions.RetainRemote = remote
						}
						if repoOptions.ContextBranch == "" {
							repoOptions.NormalizationRemote = remote
						} else if fetch {
							repoOptions.FetchedRemote = remote
						}
						return nil
					},
				}
			}
			return calculateDiscovered(options, root, rawVersion, version, repoOptions, !options.NoCache, nil, log, preparation)
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
		var unexpected gitrepo.UnexpectedPreparationError
		if errors.Is(err, gitrepo.ErrNoCommits) || errors.As(err, &unexpected) {
			log.write(logError, "An unexpected error occurred:\n"+message)
			log.write(logInfo, string(nativeText([]byte(gitGraphHint))))
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
		if err := writeFileAtomically(path, nativeText(data), info, true); err != nil {
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
	return calculateDiscovered(options, root, rawVersion, version, repoOptions, cache, loadConfig, log, repositoryPreparation{})
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

type repositoryPreparation struct {
	branch  string
	prepare func(*gitrepo.OpenOptions, func(string)) error
}

func calculateDiscovered(options cliArguments, root, rawVersion, version string, repoOptions gitrepo.OpenOptions, cache bool,
	loadConfig func(string, string, []byte, string, func(string)) (config.Config, []byte, error), log *cliLog, preparation repositoryPreparation,
) (calculationExecution, error) {
	var result calculationExecution
	cache = cache && len(options.OverrideConfiguration) == 0
	var pendingConfig []string
	emitConfig := log.configLookup
	var prepareLog *cliLog
	var endPreparation func()
	if preparation.prepare != nil {
		_, endPreparation = logscope.Begin(log.info(), fmt.Sprintf("Normalizing git directory for branch '%s'", preparation.branch))
		defer func() {
			if endPreparation != nil {
				endPreparation()
			}
		}()
		prepareLog = log.indented()
		emitConfig = prepareLog.configLookup
	} else if cache {
		emitConfig = func(message string) { pendingConfig = append(pendingConfig, message) }
	}
	flushConfig := func() {
		for _, message := range pendingConfig {
			log.configLookup(message)
		}
		pendingConfig = nil
	}
	defer flushConfig()
	if loadConfig == nil {
		locator := config.FileLocator{Log: emitConfig}
		loadConfig = locator.LoadCalculationConfig
	}
	path, custom := options.TargetPath, options.ConfigurationFile
	configDirectory := path
	if custom == "" && options.TargetURL == "" {
		var search []string
		locator := config.FileLocator{Log: func(message string) { search = append(search, message) }}
		file, err := locator.Get(&path)
		if err != nil {
			return result, err
		}
		if file == nil {
			for _, message := range search {
				emitConfig(message)
			}
			configDirectory = root
		}
	}
	cfg, configuration, err := loadConfig(configDirectory, custom, options.OverrideConfiguration, rawVersion, func(message string) {
		emitConfig(message)
	})
	if err != nil {
		if strings.Contains(err.Error(), "uses the legacy v6 configuration structure") {
			err = fmt.Errorf("An error occurred:\nThe configuration file uses the legacy v6 configuration structure, but GITVERSION_CONFIGURATION_VERSION=v7 is selected. Run 'gitversion config migrate' or set GITVERSION_CONFIGURATION_VERSION=v6 temporarily.")
		}
		return result, err
	}
	result.updateBuildNumber = cfg.UpdateBuildNumber
	repoOptions.IgnoreBranch = cfg.Ignore.BranchIgnored
	if prepareLog != nil {
		if err := preparation.prepare(&repoOptions, prepareLog.info()); err != nil {
			return result, err
		}
	}
	repo, err := gitrepo.OpenForCalculation(root, repoOptions)
	if err != nil {
		if errors.Is(err, gitrepo.ErrNoCommits) {
			log.write(logInfo, "Using latest commit on specified branch")
		}
		return result, err
	}
	var rawRefs, projectedRefs []gitrepo.Reference
	if prepareLog != nil {
		if repo.NormalizationRemote() != "" && prepareLog.enabled(logInfo) {
			rawRefs, err = repo.References()
			if err != nil {
				return result, err
			}
			projectedRefs, err = repo.TraceNormalizedReferences(rawRefs, cfg.Ignore.BranchIgnored, prepareLog.info())
			if err != nil {
				return result, err
			}
		}
		if len(repo.ShallowCommits()) > 0 {
			prepareLog.write(logInfo, "Repository is a shallow clone. GitVersion will continue, but it is recommended to use a full clone for accurate versioning.")
		}
		if repoOptions.ContextBranch == "" && prepareLog.enabled(logInfo) {
			if head, readErr := gitrepo.ReadCheckoutHead(root); readErr == nil {
				prepareLog.write(logInfo, head)
			}
			branch, sha := repo.Head()
			if repo.NormalizationRemote() != "" && strings.HasPrefix(branch, "refs/heads/") && branch != repo.CheckedOutBranch() {
				prepareLog.write(logInfo, fmt.Sprintf("Selecting local branch '%s' at %.7s (calculation view).", branch, sha))
			}
		}
		endPreparation()
		endPreparation = nil
	}
	var cachePath string
	if cache {
		cachePath, err = calculationCachePath(repo, version, options.CommitID, configuration, rawRefs)
		if err == nil {
			info, end := logscope.Begin(log.info(), "Loading version variables from disk cache file "+cachePath)
			result.values, err = loadCalculationCache(cachePath)
			switch {
			case err == nil:
				result.cacheHit = true
			case os.IsNotExist(err):
				if info != nil {
					info("Cache file " + cachePath + " not found.")
				}
			default:
				log.write(logError, "│   Unable to read cache file "+cachePath+", replacing it after calculation.")
			}
			end()
		}
	}
	flushConfig()
	if !result.cacheHit {
		events := calculate.CalculationLog{Warn: func(message string) { log.write(logWarning, message) }}
		if log.enabled(logInfo) {
			events.Info = func(message string) { log.write(logInfo, message) }
			events.TaggedVersionLookup = log.taggedVersionLookup
		}
		result.values, result.trace, err = calculate.CalculatePreparedWithLog(repo, cfg, options.CommitID, events, projectedRefs)
		if err != nil {
			return result, err
		}
		if cachePath != "" {
			// Publish only while the repository reference view still matches the key.
			current, err := calculationCachePath(repo, version, options.CommitID, configuration, nil)
			if err == nil && current == cachePath {
				_, end := logscope.Begin(log.info(), "Write version variables to cache file "+cachePath)
				if err := storeCalculationCache(cachePath, result.values); err != nil {
					log.write(logError, fmt.Sprintf("│   Unable to write cache file %s: %v", cachePath, err))
				}
				end()
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

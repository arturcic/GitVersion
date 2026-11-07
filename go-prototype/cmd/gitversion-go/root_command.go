package main

import (
	"context"
	"encoding/json"
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
)

func runRootCLI(args []string, out, diagnostics io.Writer) error {
	if _, retired := os.LookupEnv("GITVERSION_USE_V6_ARGUMENT_PARSER"); retired {
		return fmt.Errorf("GITVERSION_USE_V6_ARGUMENT_PARSER has been removed. Unset it and use GITVERSION_ARGUMENT_PARSER_VERSION=v6 for the temporary legacy parser, or GITVERSION_ARGUMENT_PARSER_VERSION=v7 (the default).")
	}
	parser := strings.ToLower(strings.TrimSpace(os.Getenv("GITVERSION_ARGUMENT_PARSER_VERSION")))
	if parser != "" && parser != "v6" && parser != "v7" {
		return fmt.Errorf("Unrecognized GITVERSION_ARGUMENT_PARSER_VERSION value '%s'. Valid values are 'v6' and 'v7'.", parser)
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
		runtime := options.OverrideConfiguration
		if version == "v6" {
			runtime = nil
		}
		resolved, err := provider.ProvideFrom(&directory, configRepositoryRoot(directory), runtime)
		if err != nil {
			return err
		}
		data, err := resolved.Serialize(version)
		if err != nil {
			return err
		}
		_, err = out.Write(append(data, '\n'))
		return err
	}
	if options.TargetURL != "" || options.TargetBranch != "" || options.CommitID != "" || options.ClonePath != "" {
		return fmt.Errorf("remote, branch, and commit targeting are pending")
	}
	if options.UpdateAssemblyInfo || options.UpdateProjectFiles || options.UpdateWixVersionFile {
		return fmt.Errorf("the selected output or file updater is pending")
	}
	if options.AllowShallow || options.Diag {
		return fmt.Errorf("shallow history and diagnostic mode are pending")
	}
	values, _, err := calculateCLI(options.TargetPath, options.ConfigurationFile, options.LogFilePath, options.OverrideConfiguration, diagnostics)
	if err != nil {
		return err
	}
	if options.Output["buildserver"] {
		agent := buildagent.Resolve(options.TargetPath, os.LookupEnv)
		update, err := updateBuildNumberSetting(options.TargetPath, options.ConfigurationFile, options.OverrideConfiguration)
		if err != nil {
			return err
		}
		if err := agent.WriteIntegration(context.Background(), out, values, update); err != nil {
			return err
		}
	}
	if options.Output["dotenv"] {
		return buildagent.WriteDotEnv(out, values)
	}
	if options.Output["file"] {
		data, err := json.MarshalIndent(values, "", "  ")
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
		_, err = fmt.Fprintln(out, value)
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
	return json.NewEncoder(out).Encode(values)
}

func calculateCLI(path, custom, logFile string, overrides []byte, diagnostics io.Writer) (map[string]any, calculate.Trace, error) {
	if _, err := selectedConfigurationVersion(); err != nil {
		return nil, calculate.Trace{}, err
	}
	var warning string
	cfg, err := config.LoadNamedWithOverrides(path, custom, overrides, func(message string) { warning = message })
	if err != nil {
		if strings.Contains(err.Error(), "uses the legacy v6 configuration structure") {
			err = fmt.Errorf("An error occurred:\n%w", err)
		}
		return nil, calculate.Trace{}, err
	}
	if warning != "" {
		fmt.Fprintln(diagnostics, warning)
	}
	values, trace, err := calculate.CalculateWithConfig(path, cfg)
	if err != nil {
		return nil, trace, err
	}
	if warning != "" && logFile != "" && logFile != "console" {
		file, err := os.OpenFile(resolveMigrationPath(path, logFile), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			return nil, trace, err
		}
		_, err = fmt.Fprintln(file, warning)
		if err := errors.Join(err, file.Close()); err != nil {
			return nil, trace, err
		}
	}
	return values, trace, nil
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

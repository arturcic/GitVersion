package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/buildagent"
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
)

func runConfigGetInvocation(args []string, stdout, stderr io.Writer) (bool, error) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] != "config" || args[i+1] != "get" {
			continue
		}
		if i > 0 && (args[0] == "calculate" || args[0] == "observe" || args[0] == "config") {
			return false, nil
		}
		root, workdir, err := parseOptionTokens(args[:i])
		if err != nil {
			return true, err
		}
		if workdir == "" {
			workdir = "."
		}
		if i+2 == len(args) {
			return true, fmt.Errorf("Could not parse command line parameter")
		}
		for _, name := range validationUnsupportedRootOptions {
			if _, exists := root[name]; exists && name != "log-file" && name != "verbosity" {
				return true, fmt.Errorf("Option '--%s' cannot be used with 'config get'.", name)
			}
		}
		flags := []string{args[i+2]}
		for _, name := range []string{"config", "target-path", "override-config", "telemetry-opt-out", "log-file", "verbosity"} {
			for _, value := range root[name] {
				flags = append(flags, "--"+name+"="+value)
			}
		}
		flags = append(flags, args[i+3:]...)
		return true, runConfigQuery(flags, workdir, false, stdout, stderr)
	}
	return false, nil
}

func runConfigQuery(args []string, workdir string, show bool, stdout, stderr io.Writer) error {
	var property *string
	if !show {
		if len(args) == 0 || strings.HasPrefix(args[0], "--") {
			return fmt.Errorf("Could not parse command line parameter")
		}
		property, args = &args[0], args[1:]
	}
	options := flag.NewFlagSet("configuration", flag.ContinueOnError)
	options.SetOutput(io.Discard)
	options.StringVar(&workdir, "target-path", workdir, "working directory")
	var name, logFile string
	options.StringVar(&name, "config", "", "configuration file")
	options.StringVar(&name, "c", "", "configuration file")
	options.StringVar(&logFile, "log-file", "", "configuration diagnostics")
	options.StringVar(&logFile, "l", "", "configuration diagnostics")
	verbosity := options.String("verbosity", "Normal", "diagnostic verbosity")
	options.Bool("telemetry-opt-out", false, "disable telemetry")
	var overrides []string
	options.Func("override-config", "runtime key=value override", func(value string) error {
		overrides = append(overrides, value)
		return nil
	})
	if err := options.Parse(args); err != nil || options.NArg() != 0 {
		return fmt.Errorf("Could not parse command line parameter")
	}
	level, err := parseVerbosity(*verbosity)
	if err != nil {
		return fmt.Errorf("An error occurred:\n%w", err)
	}
	requested := os.Getenv("GITVERSION_CONFIGURATION_VERSION")
	version, err := selectedConfigurationVersion()
	if err != nil {
		return err
	}
	if !show && version != "v7" {
		return fmt.Errorf("An error occurred:\n%s", config.RequireV7Query)
	}
	runtime, err := config.ValidationOverrides(overrides, version)
	if err != nil {
		return err
	}
	dir, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}
	if !(config.OSConfigurationFS{}).DirectoryExists(dir) {
		if !show {
			return fmt.Errorf("An error occurred:\nThe working directory '%s' does not exist.", dir)
		}
		return fmt.Errorf("The selected directory does not exist.")
	}
	locator := config.FileLocator{}
	if name != "" {
		if !filepath.IsAbs(name) {
			name = filepath.Join(dir, name)
		}
		if !(config.OSConfigurationFS{}).FileExists(name) {
			return fmt.Errorf("Could not find config file at '%s'", name)
		}
		locator.Custom = &name
	}
	log := newCLILog(logFile, level, false, stderr)
	defer log.close()
	log.initialize(dir, version, buildagent.Resolve(dir, os.LookupEnv), false)
	locator.Log = log.configLookup
	provider := config.Provider{Locator: locator, Version: requested}
	return outputConfigurationQuery(version, dir, "", property, runtime, &provider, stdout)
}

type configurationView interface {
	Serialize(version string) ([]byte, error)
	SerializeProperty(property, version string) ([]byte, error)
}

func outputConfigurationQuery(version, directory, remoteURL string, property *string, overrides []byte, provider *config.Provider, out io.Writer) error {
	provide := func(directory, project *string, runtime []byte) (configurationView, error) {
		return provider.ProvideFrom(directory, project, runtime)
	}
	return writeConfigurationQuery(version, directory, remoteURL, property, overrides, configRepositoryRoot, provide, out)
}

func writeConfigurationQuery(version, directory, remoteURL string, property *string, overrides []byte,
	findProject func(string) *string, provide func(*string, *string, []byte) (configurationView, error), out io.Writer,
) error {
	if version != "v7" {
		if property != nil {
			return errors.New(config.RequireV7Query)
		}
		overrides = nil
	}
	var project *string
	if strings.TrimSpace(remoteURL) == "" {
		project = findProject(directory)
	}
	resolved, err := provide(&directory, project, overrides)
	if err != nil {
		return err
	}
	var data []byte
	if property == nil {
		data, err = resolved.Serialize(version)
	} else {
		data, err = resolved.SerializeProperty(*property, version)
	}
	if err != nil {
		return err
	}
	_, err = out.Write(append(data, '\n'))
	return err
}

func configRepositoryRoot(dir string) *string {
	for path := dir; ; path = filepath.Dir(path) {
		if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
			return &path
		}
		if parent := filepath.Dir(path); parent == path {
			return nil
		}
	}
}

func configurationDisplayInvocation(args []string) ([]string, string, bool) {
	if len(args) >= 1 && args[0] == "--show-config" {
		return args[1:], ".", true
	}
	if len(args) >= 2 && args[1] == "--show-config" && !strings.HasPrefix(args[0], "-") {
		return args[2:], args[0], true
	}
	return nil, "", false
}

func selectedConfigurationVersion() (string, error) {
	return resolveConfigurationVersion(os.Getenv("GITVERSION_CONFIGURATION_VERSION"))
}

func resolveConfigurationVersion(requested string) (string, error) {
	version, err := config.SelectConfigurationVersion(&requested)
	if err != nil {
		return "", fmt.Errorf("An error occurred:\nUnrecognized GITVERSION_CONFIGURATION_VERSION value '%s'. Valid values are 'v6' and 'v7'.", requested)
	}
	return version, nil
}

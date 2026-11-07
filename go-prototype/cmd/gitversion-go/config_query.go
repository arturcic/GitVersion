package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
		root, workdir, err := parseOptionTokens(args[:i], false)
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
	if !show && strings.EqualFold(strings.TrimSpace(os.Getenv("GITVERSION_ARGUMENT_PARSER_VERSION")), "v6") {
		return fmt.Errorf("Could not parse command line parameter")
	}
	var property string
	if !show {
		if len(args) == 0 || strings.HasPrefix(args[0], "--") {
			return fmt.Errorf("Could not parse command line parameter")
		}
		property, args = args[0], args[1:]
	}
	options := flag.NewFlagSet("configuration", flag.ContinueOnError)
	options.SetOutput(io.Discard)
	options.StringVar(&workdir, "target-path", workdir, "working directory")
	var name, logFile string
	options.StringVar(&name, "config", "", "configuration file")
	options.StringVar(&name, "c", "", "configuration file")
	options.StringVar(&logFile, "log-file", "", "configuration diagnostics")
	options.StringVar(&logFile, "l", "", "configuration diagnostics")
	options.String("verbosity", "Normal", "diagnostic verbosity")
	options.Bool("telemetry-opt-out", false, "disable telemetry")
	var overrides []string
	options.Func("override-config", "runtime key=value override", func(value string) error {
		overrides = append(overrides, value)
		return nil
	})
	if err := options.Parse(args); err != nil || options.NArg() != 0 {
		return fmt.Errorf("Could not parse command line parameter")
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
	if show && version == "v6" {
		runtime = nil
	}
	dir, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}
	if !(config.OSConfigurationFS{}).DirectoryExists(dir) {
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
	var logging io.Writer = io.Discard
	if logFile == "console" {
		logging = stderr
	} else if logFile != "" {
		if !filepath.IsAbs(logFile) {
			logFile = filepath.Join(dir, logFile)
		}
		file, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			return err
		}
		defer file.Close()
		logging = file
	}
	fmt.Fprintln(logging, "Configuration version:", version)
	locator.Log = func(message string) { fmt.Fprintln(logging, message) }
	provider := config.Provider{Locator: locator, Version: requested}
	resolved, err := provider.ProvideFrom(&dir, configRepositoryRoot(dir), runtime)
	if err != nil {
		return err
	}
	if show {
		data, err := resolved.Serialize(version)
		if err != nil {
			return err
		}
		_, err = stdout.Write(append(data, '\n'))
		return err
	}
	data, err := resolved.SerializeProperty(property, version)
	if err != nil {
		return err
	}
	_, err = stdout.Write(append(data, '\n'))
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
	requested := os.Getenv("GITVERSION_CONFIGURATION_VERSION")
	version, err := config.SelectConfigurationVersion(&requested)
	if err != nil {
		return "", fmt.Errorf("An error occurred:\nUnrecognized GITVERSION_CONFIGURATION_VERSION value '%s'. Valid values are 'v6' and 'v7'.", requested)
	}
	return version, nil
}

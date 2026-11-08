package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
)

type migrationOptions struct {
	workingDirectory, input, output, logFile string
	inPlace, force                           bool
}

func parseMigrationOptions(args []string, directory string) (migrationOptions, error) {
	result := migrationOptions{workingDirectory: directory}
	flags := flag.NewFlagSet("config migrate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&result.workingDirectory, "target-path", directory, "working directory")
	flags.StringVar(&result.input, "config", "", "input configuration")
	flags.StringVar(&result.input, "c", "", "input configuration")
	flags.StringVar(&result.output, "output", "", "output configuration")
	flags.StringVar(&result.logFile, "log-file", "", "migration diagnostics")
	flags.StringVar(&result.logFile, "l", "", "migration diagnostics")
	flags.BoolVar(&result.inPlace, "in-place", false, "replace input")
	flags.BoolVar(&result.force, "force", false, "replace output")
	flags.Bool("telemetry-opt-out", false, "disable telemetry")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return result, errors.New("Could not parse command line parameter")
	}
	if result.inPlace && result.output != "" {
		return result, errors.New("Cannot use --output together with --in-place.")
	}
	if result.force && result.output == "" {
		return result, errors.New("--force can only be used together with --output.")
	}
	return result, nil
}

func migrationInvocation(args []string) (migrationOptions, bool, error) {
	if len(args) > 0 && (args[0] == "calculate" || args[0] == "observe" || args[0] == "config" && (len(args) < 2 || args[1] != "migrate")) {
		return migrationOptions{}, false, nil
	}
	for i := 0; i+1 < len(args); i++ {
		if args[i] != "config" || args[i+1] != "migrate" {
			continue
		}
		root, directory, err := parseOptionTokens(args[:i])
		if err != nil {
			return migrationOptions{}, true, err
		}
		for j := i + 2; j < len(args); j++ {
			name, _, inline := strings.Cut(args[j], "=")
			switch name {
			case "--config", "-c", "--output", "--target-path":
				if !inline {
					j++
				}
			case "--log-file", "-l", "--verbosity":
				return migrationOptions{}, true, fmt.Errorf("An error occurred:\n%w", argumentError(name))
			}
		}
		if directory == "" {
			directory = "."
		}
		if paths := root["target-path"]; len(paths) != 0 {
			directory = paths[len(paths)-1]
		}
		flags := []string{}
		for _, name := range []string{"config", "log-file"} {
			for _, value := range root[name] {
				flags = append(flags, "--"+name+"="+value)
			}
		}
		flags = append(flags, args[i+2:]...)
		options, err := parseMigrationOptions(flags, directory)
		return options, true, err
	}
	return migrationOptions{}, false, nil
}

func runConfigMigrationInvocation(args []string, stdout, stderr io.Writer) (bool, error) {
	options, handled, err := migrationInvocation(args)
	if err != nil || !handled {
		return handled, err
	}
	version, err := selectedConfigurationVersion()
	if err != nil {
		return true, err
	}
	log := newCLILog(options.logFile, "", false, stderr)
	defer log.close()
	log.featureSelection(version)
	return true, executeMigration(options, stdout, log)
}

func migrationInput(options migrationOptions, directory string, log *cliLog) (string, error) {
	if options.input != "" {
		return resolveMigrationPath(directory, options.input), nil
	}
	locator := config.FileLocator{Log: log.configLookup}
	file, err := locator.Get(&directory)
	if err == nil && file == nil {
		root := configRepositoryRoot(directory)
		if root != nil && *root != directory {
			file, err = locator.Get(root)
		}
	}
	if err != nil {
		return "", err
	}
	if file == nil {
		return "", os.ErrNotExist
	}
	return *file, nil
}

func resolveMigrationPath(directory, name string) string {
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	return filepath.Join(directory, name)
}

func executeMigration(options migrationOptions, stdout io.Writer, log *cliLog) error {
	directory, err := filepath.Abs(options.workingDirectory)
	if err != nil {
		return err
	}
	input, err := migrationInput(options, directory, log)
	if errors.Is(err, os.ErrNotExist) || (err == nil && !(config.OSConfigurationFS{}).FileExists(input)) {
		return errors.New("Could not find a configuration file to migrate. Specify one with --config.")
	}
	if err != nil {
		return err
	}
	original, err := (config.OSConfigurationFS{}).ReadFile(input)
	if err != nil {
		return err
	}
	migrated, err := config.MigrateConfiguration(original)
	if err != nil {
		return err
	}
	if !options.inPlace && options.output == "" {
		_, err = rawTextWriter(stdout).Write(migrated)
		return err
	}
	destination := input
	if !options.inPlace {
		destination = resolveMigrationPath(directory, options.output)
	}
	info, err := os.Lstat(destination)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	overwrite := options.inPlace || options.force
	if info != nil && !overwrite {
		return migrationConflict(destination)
	}
	if info != nil && info.IsDir() {
		return fmt.Errorf("The output path '%s' is a directory.", destination)
	}
	if info != nil {
		log.write(logWarning, fmt.Sprintf("Replacing '%s'. Comments cannot be preserved during migration.", destination))
	}
	if err := writeFileAtomically(destination, migrated, info, overwrite); err != nil {
		if !overwrite && errors.Is(err, os.ErrExist) {
			return migrationConflict(destination)
		}
		return err
	}
	return nil
}

func migrationConflict(destination string) error {
	return fmt.Errorf("The output file '%s' already exists. Use --force to replace it.", destination)
}

func writeFileAtomically(destination string, data []byte, existing os.FileInfo, overwrite bool) error {
	temp, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if existing != nil && existing.Mode().IsRegular() {
		if err := temp.Chmod(existing.Mode().Perm()); err != nil {
			_ = temp.Close()
			return err
		}
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if overwrite {
		return os.Rename(temp.Name(), destination)
	}
	// Link atomically publishes a new output and fails if another process
	// created the destination after the preflight check.
	return os.Link(temp.Name(), destination)
}

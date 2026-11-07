package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

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

func migrationInvocation(args []string) ([]string, string, bool) {
	if flags, directory, ok := configInvocation(args, "migrate"); ok {
		return flags, directory, true
	}
	if len(args) >= 4 && (args[0] == "-c" || args[0] == "--config") && args[2] == "config" && args[3] == "migrate" {
		return append(append([]string{}, args[:2]...), args[4:]...), ".", true
	}
	return nil, "", false
}

func runConfigMigration(args []string, directory string, stdout, stderr io.Writer) error {
	options, err := parseMigrationOptions(args, directory)
	if err != nil {
		return err
	}
	return executeMigration(options, stdout, stderr)
}

func migrationInput(options migrationOptions, directory string) (string, error) {
	if options.input != "" {
		return resolveMigrationPath(directory, options.input), nil
	}
	locator := config.FileLocator{}
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

func executeMigration(options migrationOptions, stdout, stderr io.Writer) error {
	directory, err := filepath.Abs(options.workingDirectory)
	if err != nil {
		return err
	}
	input, err := migrationInput(options, directory)
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
		_, err = stdout.Write(migrated)
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
	if err := writeFileAtomically(destination, migrated, info, overwrite); err != nil {
		if !overwrite && errors.Is(err, os.ErrExist) {
			return migrationConflict(destination)
		}
		return err
	}
	if info != nil {
		message := fmt.Sprintf("Replacing '%s'. Comments cannot be preserved during migration.", destination)
		fmt.Fprintln(stderr, message)
		if options.logFile != "" && options.logFile != "console" {
			log, err := os.OpenFile(resolveMigrationPath(directory, options.logFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(log, message)
			closeErr := log.Close()
			return errors.Join(err, closeErr)
		}
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

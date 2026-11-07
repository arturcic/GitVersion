// gitversion-go calculates the declared workflow subset and exposes observations.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/GitTools/GitVersion/go-prototype/internal/calculate"
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func run(args []string, out io.Writer) error {
	return execute(args, out, io.Discard)
}
func execute(args []string, out, diagnostics io.Writer) error {
	if flags, directory, ok := migrationInvocation(args); ok {
		return runConfigMigration(flags, directory, out, diagnostics)
	}
	if len(args) == 1 && args[0] == "config" {
		return fmt.Errorf("The 'config' command requires a subcommand. Use 'gitversion config get', 'gitversion config validate' or 'gitversion config migrate'.")
	}
	if flags, directory, ok := configInvocation(args, "validate"); ok {
		return runConfigValidate(flags, directory, out, diagnostics)
	}
	if flags, directory, ok := configInvocation(args, "get"); ok {
		return runConfigQuery(flags, directory, false, out, diagnostics)
	}
	if flags, directory, ok := configurationDisplayInvocation(args); ok {
		return runConfigQuery(flags, directory, true, out, diagnostics)
	}
	for i, arg := range args {
		if arg == "--update-wix-version-file" && i+2 < len(args) && args[i+1] == "config" && args[i+2] == "get" {
			return fmt.Errorf("Option --update-wix-version-file cannot be used with 'config get'")
		}
	}
	usage := "usage: gitversion-go calculate|observe --repository PATH [--output json] [--config PATH] [--trace]"
	if len(args) == 0 || (args[0] != "observe" && args[0] != "calculate") {
		return fmt.Errorf("%s", usage)
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(io.Discard)
	path := f.String("repository", "", "repository root")
	format := f.String("output", "json", "output format")
	custom := f.String("config", "", "configuration file path")
	trace := f.Bool("trace", false, "emit calculation trace on stderr")
	logFile := f.String("log-file", "", "write calculation diagnostics")
	if err := f.Parse(args[1:]); err != nil {
		return fmt.Errorf("%s: %w", usage, err)
	}
	if *path == "" || f.NArg() != 0 || *format != "json" || ((*trace || *custom != "" || *logFile != "") && args[0] != "calculate") {
		return fmt.Errorf("%s", usage)
	}
	if args[0] == "calculate" {
		if _, err := selectedConfigurationVersion(); err != nil {
			return err
		}
		var warning string
		cfg, err := config.LoadNamedWithWarning(*path, *custom, func(message string) { warning = message })
		if err != nil {
			if strings.Contains(err.Error(), "uses the legacy v6 configuration structure") {
				return fmt.Errorf("An error occurred:\n%w", err)
			}
			return err
		}
		if warning != "" {
			fmt.Fprintln(diagnostics, warning)
		}
		values, detail, err := calculate.CalculateWithConfig(*path, cfg)
		if err != nil {
			return err
		}
		if warning != "" && *logFile != "" && *logFile != "console" {
			name := *logFile
			if !filepath.IsAbs(name) {
				name = filepath.Join(*path, name)
			}
			file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(file, warning)
			closeErr := file.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
		if *trace {
			if err = json.NewEncoder(diagnostics).Encode(detail); err != nil {
				return err
			}
		}
		return json.NewEncoder(out).Encode(values)
	}
	r, err := gitrepo.Open(*path)
	if err != nil {
		return err
	}
	observation, err := r.Observe()
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(observation)
}
func main() {
	if err := execute(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if err != invalidConfiguration {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

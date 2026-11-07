// gitversion-go calculates the declared workflow subset and exposes observations.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"io"
	"os"
)

func run(args []string, out io.Writer) error {
	return execute(args, out, io.Discard)
}
func execute(args []string, out, diagnostics io.Writer) error {
	if containsCLISchemaOption(args) {
		root := compatibilityCLITree()
		if handled, err := cliSchemaInvocation(args, root); handled {
			if err != nil {
				return err
			}
			parser, err := selectedArgumentParser()
			if err != nil {
				return err
			}
			if parser == "v6" {
				return argumentError("--cli-schema")
			}
			return writeCLISchema(out, root)
		}
	}
	if len(args) > 0 && args[0] == "msbuild" {
		return runMSBuild(args[1:], out, diagnostics)
	}
	if flags, directory, ok := migrationInvocation(args); ok {
		return runConfigMigration(flags, directory, out, diagnostics)
	}
	if len(args) == 1 && args[0] == "config" {
		return fmt.Errorf("The 'config' command requires a subcommand. Use 'gitversion config get', 'gitversion config validate' or 'gitversion config migrate'.")
	}
	if handled, err := runValidationInvocation(args, out, diagnostics); handled {
		return err
	}
	if handled, err := runConfigGetInvocation(args, out, diagnostics); handled {
		return err
	}
	if flags, directory, ok := configurationDisplayInvocation(args); ok {
		return runConfigQuery(flags, directory, true, out, diagnostics)
	}

	if len(args) == 0 || (args[0] != "observe" && args[0] != "calculate") {
		return runRootCLI(args, out, diagnostics)
	}
	usage := "usage: gitversion-go calculate|observe --repository PATH [--output json] [--config PATH] [--trace]"
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(io.Discard)
	path := f.String("repository", "", "repository root")
	format := f.String("output", "json", "output format")
	custom := f.String("config", "", "configuration file path")
	trace := f.Bool("trace", false, "emit calculation trace on stderr")
	allowShallow := f.Bool("allow-shallow", false, "allow calculation from shallow history")
	logFile := f.String("log-file", "", "write calculation diagnostics")
	if err := f.Parse(args[1:]); err != nil {
		return fmt.Errorf("%s: %w", usage, err)
	}
	if *path == "" || f.NArg() != 0 || *format != "json" || ((*trace || *allowShallow || *custom != "" || *logFile != "") && args[0] != "calculate") {
		return fmt.Errorf("%s", usage)
	}
	if args[0] == "calculate" {
		values, detail, err := calculateCLI(*path, *custom, *logFile, "", nil, diagnostics, gitrepo.OpenOptions{AllowShallow: *allowShallow})
		if err != nil {
			return err
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

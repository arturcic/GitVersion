package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/GitTools/GitVersion/go-prototype/internal/calculate"
)

func writeCalculationLog(options cliArguments, result calculationExecution, diagnostics io.Writer) error {
	if options.LogFilePath == "" || options.Verbosity == "Quiet" || options.Verbosity == "Minimal" {
		return nil
	}
	write := func(out io.Writer) error {
		for _, message := range result.cacheMessages {
			if _, err := fmt.Fprintln(out, "INFO "+message); err != nil {
				return err
			}
		}
		if result.cacheHit {
			return nil
		}
		return writeCalculationDetails(out, options.Verbosity, result.trace, result.values)
	}
	if options.LogFilePath == "console" {
		return write(diagnostics)
	}
	path := resolveMigrationPath(options.TargetPath, options.LogFilePath)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	return errors.Join(write(file), file.Close())
}

func writeCalculationDetails(out io.Writer, verbosity string, trace calculate.Trace, values map[string]any) error {
	if _, err := fmt.Fprintf(out, "INFO Calculated %s for branch %s at %s\n", values["FullSemVer"], trace.Branch, values["Sha"]); err != nil {
		return err
	}
	if verbosity == "Verbose" || verbosity == "Diagnostic" {
		if _, err := fmt.Fprintf(out, "DBUG Semantic version source %s at %s\n", trace.SemanticSource, trace.SemanticSHA); err != nil {
			return err
		}
		for _, candidate := range trace.Candidates {
			if _, err := fmt.Fprintf(out, "DBUG %s selected %s from %s\n", candidate.Description, candidate.Version, candidate.SHA); err != nil {
				return err
			}
		}
	}
	return nil
}

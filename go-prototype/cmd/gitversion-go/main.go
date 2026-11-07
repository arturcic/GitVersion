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
)

func run(args []string, out io.Writer) error {
	return execute(args, out, io.Discard)
}
func execute(args []string, out, diagnostics io.Writer) error {
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
	if err := f.Parse(args[1:]); err != nil {
		return fmt.Errorf("%s: %w", usage, err)
	}
	if *path == "" || f.NArg() != 0 || *format != "json" || ((*trace || *custom != "") && args[0] != "calculate") {
		return fmt.Errorf("%s", usage)
	}
	if args[0] == "calculate" {
		cfg, err := config.LoadNamed(*path, *custom)
		if err != nil {
			return err
		}
		values, detail, err := calculate.CalculateWithConfig(*path, cfg)
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

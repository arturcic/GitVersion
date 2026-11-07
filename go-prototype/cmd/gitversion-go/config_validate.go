package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
)

var invalidConfiguration = errors.New("configuration validation failed")
var yamlLine = regexp.MustCompile(`line ([0-9]+)`)

type validationDiagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Message  string `json:"message"`
	Line     *int   `json:"line"`
	Column   *int   `json:"column"`
}
type validationEnvelope struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Valid         bool                   `json:"valid"`
	Version       *string                `json:"configurationVersion"`
	File          *string                `json:"file"`
	Diagnostics   []validationDiagnostic `json:"diagnostics"`
}

// Root options are listed in the order in which the pinned parser reports an
// unsupported option. Options shared with the subcommand are forwarded.
var validationUnsupportedRootOptions = []string{
	"diagnose", "log-file", "output", "output-file", "show-variable", "format",
	"show-config", "no-fetch", "no-cache", "no-normalize", "allow-shallow",
	"verbosity", "update-assembly-info", "update-project-files", "ensure-assembly-info",
	"update-wix-version-file", "url", "branch", "username", "password", "commit",
	"dynamic-repo-location",
}

func runValidationInvocation(args []string, stdout, stderr io.Writer) (bool, error) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] != "config" || args[i+1] != "validate" {
			continue
		}
		if i > 0 && (args[0] == "calculate" || args[0] == "observe" || args[0] == "config") {
			return false, nil
		}
		if strings.EqualFold(strings.TrimSpace(os.Getenv("GITVERSION_ARGUMENT_PARSER_VERSION")), "v6") {
			return true, errors.New("The 'config validate' command is only available with the POSIX argument parser.")
		}
		root, workdir, err := parseOptionTokens(args[:i], false)
		if err != nil {
			return true, err
		}
		if workdir == "" {
			workdir = "."
		}
		var problem string
		for _, name := range validationUnsupportedRootOptions {
			if _, exists := root[name]; exists {
				problem = fmt.Sprintf("Option '--%s' cannot be used with 'config validate'.", name)
				break
			}
		}
		var flags []string
		for _, name := range []string{"config", "target-path", "override-config", "telemetry-opt-out", "help"} {
			for _, value := range root[name] {
				flags = append(flags, "--"+name+"="+value)
			}
		}
		flags = append(flags, args[i+2:]...)
		return true, runConfigValidate(flags, workdir, problem, discoverValidationFile, stdout, stderr)
	}
	return false, nil
}

func runConfigValidate(args []string, workdir, problem string, discover func(string) (*string, error), stdout, stderr io.Writer) error {
	options := flag.NewFlagSet("config validate", flag.ContinueOnError)
	options.SetOutput(io.Discard)
	options.StringVar(&workdir, "target-path", workdir, "working directory")
	var file, format string
	options.StringVar(&file, "config", "", "configuration file")
	options.StringVar(&file, "c", "", "configuration file")
	options.StringVar(&format, "format", "", "result format")
	options.Bool("telemetry-opt-out", false, "disable telemetry")
	help := options.Bool("help", false, "show help")
	var overrides []string
	options.Func("override-config", "runtime key=value override", func(value string) error {
		overrides = append(overrides, value)
		return nil
	})
	err := options.Parse(args)
	if errors.Is(err, flag.ErrHelp) || (err == nil && *help) {
		fmt.Fprintln(stdout, "Usage: gitversion-go [<path>] config validate [--config PATH] [--override-config KEY=VALUE] [--format json]\nValidates configuration without reading Git history. --format json emits a versioned JSON result.")
		return nil
	}
	if err != nil || options.NArg() != 0 {
		return errors.New("Could not parse command line parameter")
	}
	if problem == "" && format != "" && format != "json" {
		problem = "config validate --format supports only json."
	}

	result := validationEnvelope{SchemaVersion: 1, Diagnostics: []validationDiagnostic{}}
	requested := os.Getenv("GITVERSION_CONFIGURATION_VERSION")
	version, err := config.SelectConfigurationVersion(&requested)
	if err == nil {
		result.Version = &version
		if problem != "" {
			err = &config.ValidationError{Code: "GVCONFIG_INVOCATION", Reason: problem}
		} else {
			result.File, err = validationFile(workdir, file, discover)
		}
		if err == nil {
			var data []byte
			data, err = (config.OSConfigurationFS{}).ReadFile(*result.File)
			if err == nil {
				var runtime []byte
				runtime, err = config.ValidationOverrides(overrides, version)
				if err == nil {
					_, err = config.ResolveConfiguration(data, runtime, version)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				err = &config.ValidationError{Code: "GVCONFIG_FILE", Reason: "Configuration file could not be read."}
			}
		}
	} else {
		err = &config.ValidationError{Code: "GVCONFIG_VALUE", Reason: "Invalid configuration value or invocation."}
	}
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, diagnosticFor(err))
	} else if config.IsExplicitV6(&requested) {
		result.Diagnostics = append(result.Diagnostics, validationDiagnostic{
			Code: "GVCONFIG_V6_COMPATIBILITY", Severity: "warning",
			Message: "Temporary v6 compatibility mode is removed in GitVersion 7.1. Run 'gitversion config migrate' and validate with GITVERSION_CONFIGURATION_VERSION=v7.",
		})
	}
	result.Valid = err == nil
	label := ""
	if result.File != nil {
		label = *result.File
	}
	for _, diagnostic := range result.Diagnostics {
		location := label
		if diagnostic.Line != nil && diagnostic.Column != nil {
			location = fmt.Sprintf("%s(%d,%d)", label, *diagnostic.Line, *diagnostic.Column)
		}
		fmt.Fprintf(stderr, "%s: %s %s %s: %s\n", location, diagnostic.Severity, diagnostic.Code, diagnostic.Path, diagnostic.Message)
	}
	if result.Valid {
		fmt.Fprintf(stderr, "Configuration is valid. %s\n", label)
	} else {
		fmt.Fprintf(stderr, "Configuration is invalid. %s\n", label)
	}
	if format == "json" {
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			return err
		}
	}
	if !result.Valid {
		return invalidConfiguration
	}
	return nil
}

func validationFile(workdir, name string, discover func(string) (*string, error)) (*string, error) {
	if workdir == "" {
		return nil, &config.ValidationError{Code: "GVCONFIG_INVOCATION", Reason: "The selected directory path is invalid."}
	}
	dir, err := filepath.Abs(workdir)
	if err != nil || !(config.OSConfigurationFS{}).DirectoryExists(dir) {
		return nil, &config.ValidationError{Code: "GVCONFIG_FILE", Reason: "The selected directory does not exist."}
	}
	if name != "" {
		if !filepath.IsAbs(name) {
			name = filepath.Join(dir, name)
		}
		name = filepath.Clean(name)
		return &name, nil
	}
	return discover(dir)
}

func discoverValidationFile(dir string) (*string, error) {
	locator := config.FileLocator{}
	file, err := locator.Get(&dir)
	if err != nil {
		return nil, err
	}
	if root := configRepositoryRoot(dir); root != nil && *root != dir {
		rootFile, err := locator.Get(root)
		if err != nil {
			return nil, err
		}
		if file != nil && rootFile != nil {
			return nil, &config.ValidationError{Code: "GVCONFIG_DISCOVERY", Reason: "Ambiguous configuration file selection. Select a file with --config."}
		}
		if file == nil {
			file = rootFile
		}
	}
	if file == nil {
		return nil, os.ErrNotExist
	}
	return file, nil
}

func diagnosticFor(err error) validationDiagnostic {
	diagnostic := validationDiagnostic{Severity: "error"}
	var validation *config.ValidationError
	var syntax *config.YAMLError
	var layout *config.LayoutError
	switch {
	case errors.As(err, &validation):
		diagnostic.Code, diagnostic.Path, diagnostic.Message = validation.Code, validation.PropertyPath, validation.Error()
	case errors.As(err, &syntax):
		diagnostic.Code, diagnostic.Message = "GVCONFIG_YAML", "Invalid YAML syntax or configuration value."
		if line := yamlLine.FindStringSubmatch(syntax.Error()); len(line) > 1 {
			number, _ := strconv.Atoi(line[1])
			diagnostic.Line = &number
		}
	case errors.As(err, &layout):
		diagnostic.Code, diagnostic.Message = "GVCONFIG_LAYOUT", "Invalid configuration layout. Check property placement and the selected configuration version; use 'gitversion config migrate' for a v6 file."
	case errors.Is(err, os.ErrNotExist):
		diagnostic.Code, diagnostic.Message = "GVCONFIG_FILE", "Configuration property '': Configuration file not found. Specify an existing file with --config."
	default:
		diagnostic.Code, diagnostic.Message = "GVCONFIG_FILE", "Configuration file could not be read."
	}
	return diagnostic
}

func configInvocation(args []string, command string) ([]string, string, bool) {
	if len(args) >= 2 && args[0] == "config" && args[1] == command {
		return args[2:], ".", true
	}
	if len(args) >= 3 && !strings.HasPrefix(args[0], "-") && args[1] == "config" && args[2] == command {
		return args[3:], args[0], true
	}
	return nil, "", false
}

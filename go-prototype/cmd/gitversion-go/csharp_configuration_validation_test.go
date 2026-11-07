package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/commandline"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	git "github.com/go-git/go-git/v5"
)

func TestCSharpConfigurationValidationCLI(t *testing.T) {
	var corpus struct {
		Revision string
		Methods  map[string][]struct {
			ID        string
			Arguments []json.RawMessage
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-configuration-validation.json", &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Methods) != 16 {
		t.Fatal("original validation inventory changed")
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.ID, func(t *testing.T) {
					checkCSharpValidationCase(t, method, tc.Arguments)
				})
			}
		})
	}
}

type validationObservation struct {
	exit           int
	stdout, stderr string
}

func observeValidation(invoke func(io.Writer, io.Writer) error) validationObservation {
	var stdout, stderr bytes.Buffer
	err := invoke(&stdout, &stderr)
	exit := 0
	if err != nil {
		exit = 1
		if err != invalidConfiguration {
			fmt.Fprintln(&stderr, err)
		}
	}
	return validationObservation{exit, stdout.String(), stderr.String()}
}

func (r validationObservation) requireExit(t *testing.T, expected int) {
	t.Helper()
	if r.exit != expected {
		t.Fatalf("exit %d, want %d, stdout %q, stderr %q", r.exit, expected, r.stdout, r.stderr)
	}
}

func (r validationObservation) requireJSON(t *testing.T, valid bool) validationEnvelope {
	t.Helper()
	r.requireExit(t, validationExitCode(valid))
	var result validationEnvelope
	if err := json.Unmarshal([]byte(r.stdout), &result); err != nil || result.Valid != valid {
		t.Fatalf("expected one JSON envelope with valid=%t, got %q, error %v", valid, r.stdout, err)
	}
	return result
}

func (r validationObservation) requireSafe(t *testing.T) {
	t.Helper()
	if strings.Contains(r.stdout, "secret-value") || strings.Contains(r.stderr, "secret-value") {
		t.Fatalf("diagnostics expose candidate content: stdout %q, stderr %q", r.stdout, r.stderr)
	}
}

func validationExitCode(valid bool) int {
	if valid {
		return 0
	}
	return 1
}

func checkValidationDiagnostic(t *testing.T, r validationEnvelope, code string) validationDiagnostic {
	t.Helper()
	if len(r.Diagnostics) == 0 || r.Diagnostics[0].Code != code {
		t.Fatalf("expected diagnostic %s, got %+v", code, r.Diagnostics)
	}
	return r.Diagnostics[0]
}

func checkCSharpValidationCase(t *testing.T, method string, args []json.RawMessage) {
	t.Helper()
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
	t.Setenv("GITVERSION_GIT_BACKEND", "managed")
	dir := t.TempDir()
	const explicit = "config validate --config candidate.yml"
	const jsonMode = " --format json"
	write := func(name, text string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	run := func(command string) validationObservation {
		t.Helper()
		return observeValidation(func(out, logs io.Writer) error {
			return execute(append([]string{dir}, commandline.SplitUnquoted(command, ' ')...), out, logs)
		})
	}
	snapshot := func(path string) map[string]string {
		t.Helper()
		files, err := parity.FileSet(path)
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	unchanged := func(path string, before map[string]string) {
		t.Helper()
		if after := snapshot(path); !reflect.DeepEqual(before, after) {
			t.Fatalf("validation changed files under %s", path)
		}
	}
	requireFile := func(report validationEnvelope, want string) {
		t.Helper()
		if report.File == nil || *report.File != want {
			t.Fatalf("configuration file %v, want %s", report.File, want)
		}
	}
	requireVersion := func(report validationEnvelope, want string) {
		t.Helper()
		if report.Version == nil || *report.Version != want {
			t.Fatalf("configuration version %v, want %s", report.Version, want)
		}
	}

	switch method {
	case "ValidDocumentsHaveEmptyStdoutAndNoSideEffects":
		text := cliArg[string](t, args[0])
		file := write("candidate.yml", text)
		before := snapshot(dir)
		r := run(explicit)
		r.requireExit(t, 0)
		if r.stdout != "" || !strings.Contains(r.stderr, "Configuration is valid.") {
			t.Fatalf("expected valid text result, got %+v", r)
		}
		data, err := os.ReadFile(file)
		if err != nil || string(data) != text {
			t.Fatalf("configuration content changed, error %v", err)
		}
		unchanged(dir, before)

	case "InvalidDocumentsProduceOneSafeJsonEnvelope":
		file := write("candidate.yml", cliArg[string](t, args[0]))
		code := cliArg[string](t, args[1])
		before := snapshot(dir)
		r := run(explicit + jsonMode)
		report := r.requireJSON(t, false)
		requireFile(report, file)
		requireVersion(report, "v7")
		diagnostic := checkValidationDiagnostic(t, report, code)
		var raw struct {
			Diagnostics []map[string]any
		}
		if err := json.Unmarshal([]byte(r.stdout), &raw); err != nil || len(raw.Diagnostics) != 1 {
			t.Fatalf("expected one diagnostic, error %v", err)
		}
		_, pathIsString := raw.Diagnostics[0]["path"].(string)
		if report.SchemaVersion != 1 || diagnostic.Severity != "error" || !pathIsString || diagnostic.Message == "" {
			t.Fatalf("invalid diagnostic envelope %+v", report)
		}
		if code == "GVCONFIG_YAML" && (diagnostic.Line == nil || *diagnostic.Line <= 0) {
			t.Fatal("YAML diagnostic has no source line")
		}
		if !strings.Contains(r.stderr, "Configuration is invalid.") {
			t.Fatal(r.stderr)
		}
		r.requireSafe(t)
		unchanged(dir, before)

	case "OverridesUseRuntimeRulesAndSafeDiagnostics":
		write("candidate.yml", "{}")
		r := run(explicit + jsonMode + " " + cliArg[string](t, args[0]))
		report := r.requireJSON(t, cliArg[bool](t, args[1]))
		if code := cliArg[*string](t, args[2]); code != nil {
			checkValidationDiagnostic(t, report, *code)
		}
		r.requireSafe(t)

	case "ExplicitV6CompatibilityWarnsWithoutFailing":
		t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v6")
		write("candidate.yml", "tag-prefix: '[vV]?'")
		r := run(explicit + jsonMode)
		report := r.requireJSON(t, true)
		requireVersion(report, "v6")
		checkValidationDiagnostic(t, report, "GVCONFIG_V6_COMPATIBILITY")
		if !strings.Contains(r.stderr, "removed in GitVersion 7.1") {
			t.Fatal(r.stderr)
		}

	case "MissingFilesNeverValidateDefaults":
		r := run(cliArg[string](t, args[0]))
		checkValidationDiagnostic(t, r.requireJSON(t, false), "GVCONFIG_FILE")
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("validation created entries %v, error %v", entries, err)
		}

	case "ExplicitPathSupportsSpacesAndUnicodeOutsideGit":
		file := write("config ü space.yml", "{}")
		r := observeValidation(func(out, logs io.Writer) error {
			return execute([]string{dir, "config", "validate", "--config", file, "--format", "json"}, out, logs)
		})
		report := r.requireJSON(t, true)
		requireFile(report, file)
		if len(report.Diagnostics) != 0 {
			t.Fatalf("unexpected diagnostics %+v", report.Diagnostics)
		}

	case "DiscoveryPreservesSupportedFilenamePriority":
		write("GitVersion.yaml", "bad: [")
		write(".GitVersion.yml", "bad: [")
		file := write("GitVersion.yml", "{}")
		requireFile(run("config validate"+jsonMode).requireJSON(t, true), file)

	case "RepositoryDiscoveryPreservesRootAndAmbiguityRules":
		root := dir
		if _, err := git.PlainInit(root, false); err != nil {
			t.Fatal(err)
		}
		write("GitVersion.yml", "{}")
		dir = filepath.Join(root, "child")
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		ambiguous := cliArg[bool](t, args[0])
		if ambiguous {
			write("GitVersion.yml", "{}")
		}
		before := snapshot(root)
		r := run("config validate" + jsonMode)
		r.requireJSON(t, !ambiguous)
		if ambiguous && !strings.Contains(r.stderr, "Ambiguous configuration file selection") {
			t.Fatal(r.stderr)
		}
		unchanged(root, before)

	case "IncompatibleRootOptionsAreReportedWithoutSideEffects":
		write("candidate.yml", "{}")
		before := snapshot(dir)
		r := run(cliArg[string](t, args[0]) + " " + explicit + jsonMode)
		checkValidationDiagnostic(t, r.requireJSON(t, false), "GVCONFIG_INVOCATION")
		unchanged(dir, before)

	case "ParserFailuresUseExistingStderrContract":
		r := run(cliArg[string](t, args[0]))
		r.requireExit(t, 1)
		if r.stdout != "" || !strings.Contains(r.stderr, "Could not parse command line parameter") {
			t.Fatalf("expected parser failure, got %+v", r)
		}

	case "ValidationHelpDescribesTheCommandAndLocalJsonOption":
		r := run("config validate --help")
		r.requireExit(t, 0)
		if r.stderr != "" || !strings.Contains(r.stdout, "--format") ||
			!strings.Contains(r.stdout, "versioned JSON") || !strings.Contains(r.stdout, "--override-config") {
			t.Fatalf("unexpected help result %+v", r)
		}

	case "ValidationIsUnavailableWithLegacyArgumentParser":
		t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v6")
		r := run("config validate")
		r.requireExit(t, 1)
		if r.stdout != "" {
			t.Fatal(r.stdout)
		}

	case "ExplicitValidationNeverConstructsGitOrCalculationServices":
		t.Setenv("GITVERSION_GIT_BACKEND", cliArg[string](t, args[0]))
		write("candidate.yml", cliArg[string](t, args[1]))
		write(".git", "gitdir: missing\n")
		valid := cliArg[bool](t, args[2])
		run(explicit+jsonMode).requireJSON(t, valid)
		r := observeValidation(func(out, logs io.Writer) error {
			discover := func(string) (*string, error) {
				t.Fatal("explicit validation called the discovery service")
				return nil, nil
			}
			return runConfigValidate([]string{"--config", "candidate.yml", "--format", "json"}, dir, "", discover, out, logs)
		})
		r.requireJSON(t, valid)

	case "ExplicitValidationPreservesHistoryAndDirtyFiles":
		checkpoints, err := fixtures.Generate([]fixtures.Recipe{{
			ID: "validation-history", Config: "{}",
			Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}, {Op: "checkpoint", ID: "head"}},
		}}, filepath.Join(dir, "history"))
		if err != nil {
			t.Fatal(err)
		}
		dir = checkpoints[0].Path
		write("dirty.txt", "uncommitted content")
		write("candidate.yml", cliArg[string](t, args[0]))
		before := snapshot(dir)
		r := run(explicit)
		r.requireExit(t, validationExitCode(cliArg[bool](t, args[1])))
		if r.stdout != "" {
			t.Fatal(r.stdout)
		}
		unchanged(dir, before)

	case "NestedV7DocumentDoesNotFallBackWhenV6IsSelected":
		t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v6")
		write("candidate.yml", "calculation:\n  tag-prefix: v")
		report := run(explicit+jsonMode).requireJSON(t, false)
		requireVersion(report, "v6")
		checkValidationDiagnostic(t, report, "GVCONFIG_LAYOUT")

	case "InvalidLocalFormatIsAnExecutedInvocationError":
		write("candidate.yml", "{}")
		r := run(explicit + " --format xml")
		r.requireExit(t, 1)
		if r.stdout != "" || !strings.Contains(r.stderr, "GVCONFIG_INVOCATION") {
			t.Fatalf("expected validation invocation error, got %+v", r)
		}

	default:
		t.Fatal("original validation method missing", method)
	}
}

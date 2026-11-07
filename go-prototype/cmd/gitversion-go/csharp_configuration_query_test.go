package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	"gopkg.in/yaml.v3"
)

func TestCSharpConfigurationQueryCLI(t *testing.T) {
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct {
				Cases []struct {
					ID        string
					Arguments []json.RawMessage
				}
			}
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-configuration-query.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	for method, item := range corpus.Suites["ConfigurationQueryIntegrationTests"].Methods {
		if method == "QueryIsUnavailableWithLegacyArgumentParser" {
			continue
		}
		t.Run(method, func(t *testing.T) {
			for _, c := range item.Cases {
				t.Run(c.ID, func(t *testing.T) { checkCLIQueryCase(t, method, c.Arguments) })
			}
		})
	}
}

func cliArg[T any](t *testing.T, input json.RawMessage) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(input, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func queryFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := git.PlainInit(dir, false); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeQueryConfig(t *testing.T, directory, name, document string) string {
	t.Helper()
	file := filepath.Join(directory, name)
	if err := os.WriteFile(file, []byte(document), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func checkCLIQueryCase(t *testing.T, method string, args []json.RawMessage) {
	t.Helper()
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	t.Setenv("GITVERSION_GIT_BACKEND", "managed")
	dir := queryFixture(t)
	invoke := func(directory string, args ...string) (string, string, error) {
		t.Helper()
		var out, logs bytes.Buffer
		err := execute(append([]string{directory}, args...), &out, &logs)
		if err != nil {
			logs.WriteString(err.Error() + "\n")
		}
		return out.String(), logs.String(), err
	}
	query := func(directory, property string, options ...string) (string, string, error) {
		t.Helper()
		return invoke(directory, append([]string{"config", "get", property}, options...)...)
	}
	success := func(directory, property, expected string, options ...string) string {
		t.Helper()
		out, logs, err := query(directory, property, options...)
		if err != nil || out != expected+"\n" {
			t.Fatalf("output %q, logs %q, error %v; want %q", out, logs, err, expected)
		}
		return logs
	}
	failure := func(out, stderr string, err error, diagnostic string) {
		t.Helper()
		if err == nil || out != "" || !strings.Contains(stderr, diagnostic) {
			t.Fatalf("stdout %q, stderr %q, error %v; want %q", out, stderr, err, diagnostic)
		}
	}
	child := func() string {
		t.Helper()
		path := filepath.Join(dir, "child")
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		return path
	}
	switch method {
	case "QueryUsesDefaultAndWorkflowConfiguration":
		t.Setenv("GITVERSION_CONFIGURATION_VERSION", "")
		workflow := cliArg[*string](t, args[0])
		if workflow != nil {
			writeQueryConfig(t, dir, "GitVersion.yml", "workflow: "+*workflow)
		}
		if logs := success(dir, "calculation.tag-prefix", `"[vV]?"`); logs != "" {
			t.Fatal(logs)
		}
		if workflow != nil {
			t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
			encoded, err := json.Marshal(*workflow)
			if err != nil {
				t.Fatal(err)
			}
			success(dir, "workflow", string(encoded))
		}
	case "QueryAndDisplayHonorFileThenRuntimeOverrides":
		t.Setenv("GITVERSION_GIT_BACKEND", cliArg[string](t, args[0]))
		const input = "workflow: GitFlow/v1\ncalculation:\n  tag-prefix: file-\noutput:\n  tag-pre-release-weight: 12\n"
		file := writeQueryConfig(t, dir, "GitVersion.yml", input)
		options := []string{"--override-config", "workflow=GitHubFlow/v1", "--override-config", "calculation.tag-prefix=runtime-", "--override-config", "output.tag-pre-release-weight=0"}
		logFile := filepath.Join(dir, "query.log")
		withLog := cliArg[bool](t, args[1])
		queryOptions := slices.Clone(options)
		if withLog {
			queryOptions = append(queryOptions, "--log-file", logFile)
		}
		success(dir, "calculation.tag-prefix", `"runtime-"`, queryOptions...)
		success(dir, "output.tag-pre-release-weight", "0", options...)
		display, _, err := invoke(dir, append([]string{"--show-config"}, options...)...)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"tag-prefix: runtime-", "workflow: GitHubFlow/v1", "tag-pre-release-weight: 0"} {
			if !strings.Contains(display, want) {
				t.Fatalf("missing %q in %s", want, display)
			}
		}
		if withLog {
			log, err := os.ReadFile(logFile)
			if err != nil || !bytes.Contains(log, []byte("Configuration version: v7")) {
				t.Fatalf("log %s: %v", log, err)
			}
		}
		if raw, err := os.ReadFile(file); err != nil || string(raw) != input {
			t.Fatalf("file changed: %v", err)
		}
	case "QueryPreservesNullBooleanAndStringValues":
		writeQueryConfig(t, dir, "GitVersion.yml", "output:\n  assembly-versioning-format: null\n  update-build-number: false\n  custom-version-format: 'null'")
		success(dir, cliArg[string](t, args[0]), cliArg[string](t, args[1]))
	case "QueryRejectsUnsupportedConfigurationBeforeReadingFile":
		t.Setenv("GITVERSION_CONFIGURATION_VERSION", cliArg[string](t, args[0]))
		writeQueryConfig(t, dir, "GitVersion.yml", "invalid: [")
		out, logs, err := query(dir, "calculation.tag-prefix")
		failure(out, logs, err, cliArg[string](t, args[1]))
	case "QueryErrorsUseOnlyStderr":
		out, logs, err := invoke(dir, strings.Fields(cliArg[string](t, args[0]))...)
		failure(out, logs, err, cliArg[string](t, args[1]))
		if _, err := os.Stat(filepath.Join(dir, "GitVersion_WixVersion.wxi")); !os.IsNotExist(err) {
			t.Fatalf("WiX output exists: %v", err)
		}
	case "QueryRejectsLegacyDocumentUnderV7":
		writeQueryConfig(t, dir, "GitVersion.yml", "tag-prefix: legacy-")
		out, logs, err := query(dir, "calculation.tag-prefix")
		failure(out, logs, err, "uses the legacy v6 configuration structure")
	case "QueryDiscoversLocalOrRootConfigurationAndSupportsExplicitFile":
		subdir := child()
		configDir, prefix := dir, "root-"
		if cliArg[bool](t, args[0]) {
			configDir, prefix = subdir, "local-"
		}
		writeQueryConfig(t, configDir, "GitVersion.yml", "calculation:\n  tag-prefix: "+prefix)
		writeQueryConfig(t, subdir, "custom.yml", "calculation:\n  tag-prefix: explicit-")
		success(subdir, "calculation.tag-prefix", `"`+prefix+`"`)
		success(subdir, "calculation.tag-prefix", `"explicit-"`, "--config", "custom.yml")
	case "QueryWorksWithoutGitHistoryAndKeepsVerboseLogsOffStdout":
		entries := func() []string {
			files, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, file := range files {
				names = append(names, file.Name())
			}
			return names
		}
		before := entries()
		logs := success(dir, "output.update-build-number", "true", "--verbosity", "Diagnostic", "--log-file", "console")
		if !strings.Contains(logs, "Configuration version: v7") || !reflect.DeepEqual(before, entries()) {
			t.Fatal(logs)
		}
	case "QueryPreservesAmbiguousConfigurationDiagnostic":
		subdir := child()
		writeQueryConfig(t, dir, "GitVersion.yml", "calculation:\n  tag-prefix: root-")
		writeQueryConfig(t, subdir, "GitVersion.yml", "calculation:\n  tag-prefix: local-")
		out, logs, err := query(subdir, "calculation.tag-prefix")
		failure(out, logs, err, "Ambiguous configuration file selection")
	case "ShowConfigOverrideCorrectionIsLimitedToV7":
		version, path, expected := cliArg[string](t, args[0]), cliArg[string](t, args[1]), cliArg[string](t, args[2])
		t.Setenv("GITVERSION_CONFIGURATION_VERSION", version)
		display, _, err := invoke(dir, "--show-config", "--override-config", path+"=overridden-")
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := yaml.Unmarshal([]byte(display), &fields); err != nil {
			t.Fatal(err)
		}
		if version == "v7" {
			fields = fields["calculation"].(map[string]any)
		}
		if fields["tag-prefix"] != expected {
			t.Fatalf("tag-prefix %v, want %s", fields["tag-prefix"], expected)
		}
	default:
		t.Fatal(method)
	}
}

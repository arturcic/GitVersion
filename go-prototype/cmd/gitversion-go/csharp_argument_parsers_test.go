package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/commandline"
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"gopkg.in/yaml.v3"
)

type parserMethod struct {
	Command     *string
	Environment map[string]string
	Checks      []structuralCheck
	Error       []string
	Cases       []struct {
		ID        string
		Arguments []json.RawMessage
	}
}

type structuralCheck []any

func TestCSharpArgumentParsers(t *testing.T) {
	var corpus struct {
		Suites map[string]struct{ Methods map[string]parserMethod }
	}
	data, err := os.ReadFile("../../testdata/csharp-argument-parsers.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, suite := range []string{"ArgumentParserTests", "LegacyArgumentParserTests"} {
		t.Run(suite, func(t *testing.T) {
			for name, item := range corpus.Suites[suite].Methods {
				t.Run(name, func(t *testing.T) {
					for _, c := range item.Cases {
						t.Run(c.ID, func(t *testing.T) {
							legacy := suite == "LegacyArgumentParserTests"
							if len(item.Checks) != 0 || len(item.Error) != 0 {
								checkExtractedParserCase(t, legacy, item, c.Arguments)
							} else {
								checkSpecialParserCase(t, legacy, name, c.Arguments)
							}
						})
					}
				})
			}
		})
	}
}

func checkExtractedParserCase(t *testing.T, legacy bool, item parserMethod, args []json.RawMessage) {
	t.Helper()
	cwd := t.TempDir()
	env := item.Environment
	lookup := func(name string) string {
		if name == "GITVERSION_CONFIGURATION_VERSION" {
			return "v6"
		}
		return env[name]
	}
	command := ""
	if item.Command == nil {
		command = cliArg[string](t, args[0])
	} else {
		command = *item.Command
	}
	got, err := parseCLIArguments(commandline.SplitUnquoted(command, ' '), legacy, cwd, lookup)
	if len(item.Error) != 0 {
		if err == nil || !checkParserText(err.Error(), item.Error[0], item.Error[1]) {
			t.Fatalf("%q: error %v; want %v", command, err, item.Error)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, assertion := range item.Checks {
		name, operation, expected := assertion[0].(string), assertion[1].(string), assertion[2]
		switch expected {
		case "$cwd":
			expected = cwd
		case "$arg1":
			expected = cliArg[string](t, args[1])
		}
		if name == "Authentication.Username" || name == "Authentication.Password" {
			name = strings.TrimPrefix(name, "Authentication.")
		}
		if name == "TargetUrl" {
			name = "TargetURL"
		} else if name == "CommitId" {
			name = "CommitID"
		}
		field := reflect.ValueOf(got).FieldByName(name)
		if !field.IsValid() {
			t.Fatal("unmapped C# field", name)
		}
		ok := false
		switch operation {
		case "ShouldBe":
			if expected == nil {
				ok = field.IsZero()
			} else {
				ok = reflect.DeepEqual(field.Interface(), expected)
			}
		case "ShouldContain", "ShouldNotContain":
			if name != "Output" {
				t.Fatal("unmapped collection", name)
			}
			ok = got.Output[expected.(string)] == (operation == "ShouldContain")
		case "ShouldBeEmpty":
			ok = field.Len() == 0
		default:
			t.Fatal(operation)
		}
		if !ok {
			t.Fatalf("%s: %s %s %v, got %v", command, name, operation, expected, field.Interface())
		}
	}
}

func checkParserText(value, op, expected string) bool {
	switch op {
	case "ShouldBe":
		return value == expected
	case "ShouldContain":
		return strings.Contains(value, expected)
	case "ShouldStartWith":
		return strings.HasPrefix(value, expected)
	default:
		return false
	}
}

func checkSpecialParserCase(t *testing.T, legacy bool, name string, args []json.RawMessage) {
	t.Helper()
	cwd := t.TempDir()
	version := "v6"
	flagName := func(posix, old string) string {
		if legacy {
			return old
		}
		return posix
	}
	parse := func(command string) (cliArguments, error) {
		return parseCLIArguments(commandline.SplitUnquoted(command, ' '), legacy, cwd, func(name string) string {
			if name == "GITVERSION_CONFIGURATION_VERSION" {
				return version
			}
			return ""
		})
	}
	ok := func(command string) cliArguments {
		t.Helper()
		got, err := parse(command)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	fail := func(command string, fragments ...string) {
		t.Helper()
		_, err := parse(command)
		if err == nil {
			t.Fatalf("accepted %q", command)
		}
		for _, s := range fragments {
			if !strings.Contains(err.Error(), s) {
				t.Fatalf("%v lacks %q", err, s)
			}
		}
	}
	override := func(entries ...string) string {
		var parts []string
		for _, entry := range entries {
			parts = append(parts, flagName("--override-config", "/overrideconfig"), entry)
		}
		return strings.Join(parts, " ")
	}
	readOverride := func(result cliArguments) map[string]any {
		t.Helper()
		var value map[string]any
		if err := yaml.Unmarshal(result.OverrideConfiguration, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	switch name {
	case "ConfigValidateDefersInvalidTargetPathToItsExecutor":
		t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
		got := runMigrationTest("config", "validate", "--target-path", "", "--format", "json")
		var report validationEnvelope
		if got.err == nil || json.Unmarshal([]byte(got.stdout), &report) != nil || report.Valid ||
			len(report.Diagnostics) != 1 || report.Diagnostics[0].Code != "GVCONFIG_INVOCATION" ||
			report.Diagnostics[0].Message != "Configuration property '': The selected directory path is invalid." {
			t.Fatalf("%+v %+v", got, report)
		}
	case "ConfigGetRejectsUnrelatedOptionsBeforeAndAfterCommand":
		t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
		flag := strings.Fields(cliArg[string](t, args[0]))
		query := []string{"config", "get", "calculation.tag-prefix"}
		for _, command := range [][]string{append(slices.Clone(flag), query...), append(slices.Clone(query), flag...)} {
			if result := runMigrationTest(command...); result.err == nil {
				t.Fatalf("accepted %v", command)
			}
		}
	case "ConfigGetRequiresExactlyOneProperty":
		t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
		if result := runMigrationTest(strings.Fields(cliArg[string](t, args[0]))...); result.err == nil {
			t.Fatal(result)
		}
	case "ConfigGetRejectsV6BeforeConfigFileValidation":
		t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v6")
		r := runMigrationTest("config", "get", "calculation.tag-prefix", "--config", "missing.yml")
		if r.err == nil || !strings.Contains(r.err.Error(), "require v7 configuration") {
			t.Fatal(r)
		}
	case "OverrideConfigSupportsNestedV7RootAndBranchPaths":
		version = "v7"
		values := []string{"calculation.tag-prefix=custom-", "calculation.branches.main.increment=Major", "output.branches.main.pre-release-weight=42"}
		if !legacy {
			values = append([]string{"workflow=GitHubFlow/v1"}, values...)
		}
		parsed := ok(override(values...))
		got := readOverride(parsed)
		if !legacy && got["workflow"] != "GitHubFlow/v1" {
			t.Fatal(got)
		}
		calc, out := got["calculation"].(map[string]any), got["output"].(map[string]any)
		branch := func(section map[string]any) map[string]any {
			return section["branches"].(map[string]any)["main"].(map[string]any)
		}
		if calc["tag-prefix"] != "custom-" || branch(calc)["increment"] != "Major" || branch(out)["pre-release-weight"] != "42" {
			t.Fatal(got)
		}
		effective, err := config.ResolveConfiguration(nil, parsed.OverrideConfiguration, "v7")
		if err != nil {
			t.Fatal(err)
		}
		serialized, err := effective.Serialize("v6")
		if err != nil {
			t.Fatal(err)
		}
		var flat map[string]any
		if err := yaml.Unmarshal(serialized, &flat); err != nil {
			t.Fatal(err)
		}
		main := flat["branches"].(map[string]any)["main"].(map[string]any)
		if main["pre-release-weight"] != 42 || main["increment"] != "Major" || flat["tag-prefix"] != "custom-" {
			t.Fatal(flat)
		}
	case "OverrideConfigAcceptsWorkflowOnlyInEitherVersion":
		version = cliArg[string](t, args[0])
		if got := readOverride(ok(override("Workflow=GitHubFlow/v1"))); len(got) != 1 || got["workflow"] != "GitHubFlow/v1" {
			t.Fatal(got)
		}
	case "OverrideConfigRejectsNestedWorkflowWithRootReplacement":
		version = "v7"
		path := cliArg[string](t, args[0]) + ".workflow"
		fail(override(path+"=GitHubFlow/v1"), path, "'workflow'")
	case "OverrideConfigPreservesV7BranchNameCasing", "OverrideConfigPreservesV6BranchNameCasing", "OverrideConfigSupportsV6BranchPath":
		branch, section := "ReleaseCandidate", ""
		if name == "OverrideConfigPreservesV7BranchNameCasing" {
			version, section = "v7", "Calculation."
		}
		if name == "OverrideConfigSupportsV6BranchPath" {
			branch = "main"
		}
		got := readOverride(ok(override(section + "Branches." + branch + ".Increment=Major")))
		if version == "v7" {
			got = got["calculation"].(map[string]any)
		}
		options := got["branches"].(map[string]any)[branch].(map[string]any)
		if options["increment"] != "Major" {
			t.Fatal(options)
		}
	case "OverrideConfigRejectsDottedV7BranchNameWithClearDiagnostic":
		version = "v7"
		fail(override("calculation.branches.release.1.increment=Major"), "Branch name 'release.1' contains '.'", "Use a configuration file instead")
	case "OverrideConfigRejectsV6PathInV7WithReplacement":
		version = "v7"
		fail(override("tag-prefix=custom-"), "calculation.tag-prefix", "config migrate")
	case "OverrideConfigRejectsPropertyInWrongV7SectionWithReplacement":
		version = "v7"
		fail(override("calculation.update-build-number=false"), "output.update-build-number")
	case "OverrideConfigRejectsV7PathInV6WithReplacement":
		fail(override("output.update-build-number=false"), "update-build-number", "config migrate")
	case "OverrideConfigBatchValidationDoesNotApplyAnyValues":
		data, err := config.ParseCLIOverrides([]string{"calculation.tag-prefix=custom-", "tag-prefix=legacy"}, "v7", "--override-config")
		if err == nil || len(data) != 0 {
			t.Fatalf("%q %v", data, err)
		}
	case "OverrideconfigWithInvalidOption":
		_, err := parse(override(cliArg[string](t, args[0])))
		if err == nil || err.Error() != cliArg[string](t, args[1]) {
			t.Fatal(err)
		}
	case "CheckVerbosityParsing":
		got, err := parseVerbosity(cliArg[string](t, args[0]))
		if cliArg[bool](t, args[1]) {
			if err == nil {
				t.Fatal(got)
			}
		} else if err != nil || got != cliArg[string](t, args[2]) {
			t.Fatalf("%s %v", got, err)
		}
	case "ThrowIfConfigurationFileDoesNotExist":
		path := cliArg[string](t, args[0])
		if filepath.IsAbs(path) {
			path = filepath.Join(cwd, filepath.Base(path))
		}
		fail(flagName("--config", "-config") + " " + path)
	case "EnsureConfigurationFileIsSet", "ShortAliasCIsConfigNotCommit":
		path := writeQueryConfig(t, cwd, "config.yaml", "next-version: 1.0.0")
		option := flagName("--config", "-config")
		if name == "ShortAliasCIsConfigNotCommit" {
			option = "-c"
		}
		got := ok(option + " " + path)
		if got.ConfigurationFile != path || (name == "ShortAliasCIsConfigNotCommit" && got.CommitID != "") {
			t.Fatal(got.ConfigurationFile, got.CommitID)
		}
	case "UpdateAssemblyInfoWithFilename", "UpdateAssemblyInfoWithMultipleFilenames", "UpdateProjectFilesWithMultipleFilenames", "UpdateAssemblyInfoWithMultipleFilenamesMatchingGlobbing", "UpdateAssemblyInfoWithRelativeFilename":
		checkAssemblyFileCases(t, cwd, name, flagName, ok)
	default:
		t.Fatal(name)
	}
}

func checkAssemblyFileCases(t *testing.T, cwd, name string, flagName func(string, string) string, parse func(string) cliArguments) {
	t.Helper()
	target := cwd
	files := []string{"CommonAssemblyInfo.cs"}
	switch name {
	case "UpdateAssemblyInfoWithMultipleFilenames":
		files = append(files, "VersionAssemblyInfo.cs")
	case "UpdateProjectFilesWithMultipleFilenames":
		files = []string{"CommonAssemblyInfo.csproj", "VersionAssemblyInfo.csproj"}
	case "UpdateAssemblyInfoWithMultipleFilenamesMatchingGlobbing":
		files = append(files, "VersionAssemblyInfo.cs", filepath.Join("subdir", "LocalAssemblyInfo.cs"))
	case "UpdateAssemblyInfoWithRelativeFilename":
		target = filepath.Join(cwd, "subdir1", "subdir2")
	}
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		path := filepath.Join(cwd, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	command := flagName("--target-path", "-targetpath") + " " + target + " "
	project := name == "UpdateProjectFilesWithMultipleFilenames"
	if project {
		command += flagName("--update-project-files", "-updateProjectFiles")
	} else {
		command += flagName("--update-assembly-info", "-updateAssemblyInfo")
	}
	switch name {
	case "UpdateAssemblyInfoWithRelativeFilename":
		command += ` ..\..\CommonAssemblyInfo.cs`
	case "UpdateAssemblyInfoWithMultipleFilenamesMatchingGlobbing":
		command += " **/*AssemblyInfo.cs"
	default:
		command += " " + strings.Join(files, " ")
	}
	got := parse(command)
	if got.UpdateAssemblyInfo == project || got.UpdateProjectFiles != project || len(got.UpdateAssemblyInfoFileName) != len(files) {
		t.Fatal(got.UpdateAssemblyInfoFileName)
	}
	for _, file := range files {
		if !slices.ContainsFunc(got.UpdateAssemblyInfoFileName, func(path string) bool { return filepath.Base(path) == filepath.Base(file) }) {
			t.Fatalf("missing %s from %v", file, got.UpdateAssemblyInfoFileName)
		}
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"gopkg.in/yaml.v3"
)

func TestCSharpConfigurationMigrationCLI(t *testing.T) {
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct {
				Document *string
				Cases    []struct {
					ID        string
					Arguments []json.RawMessage
				}
			}
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-configuration-migration.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	for suite, data := range corpus.Suites {
		if suite == "ConfigurationMigrationServiceTests" {
			continue
		}
		t.Run(suite, func(t *testing.T) {
			for method, item := range data.Methods {
				t.Run(method, func(t *testing.T) {
					for _, c := range item.Cases {
						t.Run(c.ID, func(t *testing.T) { checkCLIMigrationCase(t, method, item.Document, c.Arguments) })
					}
				})
			}
		})
	}
}

type migrationResult struct {
	stdout, stderr string
	err            error
}

func runMigrationTest(args ...string) migrationResult {
	var out, logs bytes.Buffer
	err := execute(args, &out, &logs)
	if err != nil {
		logs.WriteString(err.Error() + "\n")
	}
	return migrationResult{out.String(), logs.String(), err}
}

func mustReadMigrated(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func checkCLIMigrationCase(t *testing.T, method string, doc *string, args []json.RawMessage) {
	t.Helper()
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	const original = "next-version: 2.0.0"
	const warning = "Comments cannot be preserved during migration."

	if method == "ConfigMigrateParsesItsInputAndOutputOptions" || method == "ConfigMigrateUsesPositionalTargetPath" {
		command := "path config migrate --in-place"
		if method == "ConfigMigrateParsesItsInputAndOutputOptions" {
			command = cliArg[string](t, args[0]) + " --output GitVersion.v7.yml --force"
		}
		flags, dir, ok := migrationInvocation(strings.Fields(command))
		if !ok {
			t.Fatal("migration command not recognized")
		}
		opts, err := parseMigrationOptions(flags, dir)
		if err != nil {
			t.Fatal(err)
		}
		if method == "ConfigMigrateUsesPositionalTargetPath" {
			if opts.workingDirectory != "path" {
				t.Fatal(opts)
			}
		} else if opts.input != "GitVersion.yml" || opts.output != "GitVersion.v7.yml" || !opts.force {
			t.Fatal(opts)
		}
		return
	}
	if method == "ConfigMigrateRejectsOutputAndInPlaceTogether" || method == "ConfigRequiresASubcommand" {
		command := []string{"config", "migrate", "--output", "GitVersion.v7.yml", "--in-place"}
		want := "Cannot use --output together with --in-place."
		switch method {
		case "ConfigRequiresASubcommand":
			command, want = []string{"config"}, "The 'config' command requires a subcommand. Use 'gitversion config get', 'gitversion config validate' or 'gitversion config migrate'."

		}
		r := runMigrationTest(command...)
		if r.err == nil || r.err.Error() != want {
			t.Fatalf("got %v, want %q", r.err, want)
		}
		return
	}

	if method == "InvalidConfigurationVersionDisplaysDiagnosticOnStandardError" || strings.Contains(method, "Calculation") || strings.HasPrefix(method, "Calculates") || strings.HasPrefix(method, "V6") || strings.HasPrefix(method, "ExplicitV6") {
		checkVersionSelectionCase(t, method, args)
		return
	}
	if method == "ShowConfigUsesSelectedConfigurationStructure" || method == "RootWorkflowAppliesDefaultsToBothSectionsAndPreservesOverrides" {
		checkMigrationShowCase(t, method, args)
		return
	}

	dir := t.TempDir()
	migrate := func(opts ...string) migrationResult {
		t.Helper()
		return runMigrationTest(append([]string{dir, "config", "migrate"}, opts...)...)
	}
	ok := func(r migrationResult) {
		t.Helper()
		if r.err != nil {
			t.Fatalf("migration failed: %+v", r)
		}
	}
	contains := func(text string, values ...string) {
		t.Helper()
		for _, want := range values {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q in %s", want, text)
			}
		}
	}
	file := filepath.Join(dir, "GitVersion.yml")
	switch method {
	case "InPlaceMigrationWarnsThatCommentsCannotBePreserved":
		writeQueryConfig(t, dir, "legacy.yml", original)
		r := migrate("--config", "legacy.yml", "--in-place")
		ok(r)
		contains(r.stderr, warning)
	case "InPlaceMigrationWritesWorkflowAtRootAndIsIdempotent":
		writeQueryConfig(t, dir, "GitVersion.yml", cliArg[string](t, args[0]))
		ok(migrate("--config", file, "--in-place"))
		first := mustReadMigrated(t, file)
		contains(first, "workflow: GitHubFlow/v1", "  next-version: 2.0.0")
		if strings.Contains(first, "  workflow:") {
			t.Fatal(first)
		}
		ok(migrate("--config", file, "--in-place"))
		if again := mustReadMigrated(t, file); again != first {
			t.Fatalf("not idempotent: %s", again)
		}
	case "ConfigMigrateWritesMigratedConfigurationToStandardOutput":
		writeQueryConfig(t, dir, "GitVersion.yml", original)
		r := migrate()
		ok(r)
		contains(r.stdout, "calculation:", "next-version: 2.0.0", "output: {}")
	case "ConfigMigrateDoesNotOverwriteOutputWithoutForce":
		writeQueryConfig(t, dir, "GitVersion.yml", original)
		dest := writeQueryConfig(t, dir, "GitVersion.v7.yml", "existing configuration")
		r := migrate("--output", "GitVersion.v7.yml")
		if r.err == nil || r.stdout != "" || mustReadMigrated(t, dest) != "existing configuration" {
			t.Fatalf("output changed: %+v", r)
		}
		r = migrate("--output", "GitVersion.v7.yml", "--force")
		ok(r)
		if r.stdout != "" || strings.Count(r.stderr, warning) != 1 {
			t.Fatalf("%+v", r)
		}
		contains(mustReadMigrated(t, dest), "calculation:")
	case "ConfigMigrateDiscoversRootConfigurationUnlessWorkingDirectoryHasOne":
		repo := queryFixture(t)
		rootFile := writeQueryConfig(t, repo, "GitVersion.yml", original)
		child := filepath.Join(repo, "subdirectory")
		if err := os.Mkdir(child, 0700); err != nil {
			t.Fatal(err)
		}
		want := original
		if cliArg[bool](t, args[0]) {
			want = "next-version: 3.0.0"
			writeQueryConfig(t, child, "GitVersion.yml", want)
		}
		r := runMigrationTest(child, "config", "migrate")
		ok(r)
		contains(r.stdout, want)
		if mustReadMigrated(t, rootFile) != original {
			t.Fatal("root file changed")
		}
	case "ConfigMigrateLeavesNoTemporaryFileAfterWritingOutput":
		writeQueryConfig(t, dir, "GitVersion.yml", original)
		ok(migrate("--output", "GitVersion.v7.yml"))
		if names := migrationFiles(t, dir); !slices.Equal(names, []string{"GitVersion.v7.yml", "GitVersion.yml"}) {
			t.Fatal(names)
		}
	case "ConfigMigrateRejectsInvalidInputWithoutReplacingFile":
		if doc == nil {
			t.Fatal("missing original document")
		}
		writeQueryConfig(t, dir, "GitVersion.yml", *doc)
		r := migrate("--in-place")
		if r.err == nil || r.stdout != "" || mustReadMigrated(t, file) != *doc ||
			!slices.Equal(migrationFiles(t, dir), []string{"GitVersion.yml"}) {
			t.Fatalf("invalid input changed: %+v", r)
		}
	case "ConfigMigrateInPlaceMigratesExplicitConfigurationOutsideGitRepository":
		legacy := writeQueryConfig(t, dir, "legacy.yml", original)
		r := migrate("--config", "legacy.yml", "--in-place")
		ok(r)
		if r.stdout != "" || strings.Count(r.stderr, warning) != 1 {
			t.Fatalf("%+v", r)
		}
		contains(mustReadMigrated(t, legacy), "calculation:")
	case "ConfigMigrateInPlaceUsesPositionalTargetPath":
		writeQueryConfig(t, dir, "GitVersion.yml", original)
		ok(migrate("--in-place"))
		contains(mustReadMigrated(t, file), "calculation:", original)
	case "ConfigMigrateDoesNotEmitLegacyFallbackWarning":
		t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v6")
		writeQueryConfig(t, dir, "GitVersion.yml", original)
		r := migrate()
		ok(r)
		if strings.Contains(r.stdout+r.stderr, "temporary v6 compatibility mode") {
			t.Fatalf("%+v", r)
		}
	default:
		t.Fatal(method)
	}
}

func migrationFiles(t *testing.T, dir string) []string {
	t.Helper()
	items, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, item := range items {
		names = append(names, item.Name())
	}
	return names
}

func versionFixture(t *testing.T, feature bool) (string, string) {
	t.Helper()
	dir := queryFixture(t)
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git", "info"), 0700); err != nil {
		t.Fatal(err)
	}
	writeQueryConfig(t, filepath.Join(dir, ".git", "info"), "exclude", "GitVersion.yml\nrun.log\n")
	author := &object.Signature{Name: "Parity Author", Email: "parity@example.invalid", When: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	commit := func(name string) plumbing.Hash {
		t.Helper()
		writeQueryConfig(t, dir, name, name)
		if _, err := worktree.Add(name); err != nil {
			t.Fatal(err)
		}
		sha, err := worktree.Commit(name, &git.CommitOptions{Author: author})
		if err != nil {
			t.Fatal(err)
		}
		return sha
	}
	sha := commit("first.txt")
	if feature {
		if err := worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("feature/topic"), Create: true}); err != nil {
			t.Fatal(err)
		}
		sha = commit("feature.txt")
	}
	return dir, sha.String()
}

func checkVersionSelectionCase(t *testing.T, method string, args []json.RawMessage) {
	t.Helper()
	dir, sha := versionFixture(t, method == "V6V7AndMigratedConfigurationCalculateCurrentCommitLabel")
	write := func(data string) string { return writeQueryConfig(t, dir, "GitVersion.yml", data) }
	calc := func(version string, options ...string) migrationResult {
		t.Helper()
		t.Setenv("GITVERSION_CONFIGURATION_VERSION", version)
		return runMigrationTest(append([]string{"calculate", "--repository", dir}, options...)...)
	}
	parse := func(r migrationResult) map[string]any {
		t.Helper()
		if r.err != nil {
			t.Fatalf("calculation failed: %+v", r)
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(r.stdout), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	switch method {
	case "V6V7AndMigratedConfigurationCalculateCurrentCommitLabel":
		placeholder := cliArg[string](t, args[0])
		label := "ci.{" + placeholder + "}"
		write("next-version: 2.0.0\nbranches:\n  feature:\n    label: '" + label + "'")
		v6 := calc("v6")
		migrated := runMigrationTest(dir, "config", "migrate")
		if migrated.err != nil || migrated.stdout == "" {
			t.Fatalf("%+v", migrated)
		}
		var document struct {
			Calculation struct {
				Branches map[string]struct{ Label string }
			}
		}
		if err := yaml.Unmarshal([]byte(migrated.stdout), &document); err != nil {
			t.Fatal(err)
		}
		if document.Calculation.Branches["feature"].Label != label {
			t.Fatal(document)
		}
		write(migrated.stdout)
		converted := calc("v7")
		write("calculation:\n  next-version: 2.0.0\n  branches:\n    feature:\n      label: '" + label + "'\noutput: {}")
		v7 := calc("v7")
		wantLabel := "ci." + sha
		if placeholder == "ShortSha" {
			wantLabel = "ci." + sha[:7]
		}
		wantVersion := parse(v6)["FullSemVer"]
		for _, r := range []migrationResult{v6, v7, converted} {
			got := parse(r)
			if got["PreReleaseLabelName"] != wantLabel || got["Sha"] != sha || got["ShortSha"] != sha[:7] || got["FullSemVer"] != wantVersion {
				t.Fatal(got)
			}
		}
	case "V6AndV7ConfigurationCalculateTheSameVersion":
		write("next-version: 2.0.0")
		legacy := parse(calc("v6"))
		write("calculation:\n  next-version: 2.0.0\noutput: {}")
		nested := parse(calc("v7"))
		if nested["FullSemVer"] != legacy["FullSemVer"] {
			t.Fatal(legacy, nested)
		}
	case "ExplicitV6WarningUsesStandardErrorOnceWithoutContaminatingJson":
		file := write("next-version: 2.0.0")
		var options []string
		withLog := cliArg[bool](t, args[0])
		if withLog {
			options = []string{"--log-file", "run.log"}
		}
		r := calc("v6", options...)
		if got := parse(r)["FullSemVer"]; got != "2.0.0-1" {
			t.Fatal(got)
		}
		if strings.Count(r.stderr, "temporary v6 compatibility mode") != 1 {
			t.Fatal(r.stderr)
		}
		for _, want := range []string{file, "GitVersion 7.1", "gitversion config migrate", "GITVERSION_CONFIGURATION_VERSION=v7"} {
			if !strings.Contains(r.stderr, want) {
				t.Fatalf("missing %q in %s", want, r.stderr)
			}
		}
		if withLog && !strings.Contains(mustReadMigrated(t, filepath.Join(dir, "run.log")), "temporary v6 compatibility mode") {
			t.Fatal("missing log warning")
		}
	case "CalculatesVersionWithSharedRootConfiguration":
		write(cliArg[string](t, args[1]))
		if version, ok := parse(calc(cliArg[string](t, args[0])))["FullSemVer"].(string); !ok || version == "" {
			t.Fatal(version)
		}
	case "ExplicitV6WithoutUserConfigurationDoesNotWarnOnStandardError":
		r := calc("v6")
		if version, ok := parse(r)["FullSemVer"].(string); !ok || version == "" || r.stderr != "" {
			t.Fatalf("%v: %+v", version, r)
		}
	case "DefaultCalculationDisplaysConfigurationVersionMismatchOnStandardError":
		write("next-version: 2.0.0")
		checkCalcFailure(t, calc("v7"), "An error occurred:", "uses the legacy v6 configuration structure", "gitversion config migrate")
	case "InvalidConfigurationVersionDisplaysDiagnosticOnStandardError":
		checkCalcFailure(t, calc("invalid"), "An error occurred:", "Unrecognized GITVERSION_CONFIGURATION_VERSION value 'invalid'", "Valid values are 'v6' and 'v7'")
	default:
		t.Fatal(method)
	}
}

func checkCalcFailure(t *testing.T, r migrationResult, expected ...string) {
	t.Helper()
	if r.err == nil || r.stdout != "" {
		t.Fatalf("want failure without version: %+v", r)
	}
	for _, want := range expected {
		if !strings.Contains(r.stderr, want) {
			t.Fatalf("missing %q in %s", want, r.stderr)
		}
	}
}

func checkMigrationShowCase(t *testing.T, method string, args []json.RawMessage) {
	t.Helper()
	dir := queryFixture(t)
	if method == "ShowConfigUsesSelectedConfigurationStructure" {
		t.Setenv("GITVERSION_CONFIGURATION_VERSION", cliArg[string](t, args[0]))
		r := runMigrationTest(dir, "--show-config")
		nested := cliArg[bool](t, args[1])
		if r.err != nil || strings.Contains(r.stdout, "calculation:") != nested || strings.Contains(r.stdout, "output:") != nested {
			t.Fatalf("%+v", r)
		}
		return
	}
	hasOverrides := cliArg[bool](t, args[0])
	content := "workflow: GitHubFlow/v1"
	if hasOverrides {
		content += "\ncalculation:\n  tag-prefix: custom-\n  branches:\n    main: {increment: Major}\noutput:\n  assembly-versioning-scheme: None\n  branches:\n    main: {pre-release-weight: 42}"
	}
	writeQueryConfig(t, dir, "GitVersion.yml", content)
	r := runMigrationTest(dir, "--show-config")
	if r.err != nil || !strings.Contains(r.stdout, "workflow: GitHubFlow/v1") || strings.Contains(r.stdout, "  workflow:") {
		t.Fatalf("%+v", r)
	}
	var document struct {
		Calculation struct {
			TagPrefix string `yaml:"tag-prefix"`
			Branches  map[string]struct{ Increment string }
		}
		Output struct {
			FileScheme string `yaml:"assembly-file-versioning-scheme"`
			Scheme     string `yaml:"assembly-versioning-scheme"`
			Branches   map[string]struct {
				Weight int `yaml:"pre-release-weight"`
			}
		}
	}
	if err := yaml.Unmarshal([]byte(r.stdout), &document); err != nil {
		t.Fatal(err)
	}
	prefix, scheme, increment, weight := "[vV]?", "MajorMinorPatch", "Patch", 55000
	if hasOverrides {
		prefix, scheme, increment, weight = "custom-", "None", "Major", 42
	}
	if document.Output.FileScheme != "MajorMinorPatch" || document.Calculation.TagPrefix != prefix ||
		document.Output.Scheme != scheme || document.Calculation.Branches["main"].Increment != increment || document.Output.Branches["main"].Weight != weight {
		t.Fatal(document)
	}
}

func TestMigrationPublishDoesNotClobberConcurrentOutput(t *testing.T) {
	dir := t.TempDir()
	path := writeQueryConfig(t, dir, "out.yml", "concurrent writer")
	err := writeFileAtomically(path, []byte("calculation: {}\noutput: {}\n"), nil, false)
	if !os.IsExist(err) || mustReadMigrated(t, path) != "concurrent writer" ||
		!slices.Equal(migrationFiles(t, dir), []string{"out.yml"}) {
		t.Fatalf("concurrent destination or temp file changed: %v", err)
	}
}

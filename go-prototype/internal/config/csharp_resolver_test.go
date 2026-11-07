package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"gopkg.in/yaml.v3"
)

func TestCSharpConfigurationResolution(t *testing.T) {
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
	raw, err := os.ReadFile("../../testdata/csharp-configuration-resolver.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	for suite, data := range corpus.Suites {
		t.Run(suite, func(t *testing.T) {
			for method, item := range data.Methods {
				t.Run(method, func(t *testing.T) {
					for _, c := range item.Cases {
						t.Run(c.ID, func(t *testing.T) {
							switch suite {
							case "ConfigurationProviderTests":
								checkProviderCase(t, method, item.Document, c.Arguments)
							case "ConfigurationResolverTests":
								checkResolverCase(t, method, c.Arguments)
							case "ConfigurationHelperTests":
								checkMergeCase(t, method)
							default:
								t.Fatal(suite)
							}
						})
					}
				})
			}
		})
	}
}

func arg[T any](t *testing.T, input json.RawMessage) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(input, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func mustResolve(t *testing.T, data, overrides, version string) ResolvedConfiguration {
	t.Helper()
	got, err := ResolveConfiguration([]byte(data), []byte(overrides), version)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func checkField(t *testing.T, node *yaml.Node, name string, want any) {
	t.Helper()
	field := nodeField(node, name)
	var got any
	if field != nil && field.Tag != "!!null" {
		if err := field.Decode(&got); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}

func branchNode(node *yaml.Node, name string) *yaml.Node {
	return nodeField(nodeField(node, "branches"), name)
}

func sourceList(node *yaml.Node) []string {
	var values []string
	if node != nil {
		for _, item := range node.Content {
			values = append(values, item.Value)
		}
	}
	return values
}

func serializeResolved(t *testing.T, cfg ResolvedConfiguration, version string) string {
	t.Helper()
	data, err := cfg.Serialize(version)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeProviderDocument(t *testing.T, dir string, doc *string) string {
	t.Helper()
	path := filepath.Join(dir, "GitVersion.yml")
	if doc != nil {
		if err := os.WriteFile(path, []byte(*doc), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func checkProviderCase(t *testing.T, method string, doc *string, args []json.RawMessage) {
	t.Helper()
	dir := t.TempDir()
	var messages []string
	provider := Provider{Version: "v6", Locator: FileLocator{Log: func(s string) { messages = append(messages, s) }}}
	provide := func(overrides string) ResolvedConfiguration {
		t.Helper()
		got, err := provider.Provide(&dir, []byte(overrides))
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	text := pathValue(doc)
	switch method {
	case "ProvidesNestedV7ConfigurationAndMergesOutputOnlyWorkflowBranch", "RejectsMixedV7Configuration":
		provider.Version = "v7"
	case "DoesNotWarnWhenV6IsNotExplicitlySelected":
		provider.Version = pathValue(arg[*string](t, args[0]))
	case "SourceBranchesValidationShouldSucceedForWellKnownBranches":
		text = strings.ReplaceAll(text, "{wellKnownBranchKey}", arg[string](t, args[0]))
	case "ShouldOverrideTagPrefixWithOverrideConfigValue":
		if arg[bool](t, args[0]) {
			text = "tag-prefix: custom-tag-prefix-from-yml"
		}
	case "V7SerializationRoundTripsDefaultsAndBuiltInWorkflows":
		workflow := arg[*string](t, args[0])
		if workflow != nil {
			text = "workflow: " + *workflow
			doc = &text
		}
	}
	if doc != nil || method == "ShouldOverrideTagPrefixWithOverrideConfigValue" {
		doc = &text
	}
	path := writeProviderDocument(t, dir, doc)

	switch method {
	case "ProvidesNestedV7ConfigurationAndMergesOutputOnlyWorkflowBranch":
		nested, err := nestDocument(provide("").flat)
		if err != nil {
			t.Fatal(err)
		}
		calc, out := nodeField(nested, "calculation"), nodeField(nested, "output")
		checkField(t, calc, "tag-prefix", "custom-")
		checkField(t, out, "update-build-number", false)
		if branchNode(calc, "develop") == nil {
			t.Fatal("missing calculation develop branch")
		}
		checkField(t, branchNode(out, "develop"), "custom-version-format", "{SemVer}")
		checkField(t, branchNode(out, "develop"), "pre-release-weight", 42)
	case "ResolvesEquivalentV6AndV7Configuration":
		legacy := "tag-prefix: custom-\nupdate-build-number: false\nbranches:\n  main: {increment: Major, pre-release-weight: 42}"
		nested := "calculation:\n  tag-prefix: custom-\n  branches:\n    main: {increment: Major}\noutput:\n  update-build-number: false\n  branches:\n    main: {pre-release-weight: 42}"
		writeProviderDocument(t, dir, &legacy)
		want := serializeResolved(t, provide(""), "v6")
		provider.Version = "v7"
		writeProviderDocument(t, dir, &nested)
		if got := serializeResolved(t, provide(""), "v6"); got != want {
			t.Fatalf("legacy and v7 resolution differ\n%s\n%s", want, got)
		}
	case "V7SerializationRoundTripsDefaultsAndBuiltInWorkflows":
		original := provide("")
		nested := serializeResolved(t, original, "v7")
		if !strings.Contains(nested, "calculation:") || !strings.Contains(nested, "output:") {
			t.Fatal(nested)
		}
		node, err := readConfigurationMap([]byte(nested))
		if err != nil {
			t.Fatal(err)
		}
		flat, err := normalizeDocument(node, "v7", "configuration document")
		if err != nil || flat == nil {
			t.Fatalf("round trip returned %v, %v", flat, err)
		}
		if got, want := serializeResolved(t, ResolvedConfiguration{flat: flat}, "v6"), serializeResolved(t, original, "v6"); got != want {
			t.Fatalf("legacy serialization differs after v7 round trip\n%s\n%s", want, got)
		}
	case "RejectsMixedV7Configuration", "RegexIsRequired", "SourceBranchesValidationShouldFailWhenMatchingBranchConfigurationIsMissing", "DoesNotWarnWhenExplicitV6ConfigurationFailsNormalization":
		_, err := provider.Provide(&dir, nil)
		if err == nil {
			t.Fatal("expected configuration error")
		}
		switch method {
		case "RejectsMixedV7Configuration":
			if !strings.Contains(err.Error(), "unsupported root-level properties") {
				t.Fatal(err)
			}
		case "RegexIsRequired":
			if err.Error() != "Configuration property 'branches.bug.regex': Branch configuration requires a regex." {
				t.Fatal(err)
			}
		case "SourceBranchesValidationShouldFailWhenMatchingBranchConfigurationIsMissing":
			if err.Error() != "Configuration property 'branches.bug.source-branches': Referenced branch configuration does not exist." {
				t.Fatal(err)
			}
		default:
			checkNoLegacyWarning(t, messages)
		}
	case "CanConfigureVersionBumpResetMessage":
		checkField(t, provide("").flat, "version-bump-reset-message", "baseline:")
	case "SourceBranchesValidationShouldSucceedForWellKnownBranches":
		got := sourceList(nodeField(branchNode(provide("").flat, "bug"), "source-branches"))
		if !slices.Equal(got, []string{arg[string](t, args[0])}) {
			t.Fatal(got)
		}
	case "CanProvideConfigForNewBranch":
		bug := branchNode(provide("").flat, "bug")
		checkField(t, bug, "regex", `bug[\/-]`)
		checkField(t, bug, "label", "bugfix")
	case "NextVersionCanBeInteger":
		checkField(t, provide("").flat, "next-version", "2.0")
	case "NextVersionCanHaveEnormousMinorVersion":
		checkField(t, provide("").flat, "next-version", "2.118998723")
	case "CanUpdateAssemblyInformationalVersioningScheme", "CanUpdateAssemblyInformationalVersioningSchemeWithMultipleVariables", "CanUpdateAssemblyInformationalVersioningSchemeWithFullSemVer":
		wanted := map[string]string{
			"CanUpdateAssemblyInformationalVersioningScheme":                      "{NugetVersion}",
			"CanUpdateAssemblyInformationalVersioningSchemeWithMultipleVariables": "{Major}.{Minor}.{Patch}",
			"CanUpdateAssemblyInformationalVersioningSchemeWithFullSemVer":        "{FullSemVer}",
		}
		scheme := "MajorMinor"
		if wanted[method] == "{FullSemVer}" {
			scheme = "MajorMinorPatch"
		}
		flat := provide("").flat
		checkField(t, flat, "assembly-versioning-scheme", scheme)
		checkField(t, flat, "assembly-file-versioning-scheme", "MajorMinorPatch")
		checkField(t, flat, "assembly-informational-format", wanted[method])
	case "CanReadCustomVersionFormat":
		checkField(t, provide("").flat, "custom-version-format", "{Major}.{Minor}.{Patch}{PreReleaseLabelName:l}{PreReleaseNumber}")
	case "CanReadBranchSpecificCustomVersionFormat":
		flat := provide("").flat
		checkField(t, flat, "custom-version-format", "{SemVer}")
		checkField(t, branchNode(flat, "feature"), "custom-version-format", "{Major}.{Minor}.{Patch}{PreReleaseLabelName:l}{PreReleaseNumber}")
	case "CanOverrideCustomVersionFormat":
		checkField(t, provide("custom-version-format: '{Major:00}{Minor:00}{Patch:000}'").flat, "custom-version-format", "{Major:00}{Minor:00}{Patch:000}")
	case "CanReadDefaultDocument":
		flat := provide("").flat
		checkField(t, flat, "assembly-versioning-scheme", "MajorMinorPatch")
		checkField(t, flat, "assembly-file-versioning-scheme", "MajorMinorPatch")
		checkField(t, flat, "assembly-informational-format", nil)
		checkField(t, flat, "custom-version-format", nil)
		checkField(t, branchNode(flat, "develop"), "label", "alpha")
		checkField(t, branchNode(flat, "release"), "label", "beta")
		checkField(t, flat, "tag-prefix", "[vV]?")
		checkField(t, flat, "next-version", nil)
	case "WarnsOnceWhenExplicitV6LoadsAConfigurationFile":
		provide("")
		provide("")
		log := strings.Join(messages, "\n")
		for _, want := range []string{"Configuration file '" + path + "' uses the temporary v6 compatibility mode.", "GitVersion 7.1", "gitversion config migrate"} {
			if !strings.Contains(log, want) {
				t.Fatalf("missing %q from %s", want, log)
			}
		}
		if strings.Count(log, "temporary v6 compatibility mode") != 1 {
			t.Fatal(log)
		}
	case "DoesNotWarnWhenV6IsNotExplicitlySelected", "DoesNotWarnForExplicitV6BuiltInDefaults":
		provide("")
		checkNoLegacyWarning(t, messages)
	case "ShouldUseSpecifiedSourceBranchesForDevelop", "ShouldUseSpecifiedSourceBranchesForFeature":
		name, want := "develop", []string{"develop"}
		if method == "ShouldUseSpecifiedSourceBranchesForFeature" {
			name, want = "feature", []string{"develop", "release"}
		}
		got := sourceList(nodeField(branchNode(provide("").flat, name), "source-branches"))
		if !slices.Equal(got, want) {
			t.Fatalf("%v != %v", got, want)
		}
	case "ShouldNotOverrideAnythingWhenOverrideConfigIsEmpty":
		// Construct the same GitFlow builder default with two programmatic settings.
		defaults := mustResolve(t, "", "", "v6")
		putDocumentField(defaults.flat, "next-version", stringNode("1.2.3"))
		*nodeField(defaults.flat, "tag-prefix") = *stringNode("custom-tag-prefix-from-yml")
		actual := provide("")
		if want, got := serializeResolved(t, defaults, "v6"), serializeResolved(t, actual, "v6"); want != got {
			t.Fatalf("unexpected effective config\n%s\n%s", want, got)
		}
	case "ShouldUseDefaultTagPrefixWhenNotSetInConfigFile":
		checkField(t, provide("").flat, "tag-prefix", "[vV]?")
	case "ShouldUseTagPrefixFromConfigFileWhenProvided":
		checkField(t, provide("").flat, "tag-prefix", "custom-tag-prefix-from-yml")
	case "ShouldOverrideTagPrefixWithOverrideConfigValue":
		checkField(t, provide("tag-prefix: tag-prefix-from-override-configuration").flat, "tag-prefix", "tag-prefix-from-override-configuration")
	case "ShouldNotOverrideDefaultTagPrefixWhenNotSetInOverrideConfig":
		checkField(t, provide("next-version: 1.0.0").flat, "tag-prefix", "[vV]?")
	case "ShouldNotOverrideTagPrefixFromConfigFileWhenNotSetInOverrideConfig":
		checkField(t, provide("next-version: 1.0.0").flat, "tag-prefix", "custom-tag-prefix-from-yml")
	case "ShouldOverrideTagPrefixFromConfigFileWhenSetInOverrideConfig":
		checkField(t, provide("tag-prefix: custom-tag-prefix-from-console").flat, "tag-prefix", "custom-tag-prefix-from-console")
	default:
		t.Fatal(method)
	}
}

func checkNoLegacyWarning(t *testing.T, messages []string) {
	t.Helper()
	if strings.Contains(strings.Join(messages, "\n"), "temporary v6 compatibility mode") {
		t.Fatal(messages)
	}
}

func checkValidation(t *testing.T, doc, code, path, redacted string) {
	t.Helper()
	_, err := ResolveConfiguration([]byte(doc), nil, "v7")
	var typed *ValidationError
	if !errors.As(err, &typed) || typed.Code != code || typed.PropertyPath != path || (redacted != "" && strings.Contains(err.Error(), redacted)) {
		t.Fatalf("expected %s at %s, got %v", code, path, err)
	}
}

func checkResolverCase(t *testing.T, method string, args []json.RawMessage) {
	t.Helper()
	switch method {
	case "PureResolutionMatchesProviderWithFileAndOverrides":
		doc := "workflow: " + arg[string](t, args[0]) + "\ncalculation:\n  tag-prefix: file-\noutput:\n  tag-pre-release-weight: 12"
		overrides := "calculation:\n  tag-prefix: runtime-\noutput:\n  tag-pre-release-weight: 0"
		dir := t.TempDir()
		writeProviderDocument(t, dir, &doc)
		provider := Provider{Version: "v7"}
		fromFile, err := provider.Provide(&dir, []byte(overrides))
		if err != nil {
			t.Fatal(err)
		}
		pure := mustResolve(t, doc, overrides, "v7")
		if serializeResolved(t, pure, "v7") != serializeResolved(t, fromFile, "v7") {
			t.Fatal("provider differs from resolver")
		}
		checkField(t, pure.flat, "tag-prefix", "runtime-")
		checkField(t, pure.flat, "tag-pre-release-weight", 0)
		if b := nodeField(pure.flat, "branches"); b == nil || len(b.Content) == 0 {
			t.Fatal("missing workflow branches")
		}
	case "InvalidPublicInputReportsThePropertyPath":
		checkValidation(t, arg[string](t, args[0]), "GVCONFIG_PROPERTY", arg[string](t, args[1]), "")
	case "AllConfiguredRegexesAreCheckedWithPublicPaths":
		path := arg[string](t, args[0])
		parts := strings.Split(path, ".")
		lines := []string{"calculation:"}
		for i, part := range parts {
			lines = append(lines, strings.Repeat("  ", i+1)+part+":")
		}
		value := " '['"
		if strings.HasPrefix(path, "ignore.") {
			value = " ['[']"
		}
		lines[len(lines)-1] += value
		checkValidation(t, strings.Join(lines, "\n"), "GVCONFIG_REGEX", "calculation."+path, "'['")
	case "CustomBranchRelationshipsResolveWithoutRepositoryHistory":
		cfg := mustResolve(t, "calculation:\n  branches:\n    custom:\n      regex: '^custom/'\n      source-branches: [main]\n      is-source-branch-for: [feature]", "", "v7")
		if !slices.Contains(sourceList(nodeField(branchNode(cfg.flat, "feature"), "source-branches")), "custom") {
			t.Fatal("source branch missing")
		}
		checkField(t, branchNode(cfg.flat, "custom"), "regex", "^custom/")
	case "NextVersionUsesTheRuntimeSemanticVersionFormat":
		next, format := arg[string](t, args[0]), arg[string](t, args[1])
		doc := "calculation:\n  next-version: " + next + "\n  semantic-version-format: " + format
		if arg[bool](t, args[2]) {
			checkField(t, mustResolve(t, doc, "", "v7").flat, "next-version", next)
		} else {
			checkValidation(t, doc, "GVCONFIG_VALUE", "calculation.next-version", next)
		}
	case "InternalOverridesCanUseTypedEnumsAndCollections":
		set := map[string]struct{}{"develop": {}}
		var sources []string
		for name := range set {
			sources = append(sources, name)
		}
		runtime, err := yaml.Marshal(map[string]any{
			"semantic-version-format": semver.Loose, "next-version": "2.1",
			"branches": map[string]any{"main": map[string]any{"source-branches": sources}},
		})
		if err != nil {
			t.Fatal(err)
		}
		cfg := mustResolve(t, "{}", string(runtime), "v7")
		checkField(t, cfg.flat, "semantic-version-format", "Loose")
		checkField(t, cfg.flat, "next-version", "2.1")
		if got := sourceList(nodeField(branchNode(cfg.flat, "main"), "source-branches")); !slices.Equal(got, []string{"develop"}) {
			t.Fatal(got)
		}
	default:
		t.Fatal(method)
	}
}

func checkMergeCase(t *testing.T, method string) {
	t.Helper()
	parse := func(text string) *yaml.Node {
		t.Helper()
		got, err := readConfigurationMap([]byte(text))
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	switch method {
	case "Override_Replaces_Nested_Dictionary_With_Scalar_Value":
		current := parse("key: {nested: value}")
		mergeConfiguration(current, parse("key: override"))
		checkField(t, current, "key", "override")
	case "Override_Merges_Nested_Dictionaries_Recursively":
		current := parse("key: {a: 1, b: 2}")
		mergeConfiguration(current, parse("key: {b: 3, c: 4}"))
		for name, want := range map[string]int{"a": 1, "b": 3, "c": 4} {
			checkField(t, nodeField(current, "key"), name, want)
		}
	case "Override_Clones_New_Nested_Dictionaries":
		current, overlay := parse("{}"), parse("key: {a: 1}")
		mergeConfiguration(current, overlay)
		nodeField(nodeField(overlay, "key"), "a").Value = "2"
		checkField(t, nodeField(current, "key"), "a", 1)
	default:
		t.Fatal(method)
	}
}

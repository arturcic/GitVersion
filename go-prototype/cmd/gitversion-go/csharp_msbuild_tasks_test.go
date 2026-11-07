package main

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/output"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

type msbuildTaskCase struct {
	ID, Attribute, Config         string
	Azure, Github                 bool
	Arguments, Contains, Excludes []string
	Properties                    map[string]string
	EqualProperties               [][2]string `json:"equal_properties"`
	NonemptyProperties            []string    `json:"nonempty_properties"`
}

type msbuildTasksCorpus struct {
	Properties []string
	Suites     map[string]struct {
		Methods map[string]struct{ Cases []msbuildTaskCase }
	}
}

func readMSBuildTasks(t *testing.T) msbuildTasksCorpus {
	t.Helper()
	var corpus msbuildTasksCorpus
	if err := parity.ReadJSON("../../testdata/csharp-msbuild-tasks.json", &corpus); err != nil {
		t.Fatal(err)
	}
	return corpus
}

func TestCSharpMSBuildTasks(t *testing.T) {
	corpus := readMSBuildTasks(t)
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
	for suite, group := range corpus.Suites {
		for method, item := range group.Methods {
			if strings.Contains(method, "WhenRunWithMsBuild") {
				continue
			}
			t.Run(method, func(t *testing.T) {
				for _, tc := range item.Cases {
					t.Run(tc.ID, func(t *testing.T) {
						if method == "OutputsShouldMatchVariableProvider" {
							checkMSBuildTaskProperties(t, corpus.Properties)
							return
						}
						dir := t.TempDir()
						t.Setenv("TMPDIR", dir)
						if tc.Azure {
							t.Setenv("TF_BUILD", "true")
						}
						githubEnv := filepath.Join(dir, "github.env")
						if tc.Github {
							t.Setenv("GITHUB_ACTIONS", "true")
							t.Setenv("GITHUB_ENV", githubEnv)
						}
						if suite == "WriteVersionInfoTest" && len(tc.Arguments) != 0 {
							t.Setenv("BUILD_BUILDNUMBER", tc.Arguments[0])
						}
						work := msbuildVersionRepository(t, dir, tc.Azure || tc.Github, tc.Config)
						language := "C#"
						if suite == "UpdateAssemblyInfoTaskTest" {
							language = tc.Arguments[0]
						}
						project := filepath.Join(work, "App."+msbuildProjectExtension(t, language))
						writeMSBuildFixture(t, project, "<Project/>")
						writeMSBuildFixture(t, filepath.Join(work, ".git", "info", "exclude"), filepath.Base(project)+"\n")
						versionFile := writeMSBuildTaskVersionFile(t, dir, work)
						command := map[string]string{"GetVersionTaskTests": "get-version", "WriteVersionInfoTest": "write-version-info", "UpdateAssemblyInfoTaskTest": "update-assembly-info"}[suite]
						args := []string{"msbuild", command, "--project-file", project, "--version-file", versionFile}
						if suite == "UpdateAssemblyInfoTaskTest" {
							args = append(args, "--language", language)
							if strings.Contains(method, "IntermediateOutputPath") {
								args = append(args, "--intermediate-output-path", filepath.Join(dir, "obj"))
							}
						}
						var stdout, stderr bytes.Buffer
						if err := execute(args, &stdout, &stderr); err != nil || stderr.Len() != 0 {
							t.Fatalf("MSBuild %s: %v, stderr %q", command, err, &stderr)
						}
						switch suite {
						case "UpdateAssemblyInfoTaskTest":
							path := strings.TrimSpace(stdout.String())
							ext, _ := languageExtension(language)
							if !strings.HasPrefix(filepath.Base(path), "AssemblyInfo") || !strings.HasSuffix(path, ".g."+ext) {
								t.Fatalf("generated path %q", path)
							}
							assertAssemblyInfoTaskFile(t, path, tc.Attribute)
						case "GetVersionTaskTests":
							assertMSBuildTaskXML(t, stdout.Bytes(), tc)
						case "WriteVersionInfoTest":
							log := stdout.String()
							if tc.Github {
								file, err := os.ReadFile(githubEnv)
								if err != nil {
									t.Fatal(err)
								}
								log = string(file)
							}
							assertMSBuildTaskLog(t, log, tc)
						}
					})
				}
			})
		}
	}
}

func checkMSBuildTaskProperties(t *testing.T, expected []string) {
	t.Helper()
	var file struct {
		Targets []struct {
			Name  string `xml:"Name,attr"`
			Peeks []struct {
				Outputs []struct {
					Property string `xml:"PropertyName,attr"`
				} `xml:"Output"`
			} `xml:"XmlPeek"`
		} `xml:"Target"`
	}
	raw, err := os.ReadFile("../../msbuild/GitVersion.Go.targets")
	if err != nil {
		t.Fatal(err)
	}
	if err := xml.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	var actual []string
	for _, target := range file.Targets {
		if target.Name == "GetGitVersionGoVersion" {
			for _, peek := range target.Peeks {
				for _, item := range peek.Outputs {
					actual = append(actual, strings.TrimPrefix(item.Property, "GitVersion_"))
				}
			}
		}
	}
	slices.Sort(actual)
	want := slices.Sorted(slices.Values(expected))
	provider := slices.Sorted(slices.Values(output.VariableNames))
	if !slices.Equal(actual, want) || !slices.Equal(provider, want) {
		t.Fatalf("MSBuild properties %v, provider %v, want %v", actual, provider, want)
	}
}

func writeMSBuildTaskVersionFile(t *testing.T, dir, work string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if err := execute([]string{work, "--output", "json"}, &stdout, &stderr); err != nil || stderr.Len() != 0 {
		t.Fatalf("calculate task version: %v, %s", err, &stderr)
	}
	path := filepath.Join(dir, "version.json")
	writeMSBuildFixture(t, path, stdout.String())
	return path
}

func assertAssemblyInfoTaskFile(t *testing.T, path, attribute string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil || !strings.Contains(strings.ToLower(string(raw)), strings.ToLower(attribute)) {
		t.Fatalf("expected %q in %s, error %v\n%s", attribute, path, err, raw)
	}
}

func assertMSBuildTaskXML(t *testing.T, data []byte, tc msbuildTaskCase) {
	t.Helper()
	var root struct {
		XMLName xml.Name `xml:"GitVersion"`
		Values  []struct {
			XMLName xml.Name
			Value   string `xml:",chardata"`
		} `xml:",any"`
	}
	if err := xml.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, field := range root.Values {
		got[field.XMLName.Local] = field.Value
	}
	for name, expected := range tc.Properties {
		if value, ok := got[name]; !ok || value != expected {
			t.Errorf("property %s = %q, want %q", name, value, expected)
		}
	}
	for _, name := range tc.NonemptyProperties {
		if got[name] == "" {
			t.Errorf("property %s is empty", name)
		}
	}
	for _, pair := range tc.EqualProperties {
		if got[pair[0]] != got[pair[1]] {
			t.Errorf("properties %s and %s differ", pair[0], pair[1])
		}
	}
}

func assertMSBuildTaskLog(t *testing.T, log string, tc msbuildTaskCase) {
	t.Helper()
	for _, value := range tc.Contains {
		if !strings.Contains(log, value) {
			t.Errorf("build output is missing %q", value)
		}
	}
	for _, value := range tc.Excludes {
		if strings.Contains(log, value) {
			t.Errorf("build output contains unexpected %q", value)
		}
	}
}

func TestMSBuildAssemblyUpdateRejectsConflictBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "App.csproj")
	assembly := filepath.Join(dir, "Properties", "AssemblyInfo.cs")
	list := filepath.Join(dir, "compile-items.txt")
	writeMSBuildFixture(t, project, "<Project/>")
	writeMSBuildFixture(t, assembly, `[assembly: AssemblyVersion("1.0.0.0")]`)
	writeMSBuildFixture(t, list, "\ufeff"+assembly+"\r\n")
	outputDir := filepath.Join(dir, "obj")
	var stdout, stderr bytes.Buffer
	err := execute([]string{"msbuild", "update-assembly-info", "--project-file", project,
		"--compile-list", list, "--intermediate-output-path", outputDir}, &stdout, &stderr)
	want := "File contains assembly version attributes which conflict with the attributes generated by GitVersion " + assembly
	if err == nil || err.Error() != want || stdout.Len() != 0 {
		t.Fatalf("expected %q with no generated path, got %v and %q", want, err, &stdout)
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("conflicting compile items must leave the output directory absent, stat error %v", err)
	}
}

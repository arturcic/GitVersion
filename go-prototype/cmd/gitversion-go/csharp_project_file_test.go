package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/output"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

func TestCSharpProjectFileUpdater(t *testing.T) {
	var corpus struct {
		Methods map[string][]struct {
			ID, XML, Warning string
			ExpectedXML      string `json:"expected_xml"`
			FileName         string `json:"file_name"`
			CanUpdate        *bool  `json:"can_update"`
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-project-file.json", &corpus); err != nil {
		t.Fatal(err)
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.ID, func(t *testing.T) {
					project, err := parseProjectXML(tc.XML)
					if err != nil {
						t.Fatal(err)
					}
					groups, warning, err := updatableProjectGroups(project)
					if err != nil {
						t.Fatal(err)
					}
					if tc.CanUpdate != nil {
						if canUpdate := len(groups) != 0; canUpdate != *tc.CanUpdate {
							t.Fatalf("can update = %v, want %v", canUpdate, *tc.CanUpdate)
						}
						if tc.Warning == "" {
							if warning != "" {
								t.Fatalf("unexpected warning %q", warning)
							}
						} else if strings.Contains(warning, "\n") || !strings.Contains(warning, tc.Warning) {
							t.Fatalf("expected one warning containing %q, got %q", tc.Warning, warning)
						}
						return
					}

					input := output.VariableInput{Version: semver.Version{Major: 2}}
					if tc.FileName != "" {
						count := int64(3)
						input.Version = semver.Version{Major: 2, Minor: 3, Patch: 1}
						input.Branch, input.SHA, input.ShortSHA = "foo", "hash", "shortHash"
						input.Build, input.Distance = &count, count
					}
					values, err := output.FromSemanticVersion(input, config.OutputSettings{}, os.LookupEnv)
					if err != nil {
						t.Fatal(err)
					}
					version, ok := values["AssemblySemVer"].(string)
					if !ok || version == "" {
						t.Fatal("variable provider returned no assembly version")
					}
					var actual string
					if tc.FileName == "" {
						setProjectVersionElement(groups, "AssemblyVersion", version)
						actual = project.render()
					} else {
						dir := t.TempDir()
						path := filepath.Join(dir, tc.FileName)
						if err := os.WriteFile(path, []byte(tc.XML), 0644); err != nil {
							t.Fatal(err)
						}
						var diagnostics bytes.Buffer
						if err := updateProjectFiles(dir, []string{path}, values, &diagnostics); err != nil || diagnostics.Len() != 0 {
							t.Fatalf("project update: %v, %q", err, &diagnostics)
						}
						data, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						actual = string(data)
					}
					if actual != tc.ExpectedXML {
						t.Fatalf("project XML differs\ngot:\n%s\nwant:\n%s", actual, tc.ExpectedXML)
					}
				})
			}
		})
	}
}

func TestProjectUpdaterKeepsEarlierFileWhenAnotherIsMalformed(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "First.csproj")
	bad := filepath.Join(dir, "Second.csproj")
	const content = `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup/></Project>`
	for path, data := range map[string]string{first: content, bad: "<Project><Broken></Project>"} {
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	var diagnostics bytes.Buffer
	values := map[string]any{"AssemblySemVer": "1.2.3.0"}
	if err := updateProjectFiles(dir, []string{first, bad}, values, &diagnostics); err == nil || !strings.Contains(err.Error(), bad) {
		t.Fatalf("expected XML error for %s, got %v", bad, err)
	}
	got, err := os.ReadFile(first)
	if err != nil || string(got) != content {
		t.Fatalf("first project changed to %q, %v", got, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 2 {
		t.Fatalf("unexpected files after failed update: %v, %v", files, err)
	}
}

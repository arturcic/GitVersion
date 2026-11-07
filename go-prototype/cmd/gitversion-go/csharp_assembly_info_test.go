package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/output"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

func TestCSharpAssemblyInfoUpdater(t *testing.T) {
	var corpus struct {
		Methods map[string][]struct {
			ID, Approved, Scheme          string
			Arguments, Contains, Excludes []string
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-assembly-info.json", &corpus); err != nil {
		t.Fatal(err)
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.ID, func(t *testing.T) {
					parent := t.TempDir()
					dir := filepath.Join(parent, "working")
					if err := os.Mkdir(dir, 0755); err != nil {
						t.Fatal(err)
					}
					input := output.VariableInput{Version: semver.Version{Major: 1}}
					if tc.Scheme != "" {
						count := int64(3)
						input.Version = semver.Version{Major: 2, Minor: 3, Patch: 1}
						input.Branch, input.SHA, input.ShortSHA = "foo", "hash", "shortHash"
						input.Build, input.Distance = &count, count
					}
					values, err := output.FromSemanticVersion(input, config.OutputSettings{AssemblyScheme: tc.Scheme}, os.LookupEnv)
					if err != nil {
						t.Fatal(err)
					}
					if method == "ShouldStartSearchFromWorkingDirectory" {
						testAssemblyInfoSearch(t, dir, values)
						return
					}

					ext := "." + tc.Arguments[0]
					name := "AssemblyInfo" + ext
					nested := filepath.Join("src", "Project", "Properties", "VersionAssemblyInfo"+ext)
					paths := []string{name}
					ensure := false
					switch method {
					case "ShouldCreateAssemblyInfoFileWhenNotExistsAndEnsureAssemblyInfo":
						paths, ensure = []string{"VersionAssemblyInfo" + ext}, true
					case "ShouldCreateAssemblyInfoFileAtPathWhenNotExistsAndEnsureAssemblyInfo":
						paths, ensure = []string{nested}, true
					case "ShouldCreateAssemblyInfoFilesAtPathWhenNotExistsAndEnsureAssemblyInfo":
						paths, ensure = []string{name, nested}, true
					case "ShouldNotCreateAssemblyInfoFileWhenNotExistsAndNotEnsureAssemblyInfo":
						paths = []string{"NoVersionAssemblyInfo" + ext}
					}
					if strings.Contains(method, "InRelativePath") {
						paths = []string{filepath.Join("Project", "src", "Properties", name)}
					}
					if tc.Scheme != "" {
						content := ""
						if len(tc.Arguments) == 2 {
							content = tc.Arguments[1]
						}
						if err := createAssemblyInfo(filepath.Join(dir, paths[0]), content); err != nil {
							t.Fatal(err)
						}
					}
					if err := updateAssemblyInfo(dir, paths, ensure, values, io.Discard); err != nil {
						t.Fatal(err)
					}
					for _, path := range paths {
						fullPath := filepath.Join(dir, path)
						data, err := os.ReadFile(fullPath)
						if method == "ShouldNotCreateAssemblyInfoFileWhenNotExistsAndNotEnsureAssemblyInfo" {
							if !os.IsNotExist(err) {
								t.Fatalf("expected missing file, read %q, error %v", data, err)
							}
							continue
						}
						if err != nil {
							t.Fatal(err)
						}
						actual := string(data)
						if tc.Approved != "" && strings.ReplaceAll(actual, "\r\n", "\n") != tc.Approved {
							t.Fatalf("%s:\ngot  %q\nwant %q", path, actual, tc.Approved)
						}
						for _, part := range tc.Contains {
							if !strings.Contains(actual, part) {
								t.Errorf("missing %q in %q", part, actual)
							}
						}
						for _, part := range tc.Excludes {
							if strings.Contains(actual, part) {
								t.Errorf("unexpected %q in %q", part, actual)
							}
						}
					}
				})
			}
		})
	}
}

func testAssemblyInfoSearch(t *testing.T, dir string, values map[string]any) {
	t.Helper()
	inside := filepath.Join(dir, "nested", "AssemblyInfo.cs")
	outside := filepath.Join(filepath.Dir(dir), "AssemblyInfo.cs")
	other := filepath.Join(dir, "Other.cs")
	const old = `[assembly: AssemblyVersion("0.0.0.0")]`
	for _, path := range []string{inside, outside, other} {
		if err := createAssemblyInfo(path, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := updateAssemblyInfo(dir, nil, false, values, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{inside, outside, other} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if path == inside {
			if !strings.Contains(string(data), `AssemblyVersion("1.0.0.0")`) {
				t.Errorf("discovery did not update %s: %q", path, data)
			}
		} else if string(data) != old {
			t.Errorf("discovery unexpectedly updated %s: %q", path, data)
		}
	}
}

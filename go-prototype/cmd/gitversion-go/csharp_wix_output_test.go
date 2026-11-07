package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/buildagent"
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/output"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

func TestCSharpWixOutput(t *testing.T) {
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct {
				Cases []struct{ ID, Approved string }
			}
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-wix-and-output.json", &corpus); err != nil {
		t.Fatal(err)
	}
	for _, suite := range []string{"WixFileTests", "UpdateWixVersionFileTests"} {
		for method, item := range corpus.Suites[suite].Methods {
			t.Run(method, func(t *testing.T) {
				for _, tc := range item.Cases {
					t.Run(tc.ID, func(t *testing.T) {
						dir := t.TempDir()
						if suite == "WixFileTests" {
							date := time.Date(2019, 2, 20, 23, 59, 59, 0, time.UTC)
							distance := int64(5)
							variables, err := output.FromSemanticVersion(output.VariableInput{
								Version: semver.Version{Major: 1, Minor: 2, Patch: 3}, Date: &date,
								CountSHA: "versionSourceSha", SHA: "commitSha", ShortSHA: "commitShortSha",
								Branch: "develop", Build: &distance, Distance: distance,
							}, config.OutputSettings{}, os.LookupEnv)
							if err != nil {
								t.Fatal(err)
							}
							path := filepath.Join(dir, wixVersionFileName)
							if method == "UpdateWixVersionFileWhenFileAlreadyExists" {
								if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 1024*1024), 0644); err != nil {
									t.Fatal(err)
								}
							}
							if err := updateWixVersionFile(dir, variables); err != nil {
								t.Fatal(err)
							}
							data, err := os.ReadFile(path)
							if err != nil {
								t.Fatal(err)
							}
							if string(bytes.TrimPrefix(data, []byte("\ufeff"))) != tc.Approved {
								t.Fatalf("WiX text differs from the approved file\n%s", data)
							}
							return
						}
						testWixCLI(t, dir, method)
					})
				}
			})
		}
	}
}

func testWixCLI(t *testing.T, dir, method string) {
	t.Helper()
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
	cps, err := fixtures.Generate([]fixtures.Recipe{{
		ID: "wix-output", Steps: []fixtures.Step{
			{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"},
			{Op: "commit", ID: "after"}, {Op: "checkpoint", ID: "head"},
		},
	}}, filepath.Join(dir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	repo := cps[0].Path
	var stdout, diagnostics bytes.Buffer
	run := func(args ...string) {
		t.Helper()
		stdout.Reset()
		if err := execute(append([]string{repo}, args...), &stdout, &diagnostics); err != nil || diagnostics.Len() != 0 {
			t.Fatalf("CLI: %v, %q", err, &diagnostics)
		}
	}
	var variables map[string]any
	if method != "WixVersionFileCreationTest" {
		run()
		if err := json.Unmarshal(stdout.Bytes(), &variables); err != nil {
			t.Fatal(err)
		}
	}
	run("--update-wix-version-file")
	path := filepath.Join(repo, wixVersionFileName)
	if method == "WixVersionFileCreationTest" {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	definitions := readWixDefinitions(t, bytes.TrimPrefix(data, []byte("\ufeff")))
	if method == "WixVersionFileVarCountTest" && len(definitions) != len(output.VariableNames) {
		t.Fatalf("got %d definitions, want %d", len(definitions), len(output.VariableNames))
	}
	if method == "WixVersionFileContentTest" {
		for _, name := range output.VariableNames {
			value, ok := definitions[name]
			if !ok || value != buildagent.ValueText(variables[name]) {
				t.Errorf("%s = %q, present %v, want %q", name, value, ok, buildagent.ValueText(variables[name]))
			}
		}
	}
}

func readWixDefinitions(t *testing.T, data []byte) map[string]string {
	t.Helper()
	xmlReader := xml.NewDecoder(bytes.NewReader(data))
	result := map[string]string{}
	for {
		token, err := xmlReader.Token()
		if err == io.EOF {
			return result
		}
		if err != nil {
			t.Fatal(err)
		}
		if pi, ok := token.(xml.ProcInst); ok && pi.Target == "define" {
			name, value, found := strings.Cut(string(pi.Inst), "=")
			if !found {
				t.Fatalf("invalid WiX definition %q", pi.Inst)
			}
			if _, exists := result[name]; exists {
				t.Fatalf("duplicate WiX definition %q", name)
			}
			result[name] = strings.Trim(value, `"`)
		}
	}
}

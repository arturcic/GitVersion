//go:build platform

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"gopkg.in/yaml.v3"
)

// This checks the adapted configuration values against the pinned C# executable.
// Direct Go property assertions live in TestCSharpConfigurationPorts.
func TestPlatformCSharpConfigurationPorts(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var cases []fixtures.ConfigurationPort
	if err := parity.ReadJSON(filepath.Join(root, "testdata/csharp-configuration-ports.json"), &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 6 {
		t.Fatal("configuration inventory differs")
	}
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	baseDir := filepath.Join(root, ".local/baseline")
	files, err := parity.FileSet(baseDir)
	if err != nil || !reflect.DeepEqual(files, baseline.Files) {
		t.Fatal("published baseline changed", err)
	}
	spec, err := parity.FileHash(filepath.Join(root, "baseline.json"))
	if err != nil || spec != baseline.SpecSHA256 {
		t.Fatal("baseline spec changed", err)
	}
	dotnet := os.Getenv("PARITY_DOTNET")
	versions, err := exec.CommandContext(t.Context(), dotnet, "--list-runtimes").Output()
	if err != nil {
		t.Fatal(err)
	}
	runtimeDir, err := parity.RuntimeDirectory(string(versions), baseline.Spec.Runtime)
	if err != nil {
		t.Fatal(err)
	}
	runtimeFiles, err := parity.FileSet(runtimeDir)
	if err != nil {
		t.Fatal(err)
	}
	hostHash, err := parity.FileHash(dotnet)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(filepath.Join(root, ".local"), "csharp-config-ports-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)
	recipes := []fixtures.Recipe{}
	for i, tc := range cases {
		recipes = append(recipes, fixtures.Recipe{ID: fmt.Sprintf("config-port-%02d", i), Description: tc.ID, Config: tc.Config, Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "head"}}})
	}
	cps, err := fixtures.Generate(recipes, filepath.Join(dir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateConfigurations(t.Context(), cps, baseline, dotnet, baseDir, dir, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	for i, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			hash, err := parity.FileHash(filepath.Join(cps[i].Path, "GitVersion.yml"))
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "configurations", hash+"-effective/effective-config.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			var document map[string]any
			if err := yaml.Unmarshal(data, &document); err != nil {
				t.Fatal(err)
			}
			if len(tc.Expected) == 0 {
				t.Fatal("missing original assertions")
			}
			for path, want := range tc.Expected {
				var value any = document
				for _, part := range strings.Split(path, ".") {
					m, ok := value.(map[string]any)
					if !ok {
						t.Fatalf("missing path %s", path)
					}
					value, ok = m[part]
					if !ok {
						t.Fatalf("missing path %s", path)
					}
				}
				actual, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				expected, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				if string(actual) != string(expected) {
					t.Errorf("C# %s: want %s, got %s", path, expected, actual)
				}
			}
		})
	}
	after, err := parity.FileSet(baseDir)
	if err != nil || !reflect.DeepEqual(files, after) {
		t.Fatal("baseline changed during audit", err)
	}
	if err := parity.WriteJSON(filepath.Join(dir, "report.json"), map[string]any{"passed": !t.Failed(), "configurations": len(cases), "baseline_revision": baseline.Spec.Revision, "spec_sha256": spec, "runtime_files": runtimeFiles, "dotnet_sha256": hostHash, "cases": cases}); err != nil {
		t.Fatal(err)
	}
}

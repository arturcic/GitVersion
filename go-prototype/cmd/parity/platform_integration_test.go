//go:build platform

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

// The published managed assemblies are identical to the original baseline.
// The execution host/runtime must have their own platform-specific provenance;
// do not overwrite or pretend to verify the macOS host lock on another OS.
func TestPlatformCuratedParity(t *testing.T) {
	testPlatformParity(t, "curated")
}

func TestPlatformPolicyParity(t *testing.T) {
	testPlatformParity(t, "policies")
}

func TestPlatformResetParity(t *testing.T) {
	testPlatformParity(t, "resets")
}
func testPlatformParity(t *testing.T, suite string) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dotnet := os.Getenv("PARITY_DOTNET")
	exe := os.Getenv("PARITY_CANDIDATE")
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	baseDir := filepath.Join(root, ".local/baseline")
	files, err := parity.FileSet(baseDir)
	if err != nil || !reflect.DeepEqual(files, baseline.Files) {
		t.Fatal("published C# artifacts changed", err)
	}
	specHash, err := parity.FileHash(filepath.Join(root, "baseline.json"))
	if err != nil || specHash != baseline.SpecSHA256 {
		t.Fatal("baseline specification changed")
	}
	versions, err := exec.CommandContext(t.Context(), dotnet, "--list-runtimes").Output()
	if err != nil {
		t.Fatal(err)
	}
	var runtimeDir string
	for _, line := range strings.Split(string(versions), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "Microsoft.NETCore.App" && fields[1] == baseline.Spec.Runtime {
			runtimeDir = filepath.Join(strings.Trim(fields[2], "[]"), fields[1])
		}
	}
	if runtimeDir == "" {
		t.Fatal("pinned runtime unavailable")
	}
	runtimeFiles, err := parity.FileSet(runtimeDir)
	if err != nil {
		t.Fatal(err)
	}
	hostHash, err := parity.FileHash(dotnet)
	if err != nil {
		t.Fatal(err)
	}
	candidateHash, err := parity.FileHash(exe)
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(root, ".local/platform-evidence")
	if err := os.MkdirAll(parent, 0755); err != nil {
		t.Fatal(err)
	}
	workspace, err := os.MkdirTemp(parent, "run-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", workspace)
	recipes, err := fixtures.Load(filepath.Join(root, "testdata/recipes.json"))
	if err != nil {
		t.Fatal(err)
	}
	recipes = append(recipes, calculationVariants()...)
	expected := 63
	if suite == "policies" {
		recipes, err = fixtures.PolicyRecipes()
		if err != nil {
			t.Fatal(err)
		}
		expected = 250
	}
	if suite == "resets" {
		recipes = fixtures.ResetRecipes()
		expected = 192
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "recipes.json"), recipes); err != nil {
		t.Fatal(err)
	}
	cps, err := fixtures.Generate(recipes, filepath.Join(workspace, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cps) != expected {
		t.Fatal("curated inventory changed")
	}
	var golden Golden
	goldenName := "reference.json"
	if suite == "policies" {
		goldenName = "policy-reference.json"
	}
	if suite == "resets" {
		goldenName = "reset-reference.json"
	}
	if err := parity.ReadJSON(filepath.Join(root, "testdata", goldenName), &golden); err != nil {
		t.Fatal(err)
	}
	if suite != "curated" {
		if len(golden.Outputs) != expected {
			t.Fatal("portable reference inventory changed")
		}
		for _, cp := range cps {
			if _, ok := golden.Outputs[cp.ID]; !ok {
				t.Fatalf("missing portable reference %s", cp.ID)
			}
		}
	}
	commands := []parity.Command{
		{Executable: dotnet, Arguments: append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, baseline.Spec.Arguments...)},
		{Executable: exe, Arguments: []string{"calculate", "--repository", "{repo}"}},
	}
	report := Report{Mode: "platform", Calculator: "external-candidate"}
	for _, cp := range cps {
		item := CaseResult{ID: cp.ID, Status: "passed"}
		var reference []byte
		for j, command := range commands {
			name := "csharp"
			if j == 1 {
				name = "go"
			}
			path := filepath.Join(workspace, cp.ID+"-"+name)
			result, err := parity.Run(t.Context(), command, cp.Path, path, baseline.Spec.Environment, 30*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if err := saveResult(path, result); err != nil {
				t.Fatal(err)
			}
			if err := result.Validate(); err != nil {
				item.Status = "failed"
				item.Error = name + ": " + err.Error()
				break
			}
			if j == 0 {
				reference = result.Stdout
				if expected, ok := golden.Outputs[cp.ID]; ok {
					diff, err := parity.Compare(expected, reference)
					if err != nil || len(diff) > 0 {
						item.Status = "failed"
						item.Error = "portable reference differs"
						item.Differences = diff
						break
					}
				}
			} else {
				diff, err := parity.Compare(reference, result.Stdout)
				if err != nil || len(diff) > 0 {
					item.Status = "failed"
					item.Differences = diff
					if err != nil {
						item.Error = err.Error()
					}
				}
			}
		}
		if item.Status == "passed" {
			report.Passed++
		} else {
			report.Failed++
			t.Errorf("%s: %s %+v", cp.ID, item.Error, item.Differences)
		}
		report.Cases = append(report.Cases, item)
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "report.json"), report); err != nil {
		t.Fatal(err)
	}
	after, err := parity.FileHash(exe)
	if err != nil || after != candidateHash {
		t.Fatal("candidate changed")
	}
	files, err = parity.FileSet(baseDir)
	if err != nil || !reflect.DeepEqual(files, baseline.Files) {
		t.Fatal("baseline changed")
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "provenance.json"), map[string]any{"complete": report.Passed == expected, "expected": expected, "suite": suite, "passed": report.Passed, "failed": report.Failed, "platform": parity.Platform(), "candidate_sha256": candidateHash, "host_sha256": hostHash, "runtime_files": runtimeFiles, "runtime": baseline.Spec.Runtime, "source_revision": baseline.Spec.Revision, "spec_sha256": specHash, "published_files": baseline.Files}); err != nil {
		t.Fatal(err)
	}
}

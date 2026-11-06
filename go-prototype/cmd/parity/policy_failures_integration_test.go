//go:build policies

package main

import (
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInvalidPolicyFailureParity(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dotnet := os.Getenv("PARITY_DOTNET")
	baseDir := filepath.Join(root, ".local/baseline")
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	if err := parity.VerifyBaseline(baseline, filepath.Join(root, "baseline.json"), dotnet, baseDir); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, ".local/gitversion-go")
	hash, err := parity.FileHash(exe)
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(root, ".local/policy-failure-evidence")
	if err := os.MkdirAll(parent, 0755); err != nil {
		t.Fatal(err)
	}
	workspace, err := os.MkdirTemp(parent, "run-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", workspace)
	commands := []parity.Command{{Executable: dotnet, Arguments: append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, baseline.Spec.Arguments...)}, {Executable: exe, Arguments: []string{"calculate", "--repository", "{repo}"}}}
	for _, tc := range []struct{ name, setting string }{
		{"invalid-mode", "mode: ImaginaryDeployment"},
		{"invalid-increment", "increment: ImaginaryIncrement"},
		{"missing-label-capture", "label: '{BranchName}'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := fixtures.Recipe{ID: tc.name, Config: "workflow: GitHubFlow/v1\ncalculation: {branches: {main: {" + tc.setting + "}}}\n", Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "head"}}}
			if err := parity.WriteJSON(filepath.Join(workspace, tc.name+"-recipe.json"), r); err != nil {
				t.Fatal(err)
			}
			cps, err := fixtures.Generate([]fixtures.Recipe{r}, filepath.Join(workspace, tc.name))
			if err != nil {
				t.Fatal(err)
			}
			for i, command := range commands {
				name := "csharp"
				if i == 1 {
					name = "go"
				}
				path := filepath.Join(workspace, tc.name+"-"+name)
				result, err := parity.Run(t.Context(), command, cps[0].Path, path, baseline.Spec.Environment, 30*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				if err := saveResult(path, result); err != nil {
					t.Fatal(err)
				}
				if result.ExitCode != 1 || result.Before != result.After {
					t.Fatalf("%s failure contract: exit=%d, repository unchanged=%t", name, result.ExitCode, result.Before == result.After)
				}
				// The pinned C# missing-capture failure exits silently. Other
				// invalid settings, and every Go rejection, provide diagnostics.
				if (name == "go" || tc.name != "missing-label-capture") && len(result.Stderr) == 0 {
					t.Fatalf("%s omitted diagnostics for %s", name, tc.name)
				}
				if _, err := parity.Object(result.Stdout); err == nil {
					t.Fatalf("%s emitted a version for invalid config", name)
				}
			}
		})
	}
	after, err := parity.FileHash(exe)
	if err != nil || hash != after {
		t.Fatal("candidate changed")
	}
	if err := parity.VerifyBaseline(baseline, filepath.Join(root, "baseline.json"), dotnet, baseDir); err != nil {
		t.Fatal(err)
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "provenance.json"), map[string]any{"complete": !t.Failed(), "cases": 3, "candidate_sha256": hash, "behavior": "both exit 1, no version JSON, unchanged repository; Go diagnostics required; C# missing-capture failure is silent"}); err != nil {
		t.Fatal(err)
	}
}

//go:build policies

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestExpandedPolicyParity(t *testing.T) {
	recipes, err := fixtures.PolicyRecipes()
	if err != nil {
		t.Fatal(err)
	}
	testPolicyParity(t, recipes, "policy-evidence", 250)
}
func TestWorkflowParity(t *testing.T) {
	testPolicyParity(t, fixtures.WorkflowRecipes(), "workflow-evidence", 166)
}

func TestResetPolicyParity(t *testing.T) {
	testPolicyParity(t, fixtures.ResetRecipes(), "reset-evidence", 192)
}
func testPolicyParity(t *testing.T, recipes []fixtures.Recipe, directory string, expected int) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dotnet := os.Getenv("PARITY_DOTNET")
	parent := filepath.Join(root, filepath.Join(".local", directory))
	if err := os.MkdirAll(parent, 0755); err != nil {
		t.Fatal(err)
	}
	workspace, err := os.MkdirTemp(parent, "run-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", workspace)
	recipePath := filepath.Join(workspace, "recipes.json")
	if err := parity.WriteJSON(recipePath, recipes); err != nil {
		t.Fatal(err)
	}
	common := []string{"--spec", filepath.Join(root, "baseline.json"), "--recipes", recipePath, "--baseline", filepath.Join(root, ".local/baseline"), "--lock", filepath.Join(root, ".local/baseline.lock.json"), "--dotnet", dotnet}
	capture := filepath.Join(workspace, "capture")
	if err := run(t.Context(), append(append([]string{"capture"}, common...), "--out", capture)); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, ".local/gitversion-go")
	hash, err := parity.FileHash(exe)
	if err != nil {
		t.Fatal(err)
	}
	command := parity.Command{Executable: exe, Arguments: []string{"calculate", "--repository", "{repo}", "--trace"}}
	commandPath := filepath.Join(workspace, "candidate.json")
	if err := parity.WriteJSON(commandPath, command); err != nil {
		t.Fatal(err)
	}
	verify := filepath.Join(workspace, "go")
	runErr := run(t.Context(), append(append([]string{"verify"}, common...), "--expected", capture, "--candidate", commandPath, "--out", verify))
	var report Report
	if err := parity.ReadJSON(filepath.Join(verify, "report.json"), &report); err != nil {
		t.Fatal(err)
	}
	after, err := parity.FileHash(exe)
	if err != nil || hash != after {
		t.Fatal("candidate changed during acceptance")
	}
	recipeHash, err := parity.FileHash(recipePath)
	if err != nil {
		t.Fatal(err)
	}
	lockHash, err := parity.FileHash(filepath.Join(root, ".local/baseline.lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "provenance.json"), map[string]any{"candidate_sha256": hash, "recipes_sha256": recipeHash, "baseline_lock_sha256": lockHash, "expected": expected, "passed": report.Passed, "failed": report.Failed, "complete": runErr == nil && report.Passed == expected && report.Failed == 0}); err != nil {
		t.Fatal(err)
	}
	if runErr != nil || report.Passed != expected || report.Failed != 0 {
		for _, c := range report.Cases {
			if c.Status != "passed" {
				t.Logf("%s: %s %+v", c.ID, c.Error, c.Differences)
			}
		}
		t.Fatalf("policy parity: %d/%d passed: %v", report.Passed, expected, runErr)
	}
}

func TestWorkflowEdgeParity(t *testing.T) {
	recipes, err := fixtures.Load("../../testdata/workflow-edge-recipes.json")
	if err != nil {
		t.Fatal(err)
	}
	testPolicyParity(t, recipes, "workflow-edge-evidence", 13)
}

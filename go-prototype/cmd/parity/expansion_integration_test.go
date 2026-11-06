//go:build expansion

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

// Opt-in because this launches at least 2,000 processes. Never hides failures as
// unsupported cases. Every input, process result and mismatch stays on disk.
func TestGeneratedCalculationParity(t *testing.T) {
	testGeneratedParity(t, "original")
}

func TestGeneratedPolicyParity(t *testing.T) {
	testGeneratedParity(t, "policies")
}

func TestGeneratedWorkflowParity(t *testing.T) { testGeneratedParity(t, "workflows") }

func TestGeneratedResetParity(t *testing.T) {
	testGeneratedParity(t, "resets")
}
func testGeneratedParity(t *testing.T, suite string) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dotnet := os.Getenv("PARITY_DOTNET")
	var baseline parity.Baseline
	lock := filepath.Join(root, ".local/baseline.lock.json")
	if err := parity.ReadJSON(lock, &baseline); err != nil {
		t.Fatal(err)
	}
	baseDir := filepath.Join(root, ".local/baseline")
	if err := parity.VerifyBaseline(baseline, filepath.Join(root, "baseline.json"), dotnet, baseDir); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, ".local/gitversion-go")
	candidateHash, err := parity.FileHash(exe)
	if err != nil {
		t.Fatal(err)
	}
	count := 1000
	if value := os.Getenv("PARITY_GENERATED_COUNT"); value != "" {
		count, err = strconv.Atoi(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	recipes, err := fixtures.Generated(20261106, count)
	if suite == "policies" {
		recipes, err = fixtures.GeneratedPolicies(20261106, count)
	}
	if suite == "resets" {
		recipes, err = fixtures.GeneratedResets(20261106, count)
	}
	if suite == "workflows" {
		recipes, err = fixtures.GeneratedWorkflows(20261106, count)
	}
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(root, ".local/expansion-evidence")
	if suite == "policies" {
		parent = filepath.Join(root, ".local/expanded-generated-evidence")
	}
	if suite == "resets" {
		parent = filepath.Join(root, ".local/reset-generated-evidence")
	}
	if suite == "workflows" {
		parent = filepath.Join(root, ".local/workflow-generated-evidence")
	}
	if err := os.MkdirAll(parent, 0755); err != nil {
		t.Fatal(err)
	}
	workspace, err := os.MkdirTemp(parent, "run-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", workspace)
	if err := parity.WriteJSON(filepath.Join(workspace, "recipes.json"), recipes); err != nil {
		t.Fatal(err)
	}
	checkpoints, err := fixtures.Generate(recipes, filepath.Join(workspace, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != count {
		t.Fatal("checkpoint inventory changed")
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "checkpoints.json"), checkpoints); err != nil {
		t.Fatal(err)
	}
	failure := workflowFailure{}
	if suite == "workflows" {
		failure = loadWorkflowFailure(t, root)
	}
	commands := []parity.Command{
		{Executable: dotnet, Arguments: append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, baseline.Spec.Arguments...)},
		{Executable: exe, Arguments: []string{"calculate", "--repository", "{repo}", "--trace"}},
	}
	report := Report{Mode: "generated", Calculator: "external-candidate", Cases: make([]CaseResult, len(checkpoints))}
	jobs := make(chan int)
	var workers sync.WaitGroup
	for w := 0; w < 4; w++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range jobs {
				cp := checkpoints[i]
				item := CaseResult{ID: cp.ID, Status: "failed"}
				outputs := make([][]byte, 2)
				for j, command := range commands {
					path := filepath.Join(workspace, fmt.Sprintf("%s-%d", cp.ID, j))
					result, err := parity.Run(t.Context(), command, cp.Path, path, baseline.Spec.Environment, 30*time.Second)
					if err == nil {
						err = saveResult(path, result)
					}
					if err == nil {
						if cp.ID == failure.ID {
							err = failure.validate(cp, result)
							item.ExpectedRejection = true
						} else {
							err = result.Validate()
						}
					}
					if err != nil {
						item.Error = fmt.Sprintf("calculator %d: %v", j, err)
						break
					}
					outputs[j] = result.Stdout
				}
				if item.Error == "" && item.ExpectedRejection {
					item.Status = "passed"
				}
				if item.Error == "" && !item.ExpectedRejection {
					differences, compareErr := parity.Compare(outputs[0], outputs[1])
					item.Differences = differences
					if compareErr != nil {
						item.Error = compareErr.Error()
					}
					if compareErr == nil && len(item.Differences) == 0 {
						item.Status = "passed"
					}
				}
				report.Cases[i] = item
			}
		}()
	}
	for i := range checkpoints {
		jobs <- i
	}
	close(jobs)
	workers.Wait()
	for _, c := range report.Cases {
		if c.Status == "passed" {
			report.Passed++
		} else {
			report.Failed++
			t.Logf("%s: %s %+v", c.ID, c.Error, c.Differences)
		}
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "report.json"), report); err != nil {
		t.Fatal(err)
	}
	lockHash, err := parity.FileHash(lock)
	if err != nil {
		t.Fatal(err)
	}
	recipesHash, err := parity.FileHash(filepath.Join(workspace, "recipes.json"))
	if err != nil {
		t.Fatal(err)
	}
	after, err := parity.FileHash(exe)
	if err != nil || after != candidateHash {
		t.Fatal("candidate changed during experiment")
	}
	if err := parity.VerifyBaseline(baseline, filepath.Join(root, "baseline.json"), dotnet, baseDir); err != nil {
		t.Fatal(err)
	}
	provenance := map[string]any{"seed": 20261106, "expected": count, "passed": report.Passed, "failed": report.Failed, "complete": report.Passed == count, "platform": parity.Platform(), "candidate_sha256": candidateHash, "baseline_lock_sha256": lockHash, "recipes_sha256": recipesHash}
	if err := parity.WriteJSON(filepath.Join(workspace, "provenance.json"), provenance); err != nil {
		t.Fatal(err)
	}
	if report.Failed != 0 {
		t.Fatalf("%d/%d generated checkpoints differ; retained %s", report.Failed, count, workspace)
	}
}

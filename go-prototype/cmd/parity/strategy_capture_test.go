//go:build strategies

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

// Explicit capture mode only writes a new ignored evidence directory. Promotion
// to a frozen reference is a separate, reviewed operation; tests never refresh it.
func TestCaptureStrategyReference(t *testing.T) {
	captureConfigurationReference(t, "strategy", fixtures.StrategyRecipes())
}
func TestCaptureConfigurationEdgeReference(t *testing.T) {
	captureConfigurationReference(t, "configuration-edge", fixtures.ConfigurationEdgeRecipes())
}
func TestCaptureConfigurationReference(t *testing.T) {
	captureConfigurationReference(t, "configuration", fixtures.ConfigurationRecipes())
}
func captureConfigurationReference(t *testing.T, suite string, recipes []fixtures.Recipe) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	dotnet := os.Getenv("PARITY_DOTNET")
	baseDir := filepath.Join(root, ".local/baseline")
	if err := parity.VerifyBaseline(baseline, filepath.Join(root, "baseline.json"), dotnet, baseDir); err != nil {
		t.Fatal(err)
	}
	workspace, err := os.MkdirTemp(filepath.Join(root, ".local"), suite+"-capture-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", workspace)
	if err := parity.WriteJSON(filepath.Join(workspace, "recipes.json"), recipes); err != nil {
		t.Fatal(err)
	}
	cps, err := fixtures.Generate(recipes, filepath.Join(workspace, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	command := parity.Command{Executable: dotnet, Arguments: append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, baseline.Spec.Arguments...)}
	results := make([]parity.Result, len(cps))
	errors := make([]error, len(cps))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				p := filepath.Join(workspace, cps[i].ID)
				results[i], errors[i] = parity.Run(t.Context(), command, cps[i].Path, p, baseline.Spec.Environment, 30*time.Second)
				if errors[i] == nil {
					errors[i] = saveResult(p, results[i])
				}
			}
		}()
	}
	for i := range cps {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	recipeHash, err := parity.FileHash(filepath.Join(workspace, "recipes.json"))
	if err != nil {
		t.Fatal(err)
	}
	lockHash, err := parity.FileHash(filepath.Join(root, ".local/baseline.lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	golden := Golden{Schema: 1, RecipesSHA256: recipeHash, CaptureBaselineSHA256: lockHash, SpecSHA256: baseline.SpecSHA256, Cases: cps, Outputs: map[string]json.RawMessage{}}
	failures := map[string]parity.Result{}
	for i, r := range results {
		if errors[i] != nil {
			t.Error(cps[i].ID, errors[i])
			continue
		}
		if r.Validate() != nil {
			failures[cps[i].ID] = r
		} else {
			golden.Outputs[cps[i].ID] = r.Stdout
		}
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "golden.json"), golden); err != nil {
		t.Fatal(err)
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "failures.json"), failures); err != nil {
		t.Fatal(err)
	}
	t.Logf("captured %d JSON outputs and %d process rejections", len(golden.Outputs), len(failures))
}

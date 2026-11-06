//go:build integration

package gitrepo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/go-git/go-git/v5/plumbing"
)

type observationCase struct {
	ID      string `json:"id"`
	Digest  string `json:"digest"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
	Queries int    `json:"queries"`
}
type observationReport struct {
	Schema             int               `json:"schema"`
	Complete           bool              `json:"complete"`
	Expected           int               `json:"expected"`
	BaselineLockSHA256 string            `json:"baseline_lock_sha256"`
	RecipeSHA256       string            `json:"recipe_sha256"`
	ProbeFiles         map[string]string `json:"probe_files"`
	ProbeSources       map[string]string `json:"probe_sources"`
	CandidateSHA256    string            `json:"candidate_sha256"`
	Passed             int               `json:"passed"`
	Failed             int               `json:"failed"`
	Cases              []observationCase `json:"cases"`
}

// This is an explicit acceptance gate. Missing oracle/toolchain prerequisites
// fail the test, rather than skipping a comparison and reporting success.
func TestPinnedRepositoryObservations(t *testing.T) {
	dotnet := os.Getenv("PARITY_DOTNET")
	if dotnet == "" {
		t.Fatal("set PARITY_DOTNET and run scripts/build-observer.sh first")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	baseline := filepath.Join(root, ".local", "baseline")
	lockPath := filepath.Join(root, ".local", "baseline.lock.json")
	var lock parity.Baseline
	if err = parity.ReadJSON(lockPath, &lock); err != nil {
		t.Fatal(err)
	}
	if err = parity.VerifyBaseline(lock, filepath.Join(root, "baseline.json"), dotnet, baseline); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(root, ".local", "observer")
	candidate := filepath.Join(root, ".local", "gitversion-go")
	report := observationReport{Schema: 1, Expected: 47, ProbeSources: map[string]string{}, Cases: []observationCase{}}
	report.ProbeFiles, err = parity.FileSet(probe)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Program.cs", "RepositoryObserver.csproj"} {
		h, err := parity.FileHash(filepath.Join(root, "oracle", "RepositoryObserver", name))
		if err != nil {
			t.Fatal(err)
		}
		report.ProbeSources[name] = h
	}
	// No copied dependency may shadow the verified historical assemblies loaded
	// by the reflection probe. The observer has no package references of its own.
	for name := range report.ProbeFiles {
		switch name {
		case "RepositoryObserver", "RepositoryObserver.exe", "RepositoryObserver.dll", "RepositoryObserver.pdb", "RepositoryObserver.deps.json", "RepositoryObserver.runtimeconfig.json":
		default:
			t.Fatalf("unexpected probe dependency: %s", name)
		}
	}
	report.CandidateSHA256, err = parity.FileHash(candidate)
	if err != nil {
		t.Fatal(err)
	}
	report.BaselineLockSHA256, err = parity.FileHash(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	recipePath := filepath.Join(root, "testdata", "recipes.json")
	report.RecipeSHA256, err = parity.FileHash(recipePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, ".local", "observation-evidence")
	if err = os.MkdirAll(evidence, 0755); err != nil {
		t.Fatal(err)
	}
	out, err := os.MkdirTemp(evidence, "run-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", out)
	defer func() {
		if err := parity.WriteJSON(filepath.Join(out, "report.json"), report); err != nil {
			t.Error(err)
		}
	}()
	recipes, err := fixtures.Load(recipePath)
	if err != nil {
		t.Fatal(err)
	}
	cps, err := fixtures.Generate(recipes, filepath.Join(out, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cps) != 41 {
		t.Fatalf("initial corpus changed: %d checkpoints", len(cps))
	}
	adversarial := map[string]graphFixture{"equal-dates": diamond(t, true), "criss-cross": crissCross(t)}
	skew := []graphCommit{{"root", nil, 500, nil}}
	for i := 1; i <= 12; i++ {
		name := string(rune('a' + i - 1))
		parent := "root"
		if i > 1 {
			parent = string(rune('a' + i - 2))
		}
		skew = append(skew, graphCommit{name, []string{parent}, int64(i % 3), nil})
	}
	adversarial["skewed-dates"] = makeGraph(t, skew, map[string]string{"main": "l", "exclude": "f"}, "main")
	nested := diamond(t, false)
	addNestedTags(t, nested)
	if err = nested.git.Storer.PackRefs(); err != nil {
		t.Fatal(err)
	}
	if err = nested.git.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("zzz"), plumbing.NewHash(nested.ids["merge"]))); err != nil {
		t.Fatal(err)
	}
	adversarial["packed-nested-tags"] = nested
	adversarial["tied-criss-cross"] = tiedCrissCross(t)
	adversarial["unicode-ref-order"] = unicodeRefs(t)
	// Keep case order stable without relying on Go map iteration.
	for _, name := range []string{"equal-dates", "criss-cross", "skewed-dates", "packed-nested-tags", "tied-criss-cross", "unicode-ref-order"} {
		f := adversarial[name]
		digest, err := fixtures.Digest(f.path)
		if err != nil {
			t.Fatal(err)
		}
		cps = append(cps, fixtures.Checkpoint{ID: "adapter--" + name, Path: f.path, Digest: digest})
	}
	expectedCommand := parity.Command{Executable: dotnet, Arguments: []string{"--fx-version", lock.Spec.Runtime, filepath.Join(probe, "RepositoryObserver.dll"), baseline, "{repo}"}}
	candidateCommand := parity.Command{Executable: candidate, Arguments: []string{"observe", "--repository", "{repo}"}}
	for _, cp := range cps {
		t.Run(cp.ID, func(t *testing.T) {
			entry := observationCase{ID: cp.ID, Digest: cp.Digest, Status: "failed"}
			defer func() {
				if entry.Status == "passed" {
					report.Passed++
				} else {
					report.Failed++
				}
				report.Cases = append(report.Cases, entry)
			}()
			fail := func(err error) { entry.Error = err.Error(); t.Fatal(err) }
			dir := filepath.Join(out, cp.ID)
			if err = os.Mkdir(dir, 0755); err != nil {
				fail(err)
			}
			reference, err := parity.Run(t.Context(), expectedCommand, cp.Path, filepath.Join(dir, "csharp"), lock.Spec.Environment, 30*time.Second)
			if err != nil {
				fail(err)
			}
			if err = saveObservation(dir, "csharp", reference); err != nil {
				fail(err)
			}
			if err = reference.Validate(); err != nil {
				t.Log(string(reference.Stderr))
				fail(err)
			}
			actual, err := parity.Run(t.Context(), candidateCommand, cp.Path, filepath.Join(dir, "go"), nil, 30*time.Second)
			if err != nil {
				fail(err)
			}
			if err = saveObservation(dir, "go", actual); err != nil {
				fail(err)
			}
			if err = actual.Validate(); err != nil {
				t.Log(string(actual.Stderr))
				fail(err)
			}
			differences, err := parity.Compare(reference.Stdout, actual.Stdout)
			if err != nil {
				fail(err)
			}
			if err = parity.WriteJSON(filepath.Join(dir, "differences.json"), differences); err != nil {
				fail(err)
			}
			if len(differences) > 0 {
				entry.Error = "repository observations differ; see differences.json"
				t.Fatal(entry.Error)
			}
			var observed Observation
			if err = json.Unmarshal(actual.Stdout, &observed); err != nil {
				fail(err)
			}
			entry.Queries = len(observed.Walks) + len(observed.MergeBases)
			entry.Status = "passed"
		})
	}
	// Reject vacuous success or a truncated inventory, even if subtests change.
	report.Complete = report.Passed == report.Expected && report.Failed == 0 && len(report.Cases) == report.Expected
	if !report.Complete {
		t.Errorf("repository parity: %d passed, %d failed; expected %d", report.Passed, report.Failed, report.Expected)
	}
}
func saveObservation(dir, name string, result parity.Result) error {
	if err := parity.WriteJSON(filepath.Join(dir, name+"-process.json"), result); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), result.Stdout, 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".stderr"), result.Stderr, 0644)
}

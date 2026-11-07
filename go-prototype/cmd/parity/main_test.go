package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestGoldenCorpusMatchesRecipes(t *testing.T) {
	var golden Golden
	if err := parity.ReadJSON("../../testdata/reference.json", &golden); err != nil {
		t.Fatal(err)
	}
	recipeHash, err := parity.FileHash("../../testdata/recipes.json")
	if err != nil {
		t.Fatal(err)
	}
	specHash, err := parity.FileHash("../../baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	if golden.Schema != 1 || golden.SpecSHA256 != specHash || golden.RecipesSHA256 != recipeHash {
		t.Fatal("golden provenance changed")
	}
	recipes, err := fixtures.Load("../../testdata/recipes.json")
	if err != nil {
		t.Fatal(err)
	}
	cps, err := fixtures.Generate(recipes, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if err = matchCases(golden.Cases, cps); err != nil {
		t.Fatal(err)
	}
	if len(cps) != 41 || len(golden.Outputs) != 41 {
		t.Fatalf("unexpected inventory: %d checkpoints / %d outputs", len(cps), len(golden.Outputs))
	}
	for _, cp := range cps {
		t.Run(cp.ID, func(t *testing.T) {
			obj, err := parity.Object(golden.Outputs[cp.ID])
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"FullSemVer", "SemVer", "SemVerSourceSemVer", "SemVerSourceSha", "SemVerSourceIncrement", "CommitCountSourceSha", "CommitCountSourceDistance", "Sha", "CommitDate", "UncommittedChanges"} {
				if _, ok := obj[key]; !ok {
					t.Fatalf("missing reference field %s", key)
				}
			}
			if obj["Sha"] != cp.Head {
				t.Fatalf("reference SHA %v differs from fixture %s", obj["Sha"], cp.Head)
			}
			if obj["UncommittedChanges"] != json.Number("0") {
				t.Fatalf("clean fixture reference has dirty count %v", obj["UncommittedChanges"])
			}
			if cp.SemVer != "" && obj["SemVer"] != cp.SemVer {
				t.Fatalf("independent version assertion: %v / %s", obj["SemVer"], cp.SemVer)
			}
		})
	}
}

func TestCLIRejectsInvalidCommandsAndJSONDifferences(t *testing.T) {
	if err := run(context.Background(), nil); err == nil {
		t.Fatal("missing command accepted")
	}
	if err := run(context.Background(), []string{"unknown"}); err == nil {
		t.Fatal("unknown command accepted")
	}
	dir := t.TempDir()
	expected := filepath.Join(dir, "expected.json")
	actual := filepath.Join(dir, "actual.json")
	if err := os.WriteFile(expected, []byte(`{"Major":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(actual, []byte(`{"Major":"1"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"compare", "--expected", expected, "--actual", actual}); err == nil {
		t.Fatal("CLI accepted typed difference")
	}
	if err := os.WriteFile(actual, []byte(`{ "Major" : 1 }`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"compare", "--expected", expected, "--actual", actual}); err != nil {
		t.Fatal(err)
	}
}

func TestWriteNewJSONPreservesReviewedData(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lock.json")
	if err := writeNewJSON(p, map[string]int{"version": 1}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = writeNewJSON(p, map[string]int{"version": 2}); err == nil {
		t.Fatal("overwrote existing reference")
	}
	after, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("reviewed reference changed")
	}
}

func TestMatchCasesRejectsMissingReorderedAndChangedIdentities(t *testing.T) {
	base := []fixtures.Checkpoint{{ID: "a", Head: "aaa", Digest: "one"}, {ID: "b", Head: "bbb", Digest: "two"}}
	for _, tt := range []struct {
		name  string
		cases []fixtures.Checkpoint
	}{{"missing", base[:1]}, {"reordered", []fixtures.Checkpoint{base[1], base[0]}}, {"changed", []fixtures.Checkpoint{{ID: "a", Head: "changed", Digest: "one"}, base[1]}}} {
		t.Run(tt.name, func(t *testing.T) {
			if err := matchCases(base, tt.cases); err == nil {
				t.Fatal("accepted changed inventory")
			}
		})
	}
	actual := append([]fixtures.Checkpoint(nil), base...)
	actual[0].Path = "/another/location"
	if err := matchCases(base, actual); err != nil {
		t.Fatal("path dependence", err)
	}
}

func TestEffectiveConfigurationRejectsMutation(t *testing.T) {
	r := parity.Result{Stdout: []byte("workflow: GitFlow/v1\n"), Before: "state", After: "state"}
	if !effectiveConfigurationSucceeded(r) {
		t.Fatal("valid unchanged configuration output rejected")
	}
	r.After = "changed"
	if effectiveConfigurationSucceeded(r) {
		t.Fatal("effective configuration accepted after repository mutation")
	}
	r.Before = ""
	r.After = ""
	if effectiveConfigurationSucceeded(r) {
		t.Fatal("effective configuration accepted without state evidence")
	}
}

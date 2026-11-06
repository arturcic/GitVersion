package calculate

import (
	"encoding/json"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"path/filepath"
	"testing"
)

func TestExpandedPolicyFrozenReference(t *testing.T) {
	recipes, err := fixtures.PolicyRecipes()
	if err != nil {
		t.Fatal(err)
	}
	testFrozenPolicies(t, "policy-reference.json", recipes, 250)
}

func TestResetPolicyFrozenReference(t *testing.T) {
	testFrozenPolicies(t, "reset-reference.json", fixtures.ResetRecipes(), 192)
}

func testFrozenPolicies(t *testing.T, name string, recipes []fixtures.Recipe, expected int) {
	var golden struct {
		SpecSHA256 string                     `json:"spec_sha256"`
		Cases      []fixtures.Checkpoint      `json:"cases"`
		Outputs    map[string]json.RawMessage `json:"outputs"`
	}
	if err := parity.ReadJSON(filepath.Join("../../testdata", name), &golden); err != nil {
		t.Fatal(err)
	}
	specHash, err := parity.FileHash("../../baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	if golden.SpecSHA256 != specHash || len(golden.Cases) != expected || len(golden.Outputs) != expected {
		t.Fatal("expanded reference provenance or inventory changed")
	}
	cps, err := fixtures.Generate(recipes, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cps) != expected {
		t.Fatal("fixture inventory changed")
	}
	for i, cp := range cps {
		t.Run(cp.ID, func(t *testing.T) {
			if cp.ID != golden.Cases[i].ID || cp.Head != golden.Cases[i].Head || cp.Digest != golden.Cases[i].Digest {
				t.Fatal("fixture differs from independently captured reference")
			}
			before, err := fixtures.State(cp.Path)
			if err != nil {
				t.Fatal(err)
			}
			got, trace, err := Calculate(cp.Path)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			diff, err := parity.Compare(golden.Outputs[cp.ID], actual)
			if err != nil || len(diff) > 0 {
				t.Fatalf("policy mismatch: %+v %v trace=%+v", diff, err, trace)
			}
			after, err := fixtures.State(cp.Path)
			if err != nil || before != after {
				t.Fatal("calculation mutated input")
			}
		})
	}
}

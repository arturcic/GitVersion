package calculate

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestMinimizedGeneratedRegressions(t *testing.T) {
	var reference struct {
		Source  string                     `json:"source_revision"`
		Recipes []fixtures.Recipe          `json:"recipes"`
		Outputs map[string]json.RawMessage `json:"outputs"`
	}
	if err := parity.ReadJSON("../../testdata/generated-regressions.json", &reference); err != nil {
		t.Fatal(err)
	}
	if reference.Source != "14d222235b7dabb7033473528a76fd18478a9217" || len(reference.Recipes) != 35 || len(reference.Outputs) != 35 {
		t.Fatal("regression provenance or inventory changed")
	}
	cps, err := fixtures.Generate(reference.Recipes, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	for _, cp := range cps {
		t.Run(cp.ID, func(t *testing.T) {
			before, err := fixtures.State(cp.Path)
			if err != nil {
				t.Fatal(err)
			}
			values, _, err := Calculate(cp.Path)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := json.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			diff, err := parity.Compare(reference.Outputs[cp.ID], actual)
			if err != nil || len(diff) != 0 {
				t.Fatalf("regression differs: %+v %v", diff, err)
			}
			after, err := fixtures.State(cp.Path)
			if err != nil || before != after {
				t.Fatal("calculator mutated fixture")
			}
		})
	}
}

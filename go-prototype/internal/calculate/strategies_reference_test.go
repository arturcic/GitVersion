package calculate

import (
	"encoding/json"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigurationEdgeFrozenReference(t *testing.T) {
	testConfigurationReference(t, "configuration-edge", fixtures.ConfigurationEdgeRecipes(), 13, 1)
}
func TestConfigurationFrozenReference(t *testing.T) {
	testConfigurationReference(t, "configuration", fixtures.ConfigurationRecipes(), 800, 0)
}
func TestStrategyFrozenReference(t *testing.T) {
	testConfigurationReference(t, "strategy", fixtures.StrategyRecipes(), 688, 80)
}
func testConfigurationReference(t *testing.T, suite string, recipes []fixtures.Recipe, successes, rejections int) {
	var golden struct {
		SpecSHA256 string                     `json:"spec_sha256"`
		Cases      []fixtures.Checkpoint      `json:"cases"`
		Outputs    map[string]json.RawMessage `json:"outputs"`
		Rejections map[string]string          `json:"rejections"`
	}
	if err := parity.ReadJSON("../../testdata/"+suite+"-reference.json", &golden); err != nil {
		t.Fatal(err)
	}
	spec, err := parity.FileHash("../../baseline.json")
	if err != nil || spec != golden.SpecSHA256 || len(golden.Outputs) != successes || len(golden.Rejections) != rejections {
		t.Fatal("reference provenance or inventory differs", err)
	}
	cps, err := fixtures.Generate(recipes, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cps) != successes+rejections || len(golden.Cases) != len(cps) {
		t.Fatal("fixture inventory differs")
	}
	for i, cp := range cps {
		t.Run(cp.ID, func(t *testing.T) {
			pin := golden.Cases[i]
			if cp.ID != pin.ID || cp.Head != pin.Head || cp.Digest != pin.Digest {
				t.Fatal("fixture differs from frozen reference")
			}
			before, err := fixtures.State(cp.Path)
			if err != nil {
				t.Fatal(err)
			}
			got, trace, err := Calculate(cp.Path)
			if digest, ok := golden.Rejections[cp.ID]; ok {
				if digest != cp.Digest || err == nil || !strings.Contains(err.Error(), "no base versions determined") {
					t.Fatalf("expected no-candidate rejection, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if cp.SemVer != "" && got["FullSemVer"] != cp.SemVer {
					t.Fatalf("original C# assertion: want %s, got %v", cp.SemVer, got["FullSemVer"])
				}
				actual, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				diffs, err := parity.Compare(golden.Outputs[cp.ID], actual)
				if err != nil || len(diffs) != 0 {
					t.Fatalf("configuration mismatch: %+v %v trace=%+v", diffs, err, trace)
				}
			}
			after, err := fixtures.State(cp.Path)
			if err != nil || after != before {
				t.Fatal("calculation mutated fixture")
			}
		})
	}
}

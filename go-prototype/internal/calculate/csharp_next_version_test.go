package calculate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
)

func TestCSharpNextVersionCalculator(t *testing.T) {
	var corpus struct {
		Revision string
		Methods  map[string][]struct {
			ID         string
			RecipeID   string `json:"recipe_id"`
			Assertions []string
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-next-version.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Methods) != 11 {
		t.Fatal("original source inventory differs")
	}
	recipes, err := fixtures.Load("../../testdata/csharp-next-version-recipes.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(recipes) != 11 {
		t.Fatal("original fixture inventory differs")
	}
	byID := map[string]fixtures.Recipe{}
	for _, recipe := range recipes {
		byID[recipe.ID] = recipe
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			if len(cases) != 1 {
				t.Fatal("original case inventory differs")
			}
			for _, tc := range cases {
				t.Run(tc.ID, func(t *testing.T) {
					recipe, exists := byID[tc.RecipeID]
					if !exists || recipe.Description != method {
						t.Fatal("original history missing")
					}
					cps, err := fixtures.Generate([]fixtures.Recipe{recipe}, filepath.Join(t.TempDir(), "history"))
					if err != nil {
						t.Fatal(err)
					}
					if len(cps) != len(tc.Assertions) {
						t.Fatal("original assertion checkpoint missing")
					}
					for i, cp := range cps {
						t.Run(cp.ID, func(t *testing.T) {
							before, err := fixtures.Digest(cp.Path)
							if err != nil {
								t.Fatal(err)
							}
							got, _, err := Calculate(cp.Path)
							if err != nil {
								t.Fatal(err)
							}
							if cp.SemVer != tc.Assertions[i] || got["FullSemVer"] != tc.Assertions[i] {
								t.Fatalf("original assertion: want %s, got %v", tc.Assertions[i], got["FullSemVer"])
							}
							after, err := fixtures.Digest(cp.Path)
							if err != nil || before != after {
								t.Fatal("calculation mutated fixture", err)
							}
						})
					}
				})
			}
		})
	}
}

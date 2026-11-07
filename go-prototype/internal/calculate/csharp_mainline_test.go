package calculate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
)

func TestCSharpMainlineScenarios(t *testing.T) {
	var index struct {
		Revision       string
		Methods, Cases int
		Scenarios      []struct {
			File, Source   string
			Methods, Cases int
		}
	}
	read := func(path string, target any) {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, target); err != nil {
			t.Fatal(err)
		}
	}
	read("../../testdata/csharp-mainline-index.json", &index)
	if index.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(index.Scenarios) != 79 || index.Methods != 380 || index.Cases != 32064 {
		t.Fatal("source inventory changed")
	}
	slots := make(chan struct{}, 4)
	for _, spec := range index.Scenarios {
		var corpus struct {
			Revision, Source string
			Recipe           fixtures.Recipe
			Methods          map[string][]struct {
				ID, Config, Expected string
				NullLabels           []string `json:"null_labels"`
				Arguments            []any
			}
		}
		read(filepath.Join("../..", spec.File), &corpus)
		if corpus.Source != spec.Source || corpus.Revision != index.Revision || len(corpus.Methods) != spec.Methods {
			t.Fatal("source projection differs")
		}
		count := 0
		names := []string{}
		for name, cases := range corpus.Methods {
			count += len(cases)
			names = append(names, name)
		}
		if count != spec.Cases {
			t.Fatal("original case inventory differs")
		}
		sort.Strings(names)
		t.Run(corpus.Recipe.Description, func(t *testing.T) {
			t.Parallel()
			slots <- struct{}{}
			defer func() { <-slots }()
			cps, err := fixtures.Generate([]fixtures.Recipe{corpus.Recipe}, filepath.Join(t.TempDir(), "history"))
			if err != nil {
				t.Fatal(err)
			}
			if len(cps) != 1 {
				t.Fatal("expected one original shared history")
			}
			before, err := fixtures.Digest(cps[0].Path)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range names {
				t.Run(name, func(t *testing.T) {
					for _, tc := range corpus.Methods[name] {
						t.Run(tc.ID, func(t *testing.T) {
							cfg, err := config.Parse([]byte(tc.Config))
							if err != nil {
								t.Fatal(err)
							}
							for _, kind := range tc.NullLabels {
								if kind == "main" {
									cfg.Main.NullLabel = true
								} else if kind == "feature" {
									cfg.Feature.NullLabel = true
								} else {
									t.Fatal("unexpected null-label branch", kind)
								}
							}
							got, _, err := CalculateWithConfig(cps[0].Path, cfg)
							if err != nil {
								t.Fatal(err)
							}
							if got["FullSemVer"] != tc.Expected {
								t.Fatalf("original arguments %v: want %s, got %v", tc.Arguments, tc.Expected, got["FullSemVer"])
							}
						})
					}
				})
			}
			after, err := fixtures.Digest(cps[0].Path)
			if err != nil || before != after {
				t.Fatal("calculation mutated shared fixture", err)
			}
		})
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

// Golden is a portable, reviewable output set. Machine-specific build attestations
// remain beside the original capture, linked by the baseline lock digest.
type Golden struct {
	Rejections            map[string]string          `json:"rejections,omitempty"`
	Schema                int                        `json:"schema"`
	SpecSHA256            string                     `json:"spec_sha256"`
	RecipesSHA256         string                     `json:"recipes_sha256"`
	CaptureBaselineSHA256 string                     `json:"capture_baseline_sha256"`
	Cases                 []fixtures.Checkpoint      `json:"cases"`
	Outputs               map[string]json.RawMessage `json:"outputs"`
}

func freeze(capture, specPath, recipesPath, out string) error {
	var corpus Corpus
	if err := parity.ReadJSON(filepath.Join(capture, "corpus.json"), &corpus); err != nil {
		return err
	}
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(capture, "baseline.lock.json"), &baseline); err != nil {
		return err
	}
	lockHash, err := parity.FileHash(filepath.Join(capture, "baseline.lock.json"))
	if err != nil {
		return err
	}
	specHash, err := parity.FileHash(specPath)
	if err != nil {
		return err
	}
	recipeHash, err := parity.FileHash(recipesPath)
	if err != nil {
		return err
	}
	if corpus.Schema != 1 || baseline.Schema != 1 || len(corpus.Cases) == 0 || corpus.BaselineSHA256 != lockHash || baseline.SpecSHA256 != specHash || corpus.RecipesSHA256 != recipeHash {
		return fmt.Errorf("capture provenance differs")
	}
	recipes, err := fixtures.Load(recipesPath)
	if err != nil {
		return err
	}
	scratch, err := os.MkdirTemp("", "gitversion-freeze-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	cps, err := fixtures.Generate(recipes, filepath.Join(scratch, "fixtures"))
	if err != nil {
		return err
	}
	if err = matchCases(corpus.Cases, cps); err != nil {
		return err
	}
	golden := Golden{Schema: 1, SpecSHA256: specHash, RecipesSHA256: recipeHash, CaptureBaselineSHA256: lockHash, Cases: corpus.Cases, Outputs: map[string]json.RawMessage{}}
	if len(corpus.Outputs) != len(cps) {
		return fmt.Errorf("incomplete capture output inventory")
	}
	for _, cp := range cps {
		data, err := os.ReadFile(filepath.Join(capture, cp.ID, "stdout.json"))
		if err != nil {
			return err
		}
		if parity.Hash(data) != corpus.Outputs[cp.ID] {
			return fmt.Errorf("capture output changed for %s", cp.ID)
		}
		if _, err = parity.Object(data); err != nil {
			return err
		}
		golden.Outputs[cp.ID] = data
	}
	return writeNewJSON(out, golden)
}

func matchCases(expected, actual []fixtures.Checkpoint) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("reference checkpoint inventory differs")
	}
	for i, a := range actual {
		e := expected[i]
		e.Path = ""
		a.Path = ""
		if a != e {
			return fmt.Errorf("reference checkpoint identity differs for %s", a.ID)
		}
	}
	return nil
}

func writeNewJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(append(data, '\n'))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

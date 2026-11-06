//go:build integration

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

// Run explicitly after scripts/build-baseline.sh. Missing prerequisites fail,
// rather than turning the acceptance run into a green suite of skipped tests.
func TestPinnedReferenceCaptureReplayAndSeededMismatch(t *testing.T) {
	dotnet := os.Getenv("PARITY_DOTNET")
	if dotnet == "" {
		t.Fatal("set PARITY_DOTNET to the absolute dotnet host after building the pinned baseline")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local", "integration-evidence")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	workspace, err := os.MkdirTemp(out, "run-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", workspace)
	common := []string{"--spec", filepath.Join(root, "baseline.json"), "--recipes", filepath.Join(root, "testdata", "recipes.json"), "--baseline", filepath.Join(root, ".local", "baseline"), "--lock", filepath.Join(root, ".local", "baseline.lock.json"), "--dotnet", dotnet}
	capture := filepath.Join(workspace, "capture")
	replay := filepath.Join(workspace, "replay")
	args := append([]string{"capture"}, common...)
	args = append(args, "--out", capture)
	if err = run(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	args = append([]string{"verify"}, common...)
	args = append(args, "--expected", capture, "--out", replay)
	if err = run(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	var report Report
	if err = parity.ReadJSON(filepath.Join(replay, "report.json"), &report); err != nil {
		t.Fatal(err)
	}
	if report.Passed != 41 || report.Failed != 0 || len(report.Cases) != 41 || report.Calculator != "csharp-reference-replay" {
		t.Fatalf("incomplete replay %+v", report)
	}
	// Compare every newly captured C# field with the checked-in portable reference.
	var golden Golden
	if err = parity.ReadJSON(filepath.Join(root, "testdata", "reference.json"), &golden); err != nil {
		t.Fatal(err)
	}
	for _, c := range report.Cases {
		data, err := os.ReadFile(filepath.Join(capture, c.ID, "stdout.json"))
		if err != nil {
			t.Fatal(err)
		}
		diff, err := parity.Compare(golden.Outputs[c.ID], data)
		if err != nil || len(diff) > 0 {
			t.Fatalf("golden %s differs: %+v %v", c.ID, diff, err)
		}
	}
	portable := filepath.Join(workspace, "portable-reference.json")
	if err = freeze(capture, filepath.Join(root, "baseline.json"), filepath.Join(root, "testdata", "recipes.json"), portable); err != nil {
		t.Fatal(err)
	}
	var exported Golden
	if err = parity.ReadJSON(portable, &exported); err != nil {
		t.Fatal(err)
	}
	if exported.SpecSHA256 != golden.SpecSHA256 || exported.RecipesSHA256 != golden.RecipesSHA256 || len(exported.Outputs) != 41 {
		t.Fatal("portable export lost provenance or output inventory")
	}
	// A syntactically valid but deliberately wrong candidate must fail the real
	// execution/comparison path. It must not be classified as unsupported/skipped.
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(workspace, "candidate.json")
	if err = parity.WriteJSON(candidate, parity.Command{Executable: exe, Arguments: []string{"-test.run=^TestWrongCandidateHelper$", "--", "{repo}"}}); err != nil {
		t.Fatal(err)
	}
	mismatch := filepath.Join(workspace, "seeded-mismatch")
	args = append([]string{"verify"}, common...)
	args = append(args, "--expected", capture, "--out", mismatch, "--candidate", candidate)
	if err = run(context.Background(), args); err == nil {
		t.Fatal("wrong candidate passed parity")
	}
	if err = parity.ReadJSON(filepath.Join(mismatch, "report.json"), &report); err != nil {
		t.Fatal(err)
	}
	if report.Failed != 41 || report.Passed != 0 || report.Calculator != "external-candidate" {
		t.Fatalf("seeded mismatch accounting %+v", report)
	}
	for _, c := range report.Cases {
		if c.Status != "failed" || len(c.Differences) == 0 {
			t.Fatalf("missing typed mismatch evidence %+v", c)
		}
	}
	// Output tampering must fail even if the candidate itself is correct.
	first := report.Cases[0].ID
	if err = os.WriteFile(filepath.Join(capture, first, "stdout.json"), []byte(`{"SemVer":"tampered"}`), 0644); err != nil {
		t.Fatal(err)
	}
	tampered := filepath.Join(workspace, "tampered")
	args = append([]string{"verify"}, common...)
	args = append(args, "--expected", capture, "--out", tampered)
	if err = run(context.Background(), args); err == nil {
		t.Fatal("tampered reference accepted")
	}
	if err = parity.ReadJSON(filepath.Join(tampered, "report.json"), &report); err != nil {
		t.Fatal(err)
	}
	if report.Failed != 1 || report.Passed != 40 || !strings.Contains(report.Cases[0].Error, "checksum differs") {
		t.Fatalf("incorrect tamper accounting %+v", report)
	}
	if err = freeze(capture, filepath.Join(root, "baseline.json"), filepath.Join(root, "testdata", "recipes.json"), filepath.Join(workspace, "tampered-reference.json")); err == nil || !strings.Contains(err.Error(), "capture output changed") {
		t.Fatalf("export accepted tampered capture: %v", err)
	}
}

func TestWrongCandidateHelper(t *testing.T) {
	for _, a := range os.Args {
		if a == "--" {
			fmt.Print(`{"FullSemVer":"99.0.0","SemVer":"99.0.0","Major":"wrong-type","CommitCountSourceDistance":999999,"SemVerSourceSha":"wrong-source"}`)
			os.Exit(0)
		}
	}
}

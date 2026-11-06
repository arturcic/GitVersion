//go:build expansion || platform

package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

type workflowFailure struct {
	Source   string `json:"source_revision"`
	ID       string `json:"id"`
	Digest   string `json:"digest"`
	ExitCode int    `json:"exit_code"`
}

func loadWorkflowFailure(t *testing.T, root string) workflowFailure {
	t.Helper()
	var f workflowFailure
	if err := parity.ReadJSON(filepath.Join(root, "testdata/workflow-reference-failure.json"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Source != "14d222235b7dabb7033473528a76fd18478a9217" || f.ExitCode != 1 || f.ID == "" || f.Digest == "" {
		t.Fatal("invalid reference rejection contract")
	}
	return f
}
func (f workflowFailure) validate(cp fixtures.Checkpoint, r parity.Result) error {
	if cp.ID != f.ID || cp.Digest != f.Digest {
		return fmt.Errorf("reference rejection fixture changed")
	}
	if r.ExitCode != f.ExitCode || r.Error != "exit status 1" || len(r.Stdout) != 0 || r.Before == "" || r.Before != r.After {
		return fmt.Errorf("expected pinned rejection with unchanged input; got exit=%d error=%q stdout=%q", r.ExitCode, r.Error, r.Stdout)
	}
	return nil
}

func TestWorkflowReferenceRejectionContract(t *testing.T) {
	f := loadWorkflowFailure(t, "../..")
	cp := fixtures.Checkpoint{ID: f.ID, Digest: f.Digest}
	good := parity.Result{ExitCode: 1, Error: "exit status 1", Before: "unchanged", After: "unchanged"}
	if err := f.validate(cp, good); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		result parity.Result
	}{
		{"success", parity.Result{Before: "unchanged", After: "unchanged"}},
		{"different-exit", parity.Result{ExitCode: 2, Error: "exit status 2", Before: "unchanged", After: "unchanged"}},
		{"timeout", parity.Result{ExitCode: 1, Error: "context deadline exceeded", Before: "unchanged", After: "unchanged"}},
		{"mutation", parity.Result{ExitCode: 1, Error: "exit status 1", Before: "before", After: "after"}},
		{"output", parity.Result{ExitCode: 1, Error: "exit status 1", Before: "unchanged", After: "unchanged", Stdout: []byte("{}")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if f.validate(cp, tc.result) == nil {
				t.Fatal("invalid execution accepted as reference rejection")
			}
		})
	}
	cp.Digest = "different"
	if f.validate(cp, good) == nil {
		t.Fatal("changed history accepted")
	}
}

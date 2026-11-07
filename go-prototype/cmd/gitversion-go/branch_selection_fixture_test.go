package main

import (
	"path/filepath"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
)

type branchHistory struct{ path, head, target, base string }

func branchSelectionHistory(t *testing.T, dir string) branchHistory {
	t.Helper()
	repositories, err := fixtures.Generate([]fixtures.Recipe{{
		ID: "branch-selection", Steps: []fixtures.Step{
			{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.0"}, {Op: "checkpoint", ID: "base"},
			{Op: "branch", Name: "feature/work"}, {Op: "checkout", Name: "feature/work"},
			{Op: "commit", ID: "target"}, {Op: "tag", Name: "2.0.0"}, {Op: "checkpoint", ID: "target"},
			{Op: "remote-branch", Name: "origin/feature/tracking"},
			{Op: "checkout", Name: "main"}, {Op: "commit", ID: "head"}, {Op: "checkpoint", ID: "head"},
		},
	}}, filepath.Join(dir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	return branchHistory{repositories[2].Path, repositories[2].Head, repositories[1].Head, repositories[0].Head}
}

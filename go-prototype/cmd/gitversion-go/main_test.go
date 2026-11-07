package main

import (
	"bytes"
	"encoding/json"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"path/filepath"
	"strings"
	"testing"
)

func TestObserveCommand(t *testing.T) {
	cps, err := fixtures.Generate([]fixtures.Recipe{{ID: "cli", Config: "workflow: GitHubFlow/v1\n", Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "head"}}}}, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = run([]string{"observe", "--repository", cps[0].Path}, &output); err != nil {
		t.Fatal(err)
	}
	var got gitrepo.Observation
	if err = json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Head != cps[0].Head || got.Branch != "refs/heads/main" || got.Schema != 1 || len(got.Commits) != 1 || len(got.Walks) != 33 {
		t.Fatalf("incomplete CLI output: %+v", got)
	}
}
func TestUnsupportedCommandProducesNoJSON(t *testing.T) {
	for _, args := range [][]string{{"calculate"}, {"observe", "--repository"}, {"observe", "--repository", "missing", "extra"}} {
		var b bytes.Buffer
		if err := run(args, &b); err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Fatalf("bad command accepted: %v", args)
		}
		if b.Len() != 0 {
			t.Fatal("failure emitted partial JSON")
		}
	}
}

func TestCalculateCLISeparatesTraceAndRejectsUnsupportedConfig(t *testing.T) {
	cps, err := fixtures.Generate([]fixtures.Recipe{{ID: "calculate", Config: "workflow: GitHubFlow/v1\n", Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "6.7.8"}, {Op: "commit", ID: "next"}, {Op: "checkpoint", ID: "head"}}}}, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err = execute([]string{"calculate", "--repository", cps[0].Path, "--output", "json", "--trace"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err = json.Unmarshal(stdout.Bytes(), &values); err != nil {
		t.Fatal(err)
	}
	if values["FullSemVer"] != "6.7.9-1" || len(values) != 33 {
		t.Fatalf("CLI output %+v", values)
	}
	var trace map[string]any
	if err = json.Unmarshal(stderr.Bytes(), &trace); err != nil {
		t.Fatal(err)
	}
	if trace["semantic_source"] != "6.7.8" || len(trace["counted_commits"].([]any)) != 1 {
		t.Fatalf("CLI trace %+v", trace)
	}
	stdout.Reset()
	stderr.Reset()
	if err = execute([]string{"calculate", "--repository", t.TempDir()}, &stdout, &stderr); err == nil || stdout.Len() != 0 {
		t.Fatal("invalid repository emitted partial JSON")
	}
}

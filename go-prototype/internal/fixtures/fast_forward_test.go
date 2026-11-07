package fixtures

import (
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"path/filepath"
	"strings"
	"testing"
)

func TestFastForwardPreservesTargetGraph(t *testing.T) {
	recipe := Recipe{ID: "fast-forward", Config: "workflow: TrunkBased/preview1\n", Steps: []Step{
		{Op: "commit", ID: "root"}, {Op: "branch", Name: "topic"}, {Op: "checkout", Name: "topic"}, {Op: "commit", ID: "work"}, {Op: "checkpoint", ID: "target"},
		{Op: "checkout", Name: "main"}, {Op: "fast-forward", From: "topic"}, {Op: "checkpoint", ID: "head"}, {Op: "fast-forward", From: "main"}, {Op: "checkpoint", ID: "identity"},
	}}
	cps, err := Generate([]Recipe{recipe}, filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cps) != 3 || cps[1].Head != cps[2].Head || cps[0].Head != cps[1].Head || cps[0].Branch != "topic" || cps[1].Branch != "main" {
		t.Fatal("fast-forward must move only the branch ref")
	}
	r, err := git.PlainOpen(cps[1].Path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.CommitObject(plumbing.NewHash(cps[1].Head))
	if err != nil {
		t.Fatal(err)
	}
	if c.NumParents() != 1 {
		t.Fatal("fast-forward created an extra merge")
	}
}

func TestFastForwardRejectsInvalidAncestry(t *testing.T) {
	for _, tc := range []struct {
		name  string
		steps []Step
		from  string
	}{
		{"missing-source", nil, "missing"},
		{"rewind", []Step{{Op: "branch", Name: "older"}, {Op: "commit", ID: "work"}}, "older"},
		{"diverged", []Step{{Op: "branch", Name: "topic"}, {Op: "commit", ID: "main-work"}, {Op: "checkout", Name: "topic"}, {Op: "commit", ID: "topic-work"}, {Op: "checkout", Name: "main"}}, "topic"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			steps := []Step{{Op: "commit", ID: "root"}}
			steps = append(steps, tc.steps...)
			steps = append(steps, Step{Op: "fast-forward", From: tc.from}, Step{Op: "checkpoint", ID: "head"})
			if _, err := Generate([]Recipe{{ID: "invalid-forward", Steps: steps}}, filepath.Join(t.TempDir(), "history")); err == nil || !strings.Contains(err.Error(), "fast-forward") {
				t.Fatal("invalid fast-forward diagnostic differs", err)
			}
		})
	}
}

package gitrepo

import (
	"encoding/json"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/go-git/go-git/v5/plumbing"
)

func tiedCrissCross(t *testing.T) graphFixture {
	return makeGraph(t, []graphCommit{{"root", nil, 0, nil}, {"left", []string{"root"}, 0, nil}, {"right", []string{"root"}, 0, nil}, {"merge-left", []string{"left", "right"}, 0, nil}, {"merge-right", []string{"right", "left"}, 0, nil}}, map[string]string{"main": "merge-left", "other": "merge-right"}, "main")
}
func unicodeRefs(t *testing.T) graphFixture {
	return makeGraph(t, []graphCommit{{"root", nil, 0, nil}}, map[string]string{"main": "root", "\ue000": "root", "\U00010000": "root"}, "main")
}

func TestUnicodeReferenceOrder(t *testing.T) {
	f := unicodeRefs(t)
	refs, err := openGraph(t, f).References()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"refs/heads/main", "refs/heads/\U00010000", "refs/heads/\ue000"}
	if len(refs) != len(want) {
		t.Fatalf("reference count %d", len(refs))
	}
	for i, name := range want {
		if refs[i].Name != name {
			t.Fatalf("ref %d: %q, want %q", i, refs[i].Name, name)
		}
	}
}

func TestNonCommitTagFailsClosed(t *testing.T) {
	f := diamond(t, false)
	commit, err := f.git.CommitObject(plumbing.NewHash(f.ids["root"]))
	if err != nil {
		t.Fatal(err)
	}
	if err = f.git.Storer.SetReference(plumbing.NewHashReference("refs/tags/tree", commit.TreeHash)); err != nil {
		t.Fatal(err)
	}
	if _, err = openGraph(t, f).Observe(); err == nil {
		t.Fatal("non-commit tag accepted")
	}
}

func TestObservationComparatorDetectsSeededChanges(t *testing.T) {
	f := diamond(t, false)
	obs, err := openGraph(t, f).Observe()
	if err != nil {
		t.Fatal(err)
	}
	expected, err := json.Marshal(obs)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Observation)
	}{
		{"walk-order", func(o *Observation) { ids := o.Walks["head/2/false"]; ids[0], ids[1] = ids[1], ids[0] }},
		{"merge-base", func(o *Observation) {
			key := f.ids["merge"] + "/" + f.ids["right"]
			bad := f.ids["root"]
			o.MergeBases[key] = &bad
		}},
		{"missing-parent", func(o *Observation) {
			c := o.Commits[f.ids["merge"]]
			c.Parents = c.Parents[:1]
			o.Commits[f.ids["merge"]] = c
		}},
		{"missing-ref", func(o *Observation) { o.References = o.References[:1] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var changed Observation
			if err := json.Unmarshal(expected, &changed); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&changed)
			actual, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			diff, err := parity.Compare(expected, actual)
			if err != nil {
				t.Fatal(err)
			}
			if len(diff) != 1 {
				t.Fatalf("seeded observation difference: got %d differences, want 1", len(diff))
			}
		})
	}
}

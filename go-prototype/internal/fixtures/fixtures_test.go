package fixtures

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func TestCorpusDeterministicAndClean(t *testing.T) {
	recipes, err := Load("../../testdata/recipes.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(recipes) != 25 {
		t.Fatalf("corpus inventory changed: %d recipes", len(recipes))
	}
	a, err := Generate(recipes, filepath.Join(t.TempDir(), "first"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(recipes, filepath.Join(t.TempDir(), "second"))
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) || len(a) < 35 {
		t.Fatalf("incomplete checkpoint inventory: %d/%d", len(a), len(b))
	}
	for i, cp := range a {
		t.Run(cp.ID, func(t *testing.T) {
			other := b[i]
			if cp.ID != other.ID || cp.Head != other.Head || cp.Digest != other.Digest {
				t.Fatalf("nondeterministic checkpoint: %+v / %+v", cp, other)
			}
			sa, err := State(cp.Path)
			if err != nil {
				t.Fatal(err)
			}
			sb, err := State(other.Path)
			if err != nil {
				t.Fatal(err)
			}
			if sa != sb {
				t.Fatalf("state differs %s/%s", sa, sb)
			}
			repo, err := git.PlainOpen(cp.Path)
			if err != nil {
				t.Fatal(err)
			}
			head, err := repo.Head()
			if err != nil || head.Hash().String() != cp.Head {
				t.Fatalf("HEAD mismatch: %v %v", head, err)
			}
			wt, err := repo.Worktree()
			if err != nil {
				t.Fatal(err)
			}
			status, err := wt.Status()
			if err != nil || !status.IsClean() {
				t.Fatalf("dirty fixture: %v %v", status, err)
			}
		})
	}
}

func TestAnnotatedTagsAndTrueMerge(t *testing.T) {
	recipes, err := Load("../../testdata/recipes.json")
	if err != nil {
		t.Fatal(err)
	}
	cps, err := Generate(recipes, filepath.Join(t.TempDir(), "corpus"))
	if err != nil {
		t.Fatal(err)
	}
	for _, cp := range cps {
		switch cp.ID {
		case "annotated-tag--tagged":
			repo, _ := git.PlainOpen(cp.Path)
			ref, err := repo.Tag("2.0.0")
			if err != nil {
				t.Fatal(err)
			}
			tag, err := repo.TagObject(ref.Hash())
			if err != nil {
				t.Fatal(err)
			}
			if tag.Target.String() != cp.Head || tag.TargetType != plumbing.CommitObject {
				t.Fatalf("invalid annotated tag: %+v", tag)
			}
		case "feature-merge--merged":
			repo, _ := git.PlainOpen(cp.Path)
			commit, err := repo.CommitObject(plumbing.NewHash(cp.Head))
			if err != nil {
				t.Fatal(err)
			}
			if len(commit.ParentHashes) != 2 || commit.ParentHashes[0] == commit.ParentHashes[1] {
				t.Fatalf("not a true merge: %+v", commit.ParentHashes)
			}
			tree, err := commit.Tree()
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tree.File("feature.txt"); err != nil {
				t.Fatal("merged tree lost feature file", err)
			}
		}
	}
}

func TestGenerateRejectsInvalidRecipesAndExistingOutput(t *testing.T) {
	base := Recipe{ID: "case", Config: "workflow: GitHubFlow/v1\n", Steps: []Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "head"}}}
	cases := []struct {
		name  string
		steps []Step
	}{
		{"unknown-operation", []Step{{Op: "unknown"}}}, {"missing-branch", []Step{{Op: "checkout", Name: "missing"}}},
		{"unsafe-commit", []Step{{Op: "commit", ID: "../escape"}}}, {"unsafe-checkpoint", []Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "../escape"}}},
		{"missing-merge-parent", []Step{{Op: "commit", ID: "root"}, {Op: "merge", ID: "merge", From: "missing"}}},
		{"duplicate-checkpoint", append(append([]Step{}, base.Steps...), Step{Op: "checkpoint", ID: "head"})},
		{"duplicate-commit", []Step{{Op: "commit", ID: "root"}, {Op: "commit", ID: "root"}}},
		{"empty-history", []Step{{Op: "checkpoint", ID: "head"}}}, {"no-checkpoints", []Step{{Op: "commit", ID: "root"}}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := base
			r.Steps = tt.steps
			if _, err := Generate([]Recipe{r}, filepath.Join(t.TempDir(), "out")); err == nil {
				t.Fatal("accepted invalid recipe")
			}
		})
	}
	if _, err := Generate([]Recipe{base}, t.TempDir()); err == nil {
		t.Fatal("overwrote existing output")
	}
}

func TestStateDetectsWorktreeRefsAndIndexMutations(t *testing.T) {
	r := Recipe{ID: "case", Config: "workflow: GitHubFlow/v1\n", Steps: []Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "head"}}}
	cps, err := Generate([]Recipe{r}, filepath.Join(t.TempDir(), "source"))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"worktree", "ref", "index"} {
		t.Run(kind, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "repo")
			if err := Copy(cps[0].Path, dest); err != nil {
				t.Fatal(err)
			}
			before, err := State(dest)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "worktree":
				err = os.WriteFile(filepath.Join(dest, "root.txt"), []byte("changed"), 0644)
			case "ref":
				err = os.WriteFile(filepath.Join(dest, ".git", "refs", "heads", "extra"), []byte(cps[0].Head+"\n"), 0644)
			case "index":
				repo, e := git.PlainOpen(dest)
				if e != nil {
					t.Fatal(e)
				}
				idx, e := repo.Storer.Index()
				if e != nil {
					t.Fatal(e)
				}
				idx.Entries[0].Hash = plumbing.NewHash("1234567890123456789012345678901234567890")
				err = repo.Storer.SetIndex(idx)
			}
			if err != nil {
				t.Fatal(err)
			}
			after, err := State(dest)
			if err != nil {
				t.Fatal(err)
			}
			if before == after {
				t.Fatalf("missed %s mutation", kind)
			}
		})
	}
}

func TestLoadRejectsDuplicateAndUnsafeIDs(t *testing.T) {
	for _, data := range []string{`[]`, `[{"id":"../escape"}]`, `[{"id":"case"},{"id":"case"}]`, `not-json`} {
		path := filepath.Join(t.TempDir(), "recipes.json")
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("accepted invalid corpus %s", data)
		}
	}
}

func TestCopyRefusesSymlinks(t *testing.T) {
	src := t.TempDir()
	if err := os.Symlink("outside", filepath.Join(src, "link")); err != nil {
		t.Skip("symlinks unavailable")
	}
	if err := Copy(src, filepath.Join(t.TempDir(), "copy")); err == nil {
		t.Fatal("followed fixture symlink")
	}
}

func TestCheckpointHasNoFutureRefs(t *testing.T) {
	r := Recipe{ID: "case", Config: "workflow: GitHubFlow/v1\n", Steps: []Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "before"}, {Op: "branch", Name: "feature/future"}, {Op: "tag", Name: "9.0.0"}, {Op: "checkpoint", ID: "after"}}}
	cps, err := Generate([]Recipe{r}, filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := git.PlainOpen(cps[0].Path)
	b, _ := git.PlainOpen(cps[1].Path)
	if _, err = a.Tag("9.0.0"); err == nil {
		t.Fatal("future tag leaked")
	}
	if _, err = b.Tag("9.0.0"); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(cps[0].Digest, cps[1].Digest) {
		t.Fatal("ref changes absent from identity")
	}
}

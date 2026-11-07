package gitrepo

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type graphCommit struct {
	name    string
	parents []string
	seconds int64
	files   map[string]string
}

func TestInfoExcludePrecedenceAndTrackedChanges(t *testing.T) {
	for _, tc := range []struct {
		name, ignore, file string
		wantClean          bool
		stage              bool
	}{
		{"untracked-excluded", "", "scratch.txt", true, false},
		{"gitignore-negates-exclusion", "!scratch.txt\n", "scratch.txt", false, false},
		{"tracked-still-dirty", "", "file.txt", false, false},
		{"newly-staged-still-dirty", "", "scratch.txt", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{"file.txt": "original\n"}
			if tc.ignore != "" {
				files[".gitignore"] = tc.ignore
			}
			g := makeGraph(t, []graphCommit{{name: "root", files: files}}, map[string]string{"main": "root"}, "main")
			if err := os.MkdirAll(filepath.Join(g.path, ".git/info"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(g.path, ".git/info/exclude"), []byte("# comment\n/*.txt\r\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(g.path, tc.file), []byte("changed\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if tc.stage {
				wt, err := g.git.Worktree()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := wt.Add(tc.file); err != nil {
					t.Fatal(err)
				}
			}
			_, err := Open(g.path)
			if (err == nil) != tc.wantClean {
				t.Fatalf("clean=%v error=%v", tc.wantClean, err)
			}
			if !tc.wantClean && !strings.Contains(err.Error(), "dirty repository") {
				t.Fatalf("wrong rejection: %v", err)
			}
		})
	}
}

type graphFixture struct {
	path string
	git  *git.Repository
	ids  map[string]string
}

func makeGraph(t *testing.T, specs []graphCommit, branches map[string]string, head string) graphFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "repo")
	g, err := git.PlainInitWithOptions(path, &git.PlainInitOptions{InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")}})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, s := range specs {
		files := s.files
		if files == nil {
			files = map[string]string{"file.txt": s.name + "\n"}
		}
		names := []string{}
		for name := range files {
			names = append(names, name)
		}
		sort.Strings(names)
		tree := &object.Tree{}
		for _, name := range names {
			o := g.Storer.NewEncodedObject()
			o.SetType(plumbing.BlobObject)
			w, err := o.Writer()
			if err != nil {
				t.Fatal(err)
			}
			if _, err = w.Write([]byte(files[name])); err != nil {
				t.Fatal(err)
			}
			if err = w.Close(); err != nil {
				t.Fatal(err)
			}
			h, err := g.Storer.SetEncodedObject(o)
			if err != nil {
				t.Fatal(err)
			}
			tree.Entries = append(tree.Entries, object.TreeEntry{Name: name, Mode: filemode.Regular, Hash: h})
		}
		o := g.Storer.NewEncodedObject()
		if err = tree.Encode(o); err != nil {
			t.Fatal(err)
		}
		treeID, err := g.Storer.SetEncodedObject(o)
		if err != nil {
			t.Fatal(err)
		}
		parents := []plumbing.Hash{}
		for _, p := range s.parents {
			id, ok := ids[p]
			if !ok {
				t.Fatalf("missing graph parent %s", p)
			}
			parents = append(parents, plumbing.NewHash(id))
		}
		sig := object.Signature{Name: "Observation Fixture", Email: "observer@example.invalid", When: time.Unix(1735689600+s.seconds, 0).UTC()}
		c := object.Commit{Author: sig, Committer: sig, Message: s.name + "\n", TreeHash: treeID, ParentHashes: parents}
		o = g.Storer.NewEncodedObject()
		if err = c.Encode(o); err != nil {
			t.Fatal(err)
		}
		h, err := g.Storer.SetEncodedObject(o)
		if err != nil {
			t.Fatal(err)
		}
		ids[s.name] = h.String()
	}
	for name, tip := range branches {
		if err = g.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName(name), plumbing.NewHash(ids[tip]))); err != nil {
			t.Fatal(err)
		}
	}
	wt, err := g.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err = wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(head), Force: true}); err != nil {
		t.Fatal(err)
	}
	return graphFixture{path, g, ids}
}
func openGraph(t *testing.T, f graphFixture) *Repository {
	t.Helper()
	r, err := Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func diamond(t *testing.T, tied bool) graphFixture {
	t.Helper()
	times := []int64{0, 60, 120, 180}
	if tied {
		times = []int64{0, 0, 0, 0}
	}
	return makeGraph(t, []graphCommit{{"root", nil, times[0], nil}, {"left", []string{"root"}, times[1], nil}, {"right", []string{"root"}, times[2], nil}, {"merge", []string{"left", "right"}, times[3], nil}}, map[string]string{"main": "merge", "feature": "right"}, "main")
}
func assertIDs(t *testing.T, got []string, f graphFixture, want ...string) {
	t.Helper()
	ids := []string{}
	for _, name := range want {
		ids = append(ids, f.ids[name])
	}
	if !reflect.DeepEqual(got, ids) {
		t.Fatalf("order mismatch: got %v, want names %v (%v)", got, want, ids)
	}
}

func TestWalkOrderingAndExclusion(t *testing.T) {
	f := diamond(t, false)
	r := openGraph(t, f)
	cases := []struct {
		name    string
		sort    Sort
		first   bool
		exclude string
		want    []string
	}{
		{"date", Time, false, "", []string{"merge", "right", "left", "root"}},
		{"topological", Topological, false, "", []string{"merge", "right", "left", "root"}},
		{"reverse", Time | Reverse, false, "", []string{"root", "left", "right", "merge"}},
		{"first-parent", Time, true, "", []string{"merge", "left", "root"}},
		{"exclude-side", Time, false, "right", []string{"merge", "left"}},
		{"exclude-merge-hides-all-parents", Time, true, "merge", []string{}},
		{"first-parent-exclude-side", Time, true, "right", []string{"merge", "left"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.Walk(WalkOptions{Include: f.ids["merge"], Exclude: f.ids[tc.exclude], Sort: tc.sort, FirstParent: tc.first})
			if err != nil {
				t.Fatal(err)
			}
			assertIDs(t, got, f, tc.want...)
		})
	}
	if _, err := r.Walk(WalkOptions{Include: f.ids["merge"], Sort: 8}); err == nil {
		t.Fatal("unknown flags accepted")
	}
	if _, err := r.Walk(WalkOptions{Include: strings.Repeat("0", 40), Sort: Time}); err == nil {
		t.Fatal("missing commit accepted")
	}
	empty, err := r.Walk(WalkOptions{Sort: Time})
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty include: %v %v", empty, err)
	}
}

func TestEqualTimestampsAndSkewedTopology(t *testing.T) {
	f := diamond(t, true)
	r := openGraph(t, f)
	// All four commits tie. The reference heap moves the final element to its
	// root after popping; a stable timestamp sort gives a different result.
	got, err := r.Walk(WalkOptions{Include: f.ids["merge"], Sort: Time})
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, got, f, "merge", "root", "right", "left")
	got, err = r.Walk(WalkOptions{Include: f.ids["merge"], Sort: Topological | Time})
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, got, f, "merge", "left", "right", "root")
	skew := makeGraph(t, []graphCommit{{"root", nil, 500, nil}, {"child", []string{"root"}, 0, nil}}, map[string]string{"main": "child"}, "main")
	r = openGraph(t, skew)
	got, err = r.Walk(WalkOptions{Include: skew.ids["child"], Sort: Time})
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, got, skew, "root", "child")
	got, err = r.Walk(WalkOptions{Include: skew.ids["child"], Sort: Topological | Time})
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, got, skew, "child", "root")
}

func crissCross(t *testing.T) graphFixture {
	return makeGraph(t, []graphCommit{{"root", nil, 0, nil}, {"left", []string{"root"}, 10, nil}, {"right", []string{"root"}, 20, nil}, {"merge-left", []string{"left", "right"}, 30, nil}, {"merge-right", []string{"right", "left"}, 40, nil}, {"unrelated", nil, 50, nil}}, map[string]string{"main": "merge-left", "other": "merge-right", "unrelated": "unrelated"}, "main")
}
func TestMergeBase(t *testing.T) {
	f := crissCross(t)
	r := openGraph(t, f)
	for _, tc := range []struct{ name, a, b, want string }{{"identity", "left", "left", "left"}, {"ancestor", "merge-left", "root", "root"}, {"divergent", "left", "right", "root"}, {"criss-cross", "merge-left", "merge-right", "right"}, {"unrelated", "merge-left", "unrelated", ""}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.MergeBase(f.ids[tc.a], f.ids[tc.b])
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if got != nil {
					t.Fatalf("unrelated base %s", *got)
				}
			} else if got == nil || *got != f.ids[tc.want] {
				t.Fatalf("base %v, want %s", got, tc.want)
			}
		})
	}
	if _, err := r.MergeBase(f.ids["left"], strings.Repeat("0", 40)); err == nil {
		t.Fatal("missing merge-base root accepted")
	}
}

func addNestedTags(t *testing.T, f graphFixture) {
	t.Helper()
	head := plumbing.NewHash(f.ids["merge"])
	if _, err := f.git.CreateTag("light", head, nil); err != nil {
		t.Fatal(err)
	}
	sig := object.Signature{Name: "Tagger", Email: "tagger@example.invalid", When: time.Unix(1735776000, 0).UTC()}
	tag, err := f.git.CreateTag("inner", head, &git.CreateTagOptions{Tagger: &sig, Message: "inner\n"})
	if err != nil {
		t.Fatal(err)
	}
	outer := object.Tag{Name: "outer", Tagger: sig, Message: "outer\n", TargetType: plumbing.TagObject, Target: tag.Hash()}
	o := f.git.Storer.NewEncodedObject()
	if err = outer.Encode(o); err != nil {
		t.Fatal(err)
	}
	hash, err := f.git.Storer.SetEncodedObject(o)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.git.Storer.SetReference(plumbing.NewHashReference(plumbing.NewTagReferenceName("outer"), hash)); err != nil {
		t.Fatal(err)
	}
}
func TestTagsAndDiffPaths(t *testing.T) {
	f := diamond(t, false)
	addNestedTags(t, f)
	r := openGraph(t, f)
	refs, err := r.References()
	if err != nil {
		t.Fatal(err)
	}
	tags, err := r.Tags(refs)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := f.git.Tag("inner")
	if err != nil {
		t.Fatal(err)
	}
	want := []Tag{{"refs/tags/inner", f.ids["merge"], f.ids["merge"]}, {"refs/tags/light", f.ids["merge"], f.ids["merge"]}, {"refs/tags/outer", inner.Hash().String(), f.ids["merge"]}}
	if !reflect.DeepEqual(tags, want) {
		t.Fatalf("tag peeling: %+v, want %+v", tags, want)
	}
	for _, name := range []string{"root", "merge"} {
		info, err := r.commitInfo(f.ids[name])
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(info.DiffPaths, []string{"file.txt"}) || info.Message != name+"\n" {
			t.Fatalf("commit info %s: %+v", name, info)
		}
	}
	info, err := r.commitInfo(f.ids["merge"])
	if err != nil {
		t.Fatal(err)
	}
	if info.When != 1735689780 {
		t.Fatalf("committer timestamp %d", info.When)
	}
	assertIDs(t, info.Parents, f, "left", "right")
	d := makeGraph(t, []graphCommit{{"before", nil, 0, map[string]string{"old.txt": "same", "modified.txt": "before"}}, {"after", []string{"before"}, 1, map[string]string{"new.txt": "same", "modified.txt": "after"}}}, map[string]string{"main": "after"}, "main")
	info, err = openGraph(t, d).commitInfo(d.ids["after"])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(info.DiffPaths, []string{"modified.txt", "new.txt", "old.txt"}) {
		t.Fatalf("changed paths %v", info.DiffPaths)
	}
}

func TestOpenRejectsUnsupportedStates(t *testing.T) {
	for _, name := range []string{"detached", "dirty", "staged", "untracked", "shallow", "empty", "bare"} {
		t.Run(name, func(t *testing.T) {
			f := diamond(t, false)
			var err error
			switch name {
			case "untracked":
				err = os.WriteFile(filepath.Join(f.path, "untracked.txt"), []byte("untracked\n"), 0644)
			case "detached":
				err = f.git.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, plumbing.NewHash(f.ids["merge"])))
			case "dirty", "staged":
				err = os.WriteFile(filepath.Join(f.path, "file.txt"), []byte("changed\n"), 0644)
				if err == nil && name == "staged" {
					var wt *git.Worktree
					wt, err = f.git.Worktree()
					if err == nil {
						_, err = wt.Add("file.txt")
					}
				}
			case "shallow":
				err = f.git.Storer.SetShallow([]plumbing.Hash{plumbing.NewHash(f.ids["root"])})
			case "empty", "bare":
				f.path = filepath.Join(t.TempDir(), "empty")
				_, err = git.PlainInit(f.path, name == "bare")
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = Open(f.path); err == nil {
				t.Fatal("unsupported state accepted")
			}
		})
	}
}

func TestMissingParentFailsClosed(t *testing.T) {
	f := diamond(t, false)
	id := f.ids["left"]
	if err := os.Remove(filepath.Join(f.path, ".git", "objects", id[:2], id[2:])); err != nil {
		t.Fatal(err)
	}
	r := openGraph(t, f)
	if _, err := r.Observe(); err == nil {
		t.Fatal("missing reachable parent silently ignored")
	}
}

func TestPackedReferenceOrdering(t *testing.T) {
	f := diamond(t, false)
	addNestedTags(t, f)
	// Pack existing refs, then add a lexically earlier packed tag and a late loose
	// branch. The backend's loose-first order differs from global name order.
	if err := f.git.Storer.PackRefs(); err != nil {
		t.Fatal(err)
	}
	if err := f.git.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("zzz"), plumbing.NewHash(f.ids["merge"]))); err != nil {
		t.Fatal(err)
	}
	refs, err := openGraph(t, f).References()
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, r := range refs {
		names = append(names, r.Name)
	}
	want := []string{"refs/heads/zzz", "refs/heads/feature", "refs/heads/main", "refs/tags/inner", "refs/tags/light", "refs/tags/outer"}
	if !slices.Equal(names, want) {
		t.Fatalf("refs %v, want %v", names, want)
	}
}

func TestBranchReferencesResolveAndPrivateReferencesAreIgnored(t *testing.T) {
	f := diamond(t, false)
	for _, ref := range []*plumbing.Reference{
		plumbing.NewHashReference("refs/remotes/origin/main", plumbing.NewHash(f.ids["merge"])),
		plumbing.NewSymbolicReference("refs/remotes/origin/HEAD", "refs/remotes/origin/main"),
		plumbing.NewSymbolicReference("refs/heads/alias", "refs/heads/main"),
		plumbing.NewHashReference("refs/codex/private", plumbing.NewHash(f.ids["root"])),
	} {
		if err := f.git.Storer.SetReference(ref); err != nil {
			t.Fatal(err)
		}
	}
	refs, err := openGraph(t, f).References()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]Reference{}
	for _, ref := range refs {
		found[ref.Name] = ref
	}
	for _, name := range []string{"refs/remotes/origin/main", "refs/remotes/origin/HEAD", "refs/heads/alias"} {
		if found[name].SHA != f.ids["merge"] {
			t.Fatal("reference resolution", name, found[name])
		}
	}
	if found["refs/remotes/origin/HEAD"].Target != "refs/remotes/origin/main" {
		t.Fatal("symbolic target discarded")
	}
	if _, ok := found["refs/codex/private"]; ok {
		t.Fatal("private reference leaked into branch/tag namespace")
	}
}

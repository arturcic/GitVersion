package gitrepo

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/go-git/go-git/v5/plumbing"
)

type dualBackendCorpus struct {
	Revision, Reference string
	ReferenceSHA256     string `json:"reference_sha256"`
	Methods             map[string][]struct{ ID string }
}

type dualBackendReference struct {
	Revision string                     `json:"revision"`
	Cases    map[string]json.RawMessage `json:"cases"`
}

func readDualBackendCorpus(t *testing.T) dualBackendCorpus {
	t.Helper()
	var c dualBackendCorpus
	if err := parity.ReadJSON("../../testdata/csharp-dual-backend.json", &c); err != nil {
		t.Fatal(err)
	}
	if c.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(c.Methods) != 19 {
		t.Fatal("original dual-backend inventory changed")
	}
	return c
}

func readDualBackendReference(t *testing.T, c dualBackendCorpus) dualBackendReference {
	t.Helper()
	path := filepath.Join("../..", c.Reference)
	hash, err := parity.FileHash(path)
	if err != nil || hash != c.ReferenceSHA256 {
		t.Fatal("C# dual-backend reference hash differs", err)
	}
	var ref dualBackendReference
	if err := parity.ReadJSON(path, &ref); err != nil {
		t.Fatal(err)
	}
	if ref.Revision != c.Revision || len(ref.Cases) != 19 {
		t.Fatal("C# dual-backend reference inventory changed")
	}
	return ref
}

func TestCSharpDualBackendParity(t *testing.T) {
	corpus := readDualBackendCorpus(t)
	ref := readDualBackendReference(t, corpus)
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.ID, func(t *testing.T) {
					fixture := newDualBackendFixture(t, method)
					got := observeDualBackend(t, method, fixture)
					raw, err := json.Marshal(got)
					if err != nil {
						t.Fatal(err)
					}
					diff, err := compareDualBackend(ref.Cases[method], raw)
					if err != nil || len(diff) != 0 {
						t.Fatalf("pinned C# and Go differ: %v %+v", err, diff)
					}
				})
			}
		})
	}
}

type dualHead struct {
	Name string  `json:"name"`
	Tip  *string `json:"tip"`
}

type dualBranch struct {
	dualHead
	Remote   bool `json:"remote"`
	Tracking bool `json:"tracking"`
	Detached bool `json:"detached"`
}

type dualView struct {
	t       *testing.T
	layout  repositoryLayout
	repo    *Repository
	refs    []Reference
	head    dualHead
	rawHead *plumbing.Reference
	shallow bool
}

func observeDualBackend(t *testing.T, method string, fixture dualBackendFixture) any {
	t.Helper()
	layout, err := discoverLayout(fixture.path)
	if err != nil {
		t.Fatal(err)
	}
	location := func(path string) any {
		if path == "" {
			return nil
		}
		relative, err := filepath.Rel(fixture.root, path)
		if err != nil {
			t.Fatal(err)
		}
		return filepath.ToSlash(relative)
	}
	paths := map[string]any{"git": location(layout.gitDirectory), "root": location(layout.workingDirectory)}
	if method == "BareRepositoryHasNullProjectRootDirectoryOnBothBackends" ||
		method == "RepositoryPathsAreIdenticalOnBothBackends" {
		return paths
	}
	git, err := layout.open()
	if err != nil {
		t.Fatal(err)
	}
	shallow, err := layout.readShallowCommits()
	if err != nil {
		t.Fatal(err)
	}
	repo := newCommitRepository(layout, git, shallow)
	refs, err := repo.References()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := git.Reference(plumbing.HEAD, false)
	if err != nil {
		t.Fatal(err)
	}
	head := dualHead{Name: "(no branch)"}
	if raw.Type() == plumbing.SymbolicReference {
		head.Name = raw.Target().String()
	}
	resolved, err := git.Head()
	if err == nil {
		id := resolved.Hash().String()
		head.Tip = &id
	} else if !errors.Is(err, plumbing.ErrReferenceNotFound) {
		t.Fatal(err)
	}
	view := dualView{t: t, layout: layout, repo: repo, refs: refs, head: head, rawHead: raw, shallow: len(shallow) != 0}
	switch method {
	case "ReferencesAreIdenticalOnBothBackends":
		var glob []map[string]string
		for _, ref := range refs {
			if plumbing.ReferenceName(ref.Name).IsBranch() {
				glob = append(glob, map[string]string{"name": ref.Name, "target": ref.Target})
			}
		}
		return map[string]any{"refs": view.references(), "head": map[string]string{"name": "HEAD", "target": raw.Target().String()}, "glob": glob}
	case "BranchesAreIdenticalOnBothBackends":
		branches := view.branches()
		lookups := []*dualHead{}
		for _, name := range []string{"main", "develop", "refs/heads/develop", "feature/complex", "missing"} {
			var found *dualHead
			for _, branch := range branches {
				if NewReferenceName(branch.Name).EquivalentTo(&name) {
					match := branch.dualHead
					found = &match
					break
				}
			}
			lookups = append(lookups, found)
		}
		return map[string]any{"branches": branches, "lookups": lookups, "head": head,
			"detached": view.detached(), "shallow": view.shallow}
	case "TagsAreIdenticalOnBothBackends", "LightweightAndAnnotatedTagOnTheSameCommitBehaveIdenticallyOnBothBackends":
		return view.tags()
	case "CommitLogsAreIdenticalOnBothBackends":
		var branchLogs [][]string
		for _, name := range []string{"main", "develop", "feature/complex"} {
			ref, err := git.Reference(plumbing.NewBranchReferenceName(name), true)
			if err != nil {
				t.Fatal(err)
			}
			branchLogs = append(branchLogs, view.walk(WalkOptions{Include: ref.Hash().String(), Sort: Time}))
		}
		return map[string]any{"log": view.log(true), "branches": branchLogs}
	case "QueryBySequencesAreIdenticalOnBothBackends":
		return view.queries()
	case "MergeBasesAreIdenticalOnBothBackendsForAllCommitPairs":
		return view.bases()
	case "DiffPathsAreIdenticalOnBothBackendsForAllCommits":
		return view.diffs()
	case "UncommittedChangesCountIsIdenticalOnBothBackends", "IndexVersionFourBehavesIdenticallyOnBothBackends":
		return view.changes()
	case "EmptyRepositoryBehavesIdenticallyOnBothBackends":
		return map[string]any{"head": head, "detached": view.detached(), "changes": view.changes()}
	case "DetachedHeadBehavesIdenticallyOnBothBackends":
		if !view.detached() {
			t.Fatal("fixture HEAD is attached")
		}
		return map[string]any{"head": head, "detached": view.detached(), "headDetached": head.Name == "(no branch)"}
	case "RemotesAndRemoteBranchesAreIdenticalOnBothBackends":
		cfg, err := git.Config()
		if err != nil {
			t.Fatal(err)
		}
		var remotes []map[string]any
		for name, remote := range cfg.Remotes {
			fetch := []map[string]string{}
			for _, spec := range remote.Fetch {
				specification := spec.String()
				src, dst, ok := strings.Cut(strings.TrimPrefix(specification, "+"), ":")
				if !ok {
					t.Fatal("invalid refspec", specification)
				}
				fetch = append(fetch, map[string]string{"specification": specification, "direction": "Fetch", "source": src, "destination": dst})
			}
			push := cfg.Raw.Section("remote").Subsection(name).OptionAll("push")
			if push == nil {
				push = []string{}
			}
			remotes = append(remotes, map[string]any{"name": name, "url": location(remote.URLs[0]), "fetch": fetch, "push": push})
		}
		sort.Slice(remotes, func(i, j int) bool { return remotes[i]["name"].(string) < remotes[j]["name"].(string) })
		return map[string]any{"remotes": remotes, "origin": location(cfg.Remotes["origin"].URLs[0]), "branches": view.branches()}
	case "OctopusMergeBehavesIdenticallyOnBothBackends":
		c, err := repo.CommitMetadata(*head.Tip)
		if err != nil || len(c.Parents) != 3 {
			t.Fatalf("octopus parents %v, error %v", c.Parents, err)
		}
		return map[string]any{"parents": len(c.Parents), "log": view.log(false), "bases": view.bases(), "diffs": view.diffs()}
	case "PackedAndLooseReferencesBehaveIdenticallyOnBothBackends":
		var branches []dualHead
		for _, branch := range view.branches() {
			branches = append(branches, branch.dualHead)
		}
		return map[string]any{"refs": view.references(), "tags": view.tags(), "branches": branches}
	case "ShallowCloneBehavesIdenticallyOnBothBackends":
		if !view.shallow {
			t.Fatal("fixture lacks a shallow boundary")
		}
		return map[string]any{"shallow": view.shallow, "log": view.log(false)}
	case "WorktreeBehavesIdenticallyOnBothBackends":
		return map[string]any{"paths": paths, "head": head, "changes": view.changes(), "refs": view.references(), "log": view.log(false)}
	default:
		t.Fatal("original assertion missing", method)
		return nil
	}
}

func (v dualView) detached() bool {
	return v.rawHead.Type() == plumbing.HashReference
}

func (v dualView) references() []map[string]any {
	out := make([]map[string]any, 0, len(v.refs))
	for _, ref := range v.refs {
		var id any
		if ref.Target == ref.SHA {
			id = ref.SHA
		}
		out = append(out, map[string]any{"name": ref.Name, "target": ref.Target, "id": id})
	}
	return out
}

func (v dualView) branches() []dualBranch {
	cfg, err := v.repo.git.Config()
	if err != nil {
		v.t.Fatal(err)
	}
	branches := []dualBranch{}
	for _, ref := range v.refs {
		name := NewReferenceName(ref.Name)
		if !name.IsLocalBranch && !name.IsRemoteBranch {
			continue
		}
		tracking := false
		if b := cfg.Branches[name.Friendly]; name.IsLocalBranch && b != nil {
			tracking = b.Remote != "" && b.Merge != ""
		}
		tip := ref.SHA
		branches = append(branches, dualBranch{dualHead: dualHead{Name: ref.Name, Tip: &tip}, Remote: name.IsRemoteBranch, Tracking: tracking})
	}
	return branches
}

func (v dualView) tags() []Tag {
	tags, err := v.repo.Tags(v.refs)
	if err != nil {
		v.t.Fatal(err)
	}
	return tags
}

func (v dualView) changes() int {
	status, err := workingTreeStatus(v.layout, v.repo.git)
	if err != nil {
		v.t.Fatal(err)
	}
	// Count paths, so a path staged and changed again still counts once.
	changes := 0
	for _, state := range status {
		if state.Staging != ' ' || state.Worktree != ' ' {
			changes++
		}
	}
	return changes
}

func (v dualView) walk(options WalkOptions) []string {
	ids, err := v.repo.Walk(options)
	if err != nil {
		v.t.Fatal(err)
	}
	return ids
}

func (v dualView) all() []string {
	seen := map[string]bool{}
	for _, branch := range v.branches() {
		for _, id := range v.walk(WalkOptions{Include: *branch.Tip, Sort: Time}) {
			seen[id] = true
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (v dualView) log(full bool) []map[string]any {
	var result []map[string]any
	for _, id := range v.walk(WalkOptions{Include: *v.head.Tip, Sort: Time}) {
		commit := mustLoadCommit(v.t, v.repo, plumbing.NewHash(id))
		parents := make([]string, 0, len(commit.ParentHashes))
		for _, parent := range commit.ParentHashes {
			parents = append(parents, parent.String())
		}
		entry := map[string]any{"id": id, "merge": len(parents) > 1, "parents": parents}
		if full {
			_, offset := commit.Committer.When.Zone()
			entry["when"], entry["offset"], entry["message"] = commit.Committer.When.Unix(), offset/60, commit.Message
		}
		result = append(result, entry)
	}
	return result
}

func (v dualView) queries() map[string]any {
	all := v.all()
	var queries []map[string]any
	for _, sort := range []Sort{0, Time, Topological, Topological | Time, Time | Reverse, Topological | Time | Reverse} {
		for _, first := range []bool{false, true} {
			for _, exclude := range append([]string{""}, all[:3]...) {
				var value any
				if exclude != "" {
					value = exclude
				}
				ids := v.walk(WalkOptions{Include: *v.head.Tip, Exclude: exclude, Sort: sort, FirstParent: first})
				queries = append(queries, map[string]any{"sort": sort, "first": first, "exclude": value, "ids": ids})
			}
		}
	}
	return map[string]any{"ids": all, "queries": queries}
}

func (v dualView) bases() []map[string]any {
	all := v.all()
	var result []map[string]any
	for _, a := range all {
		for _, b := range all {
			base, err := v.repo.MergeBase(a, b)
			if err != nil {
				v.t.Fatal(err)
			}
			result = append(result, map[string]any{"a": a, "b": b, "result": base})
		}
	}
	return result
}

func (v dualView) diffs() map[string][]string {
	result := map[string][]string{}
	for _, id := range v.all() {
		c, err := v.repo.Commit(id)
		if err != nil {
			v.t.Fatal(err)
		}
		result[id] = c.DiffPaths
	}
	return result
}

// The generic parity comparator expects a JSON object at the root. The
// original assertions can also return a single count or an ordered array.
func compareDualBackend(expected, actual []byte) ([]parity.Difference, error) {
	wrap := func(value []byte) []byte {
		return append(append([]byte(`{"value":`), value...), '}')
	}
	return parity.Compare(wrap(expected), wrap(actual))
}

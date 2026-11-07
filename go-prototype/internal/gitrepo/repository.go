// Package gitrepo provides read-only Git observations through go-git. Version
// policy belongs outside this package. A Repository is a single-use snapshot;
// callers must not mutate the underlying repository or share it across goroutines.
package gitrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf16"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type Repository struct {
	git     *git.Repository
	dir     string
	head    string
	branch  string
	commits map[string]*object.Commit
}

// Head identifies the attached local branch and its commit in this snapshot.
func (r *Repository) Head() (branch, sha string) { return r.branch, r.head }

// Commit returns value data for a commit, including first-parent changed paths.
func (r *Repository) Commit(id string) (Commit, error) { return r.commitInfo(id) }

// CommitMetadata reads parents, message and time without diffing trees. Version
// calculation uses this view; callers needing changed paths use Commit instead.
func (r *Repository) CommitMetadata(id string) (Commit, error) {
	c, err := r.load(id)
	if err != nil {
		return Commit{}, err
	}
	out := Commit{Parents: []string{}, Message: c.Message, When: c.Committer.When.Unix()}
	for _, p := range c.ParentHashes {
		out.Parents = append(out.Parents, p.String())
	}
	return out, nil
}

type Reference struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	SHA    string `json:"sha"`
}
type Tag struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	Commit string `json:"commit"`
}
type Commit struct {
	Parents   []string `json:"parents"`
	Message   string   `json:"message"`
	When      int64    `json:"when"`
	DiffPaths []string `json:"diff_paths"`
}
type Observation struct {
	Schema      int                 `json:"schema"`
	Head        string              `json:"head"`
	Branch      string              `json:"branch"`
	References  []Reference         `json:"references"`
	Tags        []Tag               `json:"tags"`
	Commits     map[string]Commit   `json:"commits"`
	Walks       map[string][]string `json:"walks"`
	MergeBases  map[string]*string  `json:"merge_bases"`
	Uncommitted int                 `json:"uncommitted"`
}

// Open deliberately accepts only complete, clean, non-bare repositories with
// attached local HEAD. Unsupported states must not yield plausible observations.
func Open(path string) (*Repository, error) {
	dir := filepath.Join(path, ".git")
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("unsupported repository: require a normal non-bare .git directory")
	}
	g, err := openCommonConfigRepository(path)
	if err != nil {
		return nil, err
	}
	shallow, err := g.Storer.Shallow()
	if err != nil {
		return nil, err
	}
	if len(shallow) > 0 {
		return nil, fmt.Errorf("unsupported shallow repository")
	}
	head, err := g.Reference(plumbing.HEAD, false)
	if err != nil {
		return nil, err
	}
	if head.Type() != plumbing.SymbolicReference || !head.Target().IsBranch() {
		return nil, fmt.Errorf("unsupported detached HEAD")
	}
	tip, err := g.Head()
	if err != nil {
		return nil, fmt.Errorf("unsupported empty or broken HEAD: %w", err)
	}
	wt, err := g.Worktree()
	if err != nil {
		return nil, err
	}
	status, err := wt.Status()
	if err != nil {
		return nil, err
	}
	if err := reconcileUntracked(path, dir, g, status); err != nil {
		return nil, err
	}
	if !status.IsClean() {
		return nil, fmt.Errorf("unsupported dirty repository")
	}
	r := &Repository{git: g, dir: dir, head: tip.Hash().String(), branch: head.Target().String(), commits: map[string]*object.Commit{}}
	if _, err = r.load(r.head); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Repository) load(id string) (*object.Commit, error) {
	if c, ok := r.commits[id]; ok {
		return c, nil
	}
	if !plumbing.IsHash(id) {
		return nil, fmt.Errorf("invalid commit id %q", id)
	}
	c, err := r.git.CommitObject(plumbing.NewHash(id))
	if err != nil {
		return nil, fmt.Errorf("commit %s: %w", id, err)
	}
	r.commits[id] = c
	return c, nil
}

func (r *Repository) References() ([]Reference, error) {
	refs := []Reference{}
	it, err := r.git.References()
	if err != nil {
		return nil, err
	}
	defer it.Close()
	err = it.ForEach(func(ref *plumbing.Reference) error {
		if !strings.HasPrefix(ref.Name().String(), "refs/") {
			return nil
		}
		if !ref.Name().IsBranch() && !ref.Name().IsTag() && !ref.Name().IsRemote() {
			// The reference branch/tag services do not consume private ref namespaces.
			return nil
		}

		resolved, err := r.git.Reference(ref.Name(), true)
		if err != nil {
			return err
		}
		target := ref.Hash().String()
		if ref.Type() == plumbing.SymbolicReference {
			target = ref.Target().String()
		}
		refs = append(refs, Reference{ref.Name().String(), target, resolved.Hash().String()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	// The reference backend enumerates loose refs first, then unshadowed packed refs.
	loose := map[string]bool{}
	for _, ref := range refs {
		_, err := os.Stat(filepath.Join(r.dir, filepath.FromSlash(ref.Name)))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		loose[ref.Name] = err == nil
	}
	sort.Slice(refs, func(i, j int) bool {
		if loose[refs[i].Name] != loose[refs[j].Name] {
			return loose[refs[i].Name]
		}
		// StringComparer.Ordinal compares UTF-16 code units, not UTF-8 bytes.
		return slices.Compare(utf16.Encode([]rune(refs[i].Name)), utf16.Encode([]rune(refs[j].Name))) < 0
	})
	return refs, nil
}

func (r *Repository) Tags(refs []Reference) ([]Tag, error) {
	tags := []Tag{}
	for _, ref := range refs {
		if !strings.HasPrefix(ref.Name, "refs/tags/") {
			continue
		}
		id := ref.SHA
		target := id
		seen := map[string]bool{}
		for {
			if seen[id] {
				return nil, fmt.Errorf("tag cycle at %s", id)
			}
			seen[id] = true
			o, err := r.git.Object(plumbing.AnyObject, plumbing.NewHash(id))
			if err != nil {
				return nil, err
			}
			switch v := o.(type) {
			case *object.Commit:
				tags = append(tags, Tag{ref.Name, target, id})
				goto peeled
			case *object.Tag:
				if id == ref.SHA {
					target = v.Target.String()
				}
				id = v.Target.String()
			default:
				return nil, fmt.Errorf("unsupported non-commit tag %s", ref.Name)
			}
		}
	peeled:
	}
	return tags, nil
}

func (r *Repository) commitInfo(id string) (Commit, error) {
	c, err := r.load(id)
	if err != nil {
		return Commit{}, err
	}
	out := Commit{Parents: []string{}, Message: c.Message, When: c.Committer.When.Unix(), DiffPaths: []string{}}
	for _, p := range c.ParentHashes {
		out.Parents = append(out.Parents, p.String())
	}
	tree, err := c.Tree()
	if err != nil {
		return out, err
	}
	var parent *object.Tree
	if len(c.ParentHashes) > 0 {
		p, err := r.load(c.ParentHashes[0].String())
		if err != nil {
			return out, err
		}
		parent, err = p.Tree()
		if err != nil {
			return out, err
		}
	}
	changes, err := object.DiffTree(parent, tree)
	if err != nil {
		return out, err
	}
	paths := map[string]bool{}
	for _, change := range changes {
		if change.From.Name != "" {
			paths[change.From.Name] = true
		}
		if change.To.Name != "" {
			paths[change.To.Name] = true
		}
	}
	for p := range paths {
		out.DiffPaths = append(out.DiffPaths, p)
	}
	sort.Strings(out.DiffPaths)
	return out, nil
}

// Observe exercises the contract used by Milestone B, including every supported
// sort combination, first-parent walks and each branch as an exclusion root.
func (r *Repository) Observe() (Observation, error) {
	out := Observation{Schema: 1, Head: r.head, Branch: r.branch, Commits: map[string]Commit{}, Walks: map[string][]string{}, MergeBases: map[string]*string{}}
	refs, err := r.References()
	if err != nil {
		return out, err
	}
	out.References = refs
	out.Tags, err = r.Tags(refs)
	if err != nil {
		return out, err
	}
	tips := []string{r.head}
	branches := []Reference{}
	for _, ref := range refs {
		if strings.HasPrefix(ref.Name, "refs/heads/") {
			branches = append(branches, ref)
			tips = append(tips, ref.SHA)
		}
	}
	for _, tag := range out.Tags {
		tips = append(tips, tag.Commit)
	}
	sort.Strings(tips)
	tips = slices.Compact(tips)
	for _, tip := range tips {
		ids, err := r.Walk(WalkOptions{Include: tip, Sort: Time})
		if err != nil {
			return out, err
		}
		for _, id := range ids {
			if _, ok := out.Commits[id]; !ok {
				info, err := r.commitInfo(id)
				if err != nil {
					return out, err
				}
				out.Commits[id] = info
			}
		}
	}
	for sortBy := Sort(0); sortBy < 8; sortBy++ {
		for _, first := range []bool{false, true} {
			key := fmt.Sprintf("head/%d/%t", sortBy, first)
			ids, err := r.Walk(WalkOptions{Include: r.head, Sort: sortBy, FirstParent: first})
			if err != nil {
				return out, err
			}
			out.Walks[key] = ids
			for _, b := range branches {
				key := fmt.Sprintf("exclude/%s/%d/%t", b.Name, sortBy, first)
				ids, err := r.Walk(WalkOptions{Include: r.head, Exclude: b.SHA, Sort: sortBy, FirstParent: first})
				if err != nil {
					return out, err
				}
				out.Walks[key] = ids
			}
		}
	}
	for _, b := range branches {
		ids, err := r.Walk(WalkOptions{Include: b.SHA, Sort: Time})
		if err != nil {
			return out, err
		}
		out.Walks["branch/"+b.Name] = ids
	}
	for _, a := range tips {
		for _, b := range tips {
			base, err := r.MergeBase(a, b)
			if err != nil {
				return out, err
			}
			out.MergeBases[a+"/"+b] = base
		}
	}
	return out, nil
}

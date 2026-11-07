// Package gitrepo provides read-only Git observations through go-git. Version
// policy belongs outside this package. A Repository is a single-use snapshot;
// callers must not mutate the underlying repository or share it across goroutines.
package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type Repository struct {
	git                                 *git.Repository
	dir                                 string
	gitDir                              string
	dirty                               int
	contextual                          bool
	physicalBranch, normalizationRemote string
	fetchedRemote                       string
	excludedRemotes                     map[string]bool
	head                                string
	branch                              string
	commits                             map[string]*object.Commit
	shallow                             map[ObjectID]struct{}
}

// Head identifies the attached local branch and its commit in this snapshot.
func (r *Repository) Head() (branch, sha string) { return r.branch, r.head }

// GitDirectory is this checkout's metadata directory, including linked worktrees.
func (r *Repository) GitDirectory() string { return r.gitDir }

func (r *Repository) UncommittedChanges() int { return r.dirty }

func (r *Repository) ContextualHead() bool { return r.contextual }

func (r *Repository) NormalizationRemote() string { return r.normalizationRemote }

func (r *Repository) FetchedRemote() string { return r.fetchedRemote }

func (r *Repository) CheckedOutBranch() string { return r.physicalBranch }

// ShallowCommits returns the sorted shallow boundaries visible in this snapshot.
func (r *Repository) ShallowCommits() []string {
	ids := make([]string, 0, len(r.shallow))
	for id := range r.shallow {
		ids = append(ids, id.String())
	}
	slices.Sort(ids)
	return ids
}

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

// Open deliberately accepts only complete, clean working trees with
// attached local HEAD. Unsupported states must not yield plausible observations.
func Open(path string) (*Repository, error) {
	return OpenForCalculation(path, OpenOptions{})
}

type OpenOptions struct {
	AllowShallow, NormalizeDetached, AllowDirty bool
	BuildBranch, BuildTag                       string
	TargetBranch, ContextBranch                 string
	NormalizationRemote, FetchedRemote          string
	PreparationContext                          context.Context
	RemoteCredentials                           Credentials
	IgnoreBranch                                func(string) (bool, error)
	RetainRemote                                string
}

// OpenForCalculation opens a clean worktree using explicitly selected policies.
var ErrNoCommits = errors.New("No commits found on the current branch.")

func OpenForCalculation(path string, options OpenOptions) (*Repository, error) {
	layout, err := openLayout(path)
	if err != nil {
		return nil, err
	}
	if layout.workingDirectory == "" {
		return nil, fmt.Errorf("unsupported bare repository")
	}
	shallow, err := layout.readShallowCommits()
	if err != nil {
		return nil, err
	}
	if len(shallow) > 0 && !options.AllowShallow {
		return nil, errors.New("Repository is a shallow clone. Git repositories must contain the full history. See https://gitversion.net/docs/reference/requirements#unshallow for more info.")
	}
	g, err := layout.open()
	if err != nil {
		return nil, err
	}
	store := referenceStore{git: g, commonDirectory: layout.commonDirectory}
	head, err := store.lookup(plumbing.HEAD)
	if err != nil {
		return nil, err
	}
	detached := head.Type() != plumbing.SymbolicReference || !head.Target().IsBranch()
	if detached && !options.NormalizeDetached && options.TargetBranch == "" && options.ContextBranch == "" {
		return nil, fmt.Errorf("unsupported detached HEAD")
	}
	tip, err := store.resolve(plumbing.HEAD)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil, ErrNoCommits
		}
		return nil, fmt.Errorf("unsupported empty or broken HEAD: %w", err)
	}
	status, err := workingTreeStatus(layout, g)
	if err != nil {
		return nil, err
	}
	if !options.AllowDirty && !status.IsClean() {
		return nil, fmt.Errorf("unsupported dirty repository")
	}
	r := newCommitRepository(layout, g, shallow)
	for _, change := range status {
		if change.Staging != git.Unmodified || change.Worktree != git.Unmodified {
			r.dirty++
		}
	}
	r.head = tip.Hash().String()
	r.branch = head.Target().String()
	r.physicalBranch, r.normalizationRemote, r.fetchedRemote = r.branch, options.NormalizationRemote, options.FetchedRemote
	if options.RetainRemote != "" {
		remotes, err := g.Remotes()
		if err != nil {
			return nil, err
		}
		r.excludedRemotes = make(map[string]bool, len(remotes))
		for _, remote := range remotes {
			name := remote.Config().Name
			if name != options.RetainRemote {
				r.excludedRemotes[name] = true
			}
		}
	}
	switch {
	case options.ContextBranch != "":
		r.branch, r.contextual = options.ContextBranch, true
	default:
		var selected bool
		if options.TargetBranch != "" {
			selected, err = r.selectTargetBranch(options.TargetBranch)
			if err != nil {
				return nil, err
			}
		}
		if !selected && options.TargetBranch == "" && options.NormalizationRemote != "" && strings.TrimSpace(options.BuildBranch) != "" {
			if err := r.applyProviderBranch(options.BuildBranch, options.NormalizationRemote); err != nil {
				return nil, err
			}
			selected = true
		}
		if detached && !selected {
			if !options.NormalizeDetached {
				return nil, fmt.Errorf("unsupported detached HEAD")
			}
			r.branch, err = r.detachedBranch(layout.workingDirectory, options)
			if err != nil {
				return nil, err
			}
		}
	}
	if _, err = r.load(r.head); err != nil {
		return nil, err
	}
	return r, nil
}

// CI providers name the build branch at the checkout tip. An unqualified
// provider branch may resolve to its tracking ref, but normalization cannot
// move the checkout to another commit.
func (r *Repository) applyProviderBranch(branch, remote string) error {
	name := strings.TrimPrefix(branch, "refs/heads/")
	if name == branch {
		name = strings.TrimPrefix(branch, "refs/")
	}
	local := plumbing.NewBranchReferenceName(name)
	if err := local.Validate(); err != nil {
		return fmt.Errorf("invalid build branch %q", branch)
	}
	ref, err := r.git.Reference(plumbing.NewRemoteReferenceName(remote, branch), true)
	if err != nil && !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return err
	}
	if err == nil && ref.Hash().String() != r.head {
		return fmt.Errorf("GitVersion has a bug, your HEAD has moved after repo normalisation after step 'EnsureLocalBranchExistsForCurrentBranch'")
	}
	r.branch = local.String()
	return nil
}

// A named target uses the actual branch tip. Prefer local references over
// tracking references and keep the checked-out branch when no name matches.
func (r *Repository) selectTargetBranch(target string) (bool, error) {
	if r.branch != "" && NewReferenceName(r.branch).EquivalentTo(&target) {
		return true, nil
	}
	refs, err := r.References()
	if err != nil {
		return false, err
	}
	var selected *Reference
	for _, ref := range refs {
		name := NewReferenceName(ref.Name)
		if (!name.IsLocalBranch && !name.IsRemoteBranch) || !name.EquivalentTo(&target) {
			continue
		}
		if name.IsLocalBranch {
			r.branch, r.head = ref.Name, ref.SHA
			return true, nil
		}
		if selected == nil {
			selected = &ref
		}
	}
	if selected != nil {
		r.branch, r.head = selected.Name, selected.SHA
		prefix := "refs/remotes/" + r.normalizationRemote + "/"
		if name, ok := strings.CutPrefix(r.branch, prefix); r.normalizationRemote != "" && ok {
			r.branch = "refs/heads/" + name
		}
	}
	return selected != nil, nil
}

// newCommitRepository applies storage-level shallow boundaries to every history
// consumer. Open separately checks whether a repository can be used to calculate.
func newCommitRepository(layout repositoryLayout, g *git.Repository, shallow []ObjectID) *Repository {
	r := &Repository{git: g, dir: layout.commonDirectory, gitDir: layout.gitDirectory, commits: map[string]*object.Commit{}}
	if len(shallow) > 0 {
		r.shallow = make(map[ObjectID]struct{}, len(shallow))
		for _, id := range shallow {
			r.shallow[id] = struct{}{}
		}
	}
	return r
}

func (r *Repository) load(id string) (*object.Commit, error) {
	if c, ok := r.commits[id]; ok {
		return c, nil
	}
	parsed, err := parseObjectID(id)
	if err != nil || parsed.length != sha1Size {
		return nil, fmt.Errorf("invalid commit id %q", id)
	}
	c, err := r.git.CommitObject(plumbing.Hash(parsed.data[:sha1Size]))
	if err != nil {
		return nil, fmt.Errorf("commit %s: %w", id, err)
	}
	c.Message = decodeCommitText(c.Message, string(c.Encoding))
	c.Author.Name = decodeCommitText(c.Author.Name, "")
	c.Author.Email = decodeCommitText(c.Author.Email, "")
	c.Committer.Name = decodeCommitText(c.Committer.Name, "")
	c.Committer.Email = decodeCommitText(c.Committer.Email, "")
	if _, boundary := r.shallow[parsed]; boundary {
		c.ParentHashes = nil
	}
	r.commits[id] = c
	return c, nil
}

func (r *Repository) References() ([]Reference, error) {
	refs := []Reference{}
	store := referenceStore{git: r.git, commonDirectory: r.dir}
	entries, err := store.list("refs/")
	if err != nil {
		return nil, err
	}
	for _, ref := range entries {
		if path, ok := strings.CutPrefix(ref.Name().String(), "refs/remotes/"); ok {
			remote, _, _ := strings.Cut(path, "/")
			if r.excludedRemotes[remote] {
				continue
			}
		}
		if !ref.Name().IsBranch() && !ref.Name().IsTag() && !ref.Name().IsRemote() {
			// The version calculator uses local and remote branches and tags.
			continue
		}
		resolved, err := store.resolve(ref.Name())
		if err != nil {
			return nil, err
		}
		target := ref.Hash().String()
		if ref.Type() == plumbing.SymbolicReference {
			target = ref.Target().String()
		}
		refs = append(refs, Reference{ref.Name().String(), target, resolved.Hash().String()})
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
	sort.SliceStable(refs, func(i, j int) bool { return loose[refs[i].Name] && !loose[refs[j].Name] })
	return refs, nil
}

func (r *Repository) Tags(refs []Reference) ([]Tag, error) {
	tags := []Tag{}
	for _, ref := range refs {
		if !NewReferenceName(ref.Name).IsTag {
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
	out.DiffPaths, err = changedTreePaths(parent, tree)
	return out, err
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

// Package fixtures creates deterministic repositories using go-git exclusively.
package fixtures

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

type Step struct {
	Op        string `json:"op"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	From      string `json:"from,omitempty"`
	Message   string `json:"message,omitempty"`
	Annotated bool   `json:"annotated,omitempty"`
	SemVer    string `json:"semver,omitempty"`
	Seconds   *int64 `json:"seconds,omitempty"`
}
type Recipe struct {
	InitialBranch string `json:"initial_branch,omitempty"`
	ID            string `json:"id"`
	Description   string `json:"description"`
	Config        string `json:"config"`
	Steps         []Step `json:"steps"`
}
type Checkpoint struct {
	ID     string `json:"id"`
	Recipe string `json:"recipe"`
	Head   string `json:"head"`
	Branch string `json:"branch"`
	Digest string `json:"digest"`
	SemVer string `json:"semver,omitempty"`
	Path   string `json:"-"`
}

var safeID = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func Load(path string) ([]Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var recipes []Recipe
	if err = json.Unmarshal(data, &recipes); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, r := range recipes {
		if !safeID.MatchString(r.ID) || seen[r.ID] {
			return nil, fmt.Errorf("invalid or duplicate recipe ID %q", r.ID)
		}
		seen[r.ID] = true
	}
	if len(recipes) == 0 {
		return nil, fmt.Errorf("empty corpus")
	}
	return recipes, nil
}

// Generate refuses existing destinations, so capturing cannot overwrite reviewed data.
func Generate(recipes []Recipe, dest string) ([]Checkpoint, error) {
	if err := os.Mkdir(dest, 0755); err != nil {
		return nil, err
	}
	var out []Checkpoint
	for _, recipe := range recipes {
		c, err := build(recipe, dest)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", recipe.ID, err)
		}
		out = append(out, c...)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no checkpoints")
	}
	return out, nil
}

func build(recipe Recipe, dest string) ([]Checkpoint, error) {
	if !safeID.MatchString(recipe.ID) {
		return nil, fmt.Errorf("invalid recipe ID")
	}
	source, err := os.MkdirTemp(dest, ".source-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(source)
	branch := recipe.InitialBranch
	if branch == "" {
		branch = "main"
	}
	if err := plumbing.NewBranchReferenceName(branch).Validate(); err != nil {
		return nil, err
	}
	repo, err := git.PlainInitWithOptions(source, &git.PlainInitOptions{InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName(branch)}})
	if err != nil {
		return nil, err
	}
	heads := map[string]plumbing.Hash{}
	trees := map[plumbing.Hash]map[string]string{}
	ids := map[string]bool{}
	seen := map[string]bool{}
	out := []Checkpoint{}
	ordinal := 0
	for _, s := range recipe.Steps {
		switch s.Op {
		case "commit", "merge":
			if !safeID.MatchString(s.ID) || ids[s.ID] {
				return nil, fmt.Errorf("invalid or duplicate commit %q", s.ID)
			}
			ids[s.ID] = true
			files := map[string]string{"GitVersion.yml": recipe.Config}
			parents := []plumbing.Hash{}
			if h, ok := heads[branch]; ok {
				parents = append(parents, h)
				for k, v := range trees[h] {
					files[k] = v
				}
			}
			if s.Op == "merge" {
				other, ok := heads[s.From]
				if !ok || len(parents) == 0 || other == parents[0] {
					return nil, fmt.Errorf("invalid merge from %q", s.From)
				}
				parents = append(parents, other)
				for k, v := range trees[other] {
					if old, exists := files[k]; exists && old != v {
						return nil, fmt.Errorf("conflicting merge file %s", k)
					}
					files[k] = v
				}
			}
			files[s.ID+".txt"] = s.ID + "\n"
			tree := &object.Tree{}
			names := []string{}
			for k := range files {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, name := range names {
				blob := repo.Storer.NewEncodedObject()
				blob.SetType(plumbing.BlobObject)
				w, err := blob.Writer()
				if err != nil {
					return nil, err
				}
				_, err = io.WriteString(w, files[name])
				closeErr := w.Close()
				if err != nil {
					return nil, err
				}
				if closeErr != nil {
					return nil, closeErr
				}
				h, err := repo.Storer.SetEncodedObject(blob)
				if err != nil {
					return nil, err
				}
				tree.Entries = append(tree.Entries, object.TreeEntry{Name: name, Mode: filemode.Regular, Hash: h})
			}
			encoded := repo.Storer.NewEncodedObject()
			if err = tree.Encode(encoded); err != nil {
				return nil, err
			}
			th, err := repo.Storer.SetEncodedObject(encoded)
			if err != nil {
				return nil, err
			}
			sig := object.Signature{Name: "Parity Fixture", Email: "parity@example.invalid", When: time.Date(2025, 1, 1, 0, ordinal, 0, 0, time.UTC)}
			if s.Seconds != nil {
				sig.When = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(*s.Seconds) * time.Second)
			}
			ordinal++
			message := s.Message
			if message == "" {
				message = s.ID
			}
			commit := &object.Commit{Author: sig, Committer: sig, Message: message + "\n", TreeHash: th, ParentHashes: parents}
			encoded = repo.Storer.NewEncodedObject()
			if err = commit.Encode(encoded); err != nil {
				return nil, err
			}
			h, err := repo.Storer.SetEncodedObject(encoded)
			if err != nil {
				return nil, err
			}
			heads[branch] = h
			trees[h] = files
			if err = repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName(branch), h)); err != nil {
				return nil, err
			}
		case "branch":
			ref := plumbing.NewBranchReferenceName(s.Name)
			if err = ref.Validate(); err != nil {
				return nil, err
			}
			if _, ok := heads[s.Name]; ok {
				return nil, fmt.Errorf("duplicate branch")
			}
			h, ok := heads[branch]
			if !ok {
				return nil, fmt.Errorf("branch without commit")
			}
			heads[s.Name] = h
			if err = repo.Storer.SetReference(plumbing.NewHashReference(ref, h)); err != nil {
				return nil, err
			}
		case "delete-branch":
			if _, ok := heads[s.Name]; !ok || s.Name == branch {
				return nil, fmt.Errorf("cannot delete missing or current branch %q", s.Name)
			}
			if err := repo.Storer.RemoveReference(plumbing.NewBranchReferenceName(s.Name)); err != nil {
				return nil, err
			}
			delete(heads, s.Name)
		case "fast-forward":
			from, ok := heads[s.From]
			if !ok {
				return nil, fmt.Errorf("unknown fast-forward source %q", s.From)
			}
			current, err := repo.CommitObject(heads[branch])
			if err != nil {
				return nil, err
			}
			target, err := repo.CommitObject(from)
			if err != nil {
				return nil, err
			}
			ancestor, err := current.IsAncestor(target)
			if err != nil || !ancestor {
				return nil, fmt.Errorf("invalid fast-forward from %q: %v", s.From, err)
			}
			heads[branch] = from
			if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName(branch), from)); err != nil {
				return nil, err
			}
		case "checkout":
			if _, ok := heads[s.Name]; !ok {
				return nil, fmt.Errorf("unknown branch %q", s.Name)
			}
			branch = s.Name
			if err = repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName(branch))); err != nil {
				return nil, err
			}
		case "tag":
			h, ok := heads[branch]
			if !ok {
				return nil, fmt.Errorf("tag without commit")
			}
			var options *git.CreateTagOptions
			if s.Annotated {
				options = &git.CreateTagOptions{Tagger: &object.Signature{Name: "Parity Fixture", Email: "parity@example.invalid", When: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)}, Message: s.Name + "\n"}
			}
			if _, err = repo.CreateTag(s.Name, h, options); err != nil {
				return nil, err
			}
		case "checkpoint":
			if !safeID.MatchString(s.ID) || seen[s.ID] {
				return nil, fmt.Errorf("invalid or duplicate checkpoint %q", s.ID)
			}
			seen[s.ID] = true
			if _, ok := heads[branch]; !ok {
				return nil, fmt.Errorf("checkpoint without commit")
			}
			wt, err := repo.Worktree()
			if err != nil {
				return nil, err
			}
			if err = wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(branch), Force: true}); err != nil {
				return nil, err
			}
			id := recipe.ID + "--" + s.ID
			path := filepath.Join(dest, id)
			if err = Copy(source, path); err != nil {
				return nil, err
			}
			digest, err := Digest(path)
			if err != nil {
				return nil, err
			}
			out = append(out, Checkpoint{ID: id, Recipe: recipe.ID, Head: heads[branch].String(), Branch: branch, Digest: digest, SemVer: s.SemVer, Path: path})
		default:
			return nil, fmt.Errorf("unknown operation %q", s.Op)
		}
	}
	return out, nil
}

// Copy preserves actual repository state, including index, refs and worktree.
// Symlinks and special files are refused; generated fixtures contain neither.
func Copy(source, dest string) error {
	if err := os.Mkdir(dest, 0755); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dest, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.Mkdir(target, 0755)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported fixture file %s", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

// Digest hashes relative file paths, executable bits and bytes, excluding only
// index stat-cache bytes (which embed filesystem times and inode numbers).
// Index semantics are checked separately by State, not ignored.
func Digest(root string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if filepath.ToSlash(rel) == ".git/index" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file %s", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00%d\x00", filepath.ToSlash(rel), info.Mode().Perm()&0111, len(data))
		_, err = h.Write(data)
		return err
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// State includes logical index content as well as all other repository bytes.
func State(root string) (string, error) {
	digest, err := Digest(root)
	if err != nil {
		return "", err
	}
	// Reading the logical index does not require opening a repository. Avoid
	// applying go-git extension validation to an independent mutation audit.
	st := filesystem.NewStorage(osfs.New(filepath.Join(root, ".git")), cache.NewObjectLRUDefault())
	idx, err := st.Index()
	if err != nil {
		return "", err
	}
	entries := []string{}
	for _, e := range idx.Entries {
		entries = append(entries, fmt.Sprintf("%s:%s:%o:%d:%t:%t", e.Name, e.Hash, e.Mode, e.Stage, e.SkipWorktree, e.IntentToAdd))
	}
	sort.Strings(entries)
	data, _ := json.Marshal(struct {
		Digest string
		Index  []string
	}{digest, entries})
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

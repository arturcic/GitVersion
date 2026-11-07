package gitrepo

import (
	"errors"
	"io"
	"sort"

	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func changedTreePaths(before, after *object.Tree) ([]string, error) {
	paths := []string{}
	if before != nil && after != nil && before.Hash == after.Hash {
		return paths, nil
	}
	if before == nil || after == nil {
		tree := after
		if tree == nil {
			tree = before
		}
		files, err := flattenTree(tree)
		if err != nil {
			return nil, err
		}
		for name := range files {
			paths = append(paths, name)
		}
		sort.Strings(paths)
		return paths, nil
	}
	changes, err := object.DiffTree(before, after)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, change := range changes {
		for _, name := range []string{change.From.Name, change.To.Name} {
			if name != "" {
				seen[name] = struct{}{}
			}
		}
	}
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func flattenTree(tree *object.Tree) (map[string]object.TreeEntry, error) {
	files := map[string]object.TreeEntry{}
	if tree == nil {
		return files, nil
	}
	walker := object.NewTreeWalker(tree, true, nil)
	defer walker.Close()
	for {
		name, entry, err := walker.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		if entry.Mode != filemode.Dir {
			files[name] = entry
		}
	}
}

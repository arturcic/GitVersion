package gitrepo

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf16"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type referenceStore struct {
	git             *git.Repository
	commonDirectory string
}

func (s referenceStore) lookup(name plumbing.ReferenceName) (*plumbing.Reference, error) {
	return s.git.Reference(name, false)
}

func (s referenceStore) resolve(name plumbing.ReferenceName) (*plumbing.Reference, error) {
	return s.git.Reference(name, true)
}

func ordinalReferenceLess(a, b string) bool {
	return slices.Compare(utf16.Encode([]rune(a)), utf16.Encode([]rune(b))) < 0
}

func (s referenceStore) list(prefix string) ([]*plumbing.Reference, error) {
	it, err := s.git.References()
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var refs []*plumbing.Reference
	err = it.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().String()
		if strings.HasPrefix(name, prefix) && !strings.HasSuffix(name, ".lock") {
			refs = append(refs, ref)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(refs, func(i, j int) bool { return ordinalReferenceLess(refs[i].Name().String(), refs[j].Name().String()) })
	return refs, nil
}

// packedTarget reads the optional fully peeled target that follows a packed
// entry. go-git resolves the reference itself and discards this metadata.
func (s referenceStore) packedTarget(name plumbing.ReferenceName) (*ObjectID, error) {
	if _, err := os.Stat(filepath.Join(s.commonDirectory, filepath.FromSlash(name.String()))); !os.IsNotExist(err) {
		return nil, err // A loose ref shadows both packed values.
	}
	file, err := os.Open(filepath.Join(s.commonDirectory, "packed-refs"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	match := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if match {
			if raw, ok := strings.CutPrefix(line, "^"); ok {
				id, err := parseObjectID(raw)
				if err != nil {
					return nil, err
				}
				return &id, nil
			}
			return nil, nil
		}
		_, ref, ok := strings.Cut(line, " ")
		match = ok && ref == name.String()
	}
	return nil, scanner.Err()
}

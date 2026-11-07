package gitrepo

import (
	"slices"
	"strings"
)

// NormalizedReferences projects the CI remote as local branches for version
// calculation. Existing checkout and target branches keep their selected tips.
func (repo *Repository) NormalizedReferences(refs []Reference, ignoredBranch func(string) (bool, error)) ([]Reference, error) {
	return repo.normalizedReferences(refs, repo.NormalizationRemote(), ignoredBranch)
}

func (repo *Repository) normalizedReferences(refs []Reference, remote string, ignoredBranch func(string) (bool, error)) ([]Reference, error) {
	if remote == "" {
		return refs, nil
	}
	current, tip := repo.Head()
	indices := make(map[string]int, len(refs))
	for i, ref := range refs {
		indices[ref.Name] = i
	}
	normalized := slices.Clone(refs)
	update := func(ref Reference) {
		if i, exists := indices[ref.Name]; exists {
			normalized[i] = ref
		} else {
			normalized = append(normalized, ref)
			indices[ref.Name] = len(normalized) - 1
		}
	}
	for _, ref := range refs {
		name, ok := strings.CutPrefix(ref.Name, "refs/remotes/"+remote+"/")
		if !ok || name == "HEAD" {
			continue
		}
		local := "refs/heads/" + name
		if ignoredBranch != nil {
			ignored, err := ignoredBranch(name)
			if err != nil {
				return nil, err
			}
			if ignored {
				continue
			}
		}
		if local == repo.CheckedOutBranch() || local == current {
			continue
		}
		update(Reference{Name: local, Target: ref.SHA, SHA: ref.SHA})
	}
	if strings.HasPrefix(current, "refs/heads/") && current != repo.CheckedOutBranch() {
		update(Reference{Name: current, Target: tip, SHA: tip})
	}
	return normalized, nil
}

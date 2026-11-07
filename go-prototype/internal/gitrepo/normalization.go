package gitrepo

import (
	"fmt"
	"slices"
	"strings"
)

// NormalizedReferences projects the CI remote as local branches for version
// calculation. Existing checkout and target branches keep their selected tips.
func (repo *Repository) NormalizedReferences(refs []Reference, ignoredBranch func(string) (bool, error)) ([]Reference, error) {
	return repo.TraceNormalizedReferences(refs, ignoredBranch, nil)
}

// TraceNormalizedReferences reports changes to the in-memory branch view.
func (repo *Repository) TraceNormalizedReferences(refs []Reference, ignoredBranch func(string) (bool, error), info func(string)) ([]Reference, error) {
	return repo.normalizedReferences(refs, repo.NormalizationRemote(), ignoredBranch, info)
}

func (repo *Repository) normalizedReferences(refs []Reference, remote string, ignoredBranch func(string) (bool, error), info func(string)) ([]Reference, error) {
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
		protected := local == repo.CheckedOutBranch() || local == current
		if ignoredBranch != nil {
			ignored, err := ignoredBranch(name)
			if err != nil {
				return nil, err
			}
			if ignored {
				if info != nil && !protected {
					info(fmt.Sprintf("Skipping ignored remote tracking branch '%s'.", ref.Name))
				}
				continue
			}
		}
		if protected {
			continue
		}
		if info != nil {
			if index, exists := indices[local]; exists {
				if normalized[index].Target == ref.Target {
					info(fmt.Sprintf("Skipping update of '%s' as it already matches the remote ref.", ref.Name))
				} else {
					info(fmt.Sprintf("Updating local ref '%s' to point at %.7s (calculation view).", local, ref.SHA))
				}
			} else {
				info(fmt.Sprintf("Creating local branch from remote tracking '%s' (calculation view).", ref.Name))
			}
		}
		update(Reference{Name: local, Target: ref.SHA, SHA: ref.SHA})
	}
	if strings.HasPrefix(current, "refs/heads/") && current != repo.CheckedOutBranch() {
		update(Reference{Name: current, Target: tip, SHA: tip})
	}
	return normalized, nil
}

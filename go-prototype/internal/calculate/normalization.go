package calculate

import (
	"slices"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
)

// normalizedReferences projects the CI remote as local branches for version
// calculation. Existing checkout and target branches keep their selected tips.
func normalizedReferences(repo *gitrepo.Repository, cfg config.Config, refs []gitrepo.Reference) ([]gitrepo.Reference, error) {
	remote := repo.NormalizationRemote()
	if remote == "" {
		return refs, nil
	}
	current, tip := repo.Head()
	indices := make(map[string]int, len(refs))
	for i, ref := range refs {
		indices[ref.Name] = i
	}
	normalized := slices.Clone(refs)
	for _, ref := range refs {
		name, ok := strings.CutPrefix(ref.Name, "refs/remotes/"+remote+"/")
		if !ok || name == "HEAD" {
			continue
		}
		local := "refs/heads/" + name
		ignored, err := cfg.Ignore.BranchIgnored(name)
		if err != nil {
			return nil, err
		}
		if ignored || local == repo.CheckedOutBranch() {
			continue
		}
		sha := ref.SHA
		if local == current {
			sha = tip
		}
		replacement := gitrepo.Reference{Name: local, Target: sha, SHA: sha}
		if i, exists := indices[local]; exists {
			normalized[i] = replacement
		} else {
			normalized = append(normalized, replacement)
		}
	}
	return normalized, nil
}

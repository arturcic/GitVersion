package calculate

import (
	"slices"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
)

type effectiveBranchConfiguration struct {
	branch  string
	value   config.Effective
	options config.Options
}

// The callback supplies direct source candidates, before inherited branch
// options are resolved. The shared traversal set also breaks source cycles.
type branchConfigurationFinder struct {
	config   config.Config
	branches []branchRef
	sources  func(string, map[string]bool) ([]string, error)
	commit   func(string) (message string, isMerge bool, err error)
}

func (f branchConfigurationFinder) configurations(branch, sha string) ([]effectiveBranchConfiguration, error) {
	visited := map[string]bool{}
	kind, _, err := f.config.Branch(branch)
	if err != nil {
		return nil, err
	}
	inherited := f.config.DeclaredOptions(kind).Increment == nil
	var walk func(string, *config.Options, bool) ([]effectiveBranchConfiguration, error)
	walk = func(current string, child *config.Options, resolveTarget bool) ([]effectiveBranchConfiguration, error) {
		if visited[current] {
			return nil, nil
		}
		visited[current] = true
		kind, _, err := f.config.Branch(current)
		if err != nil {
			return nil, err
		}
		options := f.config.DeclaredOptions(kind)
		if child != nil {
			options = child.Inherit(options)
		}
		emit := func() ([]effectiveBranchConfiguration, error) {
			projection := options
			projection.Label = ""
			value, err := f.config.EffectiveOptions(branch, sha, projection, inherited)
			value.Label = options.Label
			if err != nil {
				return nil, err
			}
			return []effectiveBranchConfiguration{{current, value, options}}, nil
		}
		if options.Increment != nil {
			return emit()
		}
		sources, err := f.sources(current, visited)
		if err != nil {
			return nil, err
		}
		if resolveTarget {
			target, err := f.pullRequestTarget(current)
			if err != nil {
				return nil, err
			}
			if target != "" {
				sources = []string{target}
			}
		}
		if len(sources) == 0 {
			if f.config.FallbackIncrement == nil {
				return nil, nil
			}
			options.Increment = f.config.FallbackIncrement
			return emit()
		}
		var out []effectiveBranchConfiguration
		for _, source := range sources {
			configs, err := walk(source, &options, false)
			if err != nil {
				return nil, err
			}
			out = append(out, configs...)
		}
		return out, nil
	}
	return walk(branch, nil, true)
}

func (f branchConfigurationFinder) pullRequestTarget(branch string) (string, error) {
	kind, _, err := f.config.Branch(branch)
	if err != nil {
		return "", err
	}
	isPullRequest := kind == "pull-request" || gitrepo.NewReferenceName(branch).IsPullRequest
	if !isPullRequest || f.commit == nil {
		return "", nil
	}
	message, merge, err := f.commit(branch)
	if err != nil {
		return "", err
	}
	parsed, err := ParseMergeMessage(&message, nil)
	if err != nil {
		return "", err
	}
	if !parsed.IsMergedPullRequest() || parsed.TargetBranch == nil || strings.TrimSpace(*parsed.TargetBranch) == "" {
		return "", nil
	}
	// C# also accepts a release-branch message on a single-parent commit,
	// which is how a squash merge can carry a pull-request target.
	if !merge {
		if parsed.MergedBranch == nil || *parsed.MergedBranch == "" {
			return "", nil
		}
		merged := *parsed.MergedBranch
		if parsed.FormatName != nil && *parsed.FormatName == "RemoteTracking" && !strings.HasPrefix(merged, "refs/remotes/") {
			merged = "refs/remotes/" + merged
		}
		mergedKind, _, err := f.config.Branch(merged)
		if err != nil {
			return "", err
		}
		if !truth(f.config.Options(mergedKind).IsRelease) {
			return "", nil
		}
	}
	var target string
	remote := false
	for _, candidate := range f.branches {
		if candidate.name == branch || !slices.Contains(f.config.SourceKinds(kind), candidate.kind) {
			continue
		}
		name := gitrepo.NewReferenceName(candidate.name)
		if !name.EquivalentTo(parsed.TargetBranch) {
			continue
		}
		isRemote := name.IsRemoteBranch
		if target == "" || remote && !isRemote {
			target, remote = candidate.name, isRemote
		}
	}
	return target, nil
}

package calculate

import (
	"fmt"
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"regexp"
	"strings"
)

type branchRef struct{ name, sha, kind string }

func configurationSource(r *gitrepo.Repository, cfg config.Config, branch string, refs []branchRef, visited map[string]bool) (string, error) {
	if visited[branch] {
		return "", fmt.Errorf("cyclic branch inheritance")
	}
	visited[branch] = true
	kind, _, err := cfg.Branch(branch)
	if err != nil {
		return "", err
	}
	if kind == "main" || cfg.Options(kind).Increment != nil {
		return branch, nil
	}
	var tip string
	for _, b := range refs {
		if b.name == branch {
			tip = b.sha
		}
	}
	allowed := cfg.SourceKinds(kind)
	selected := ""
	bestTime := int64(-1 << 63)
	for _, b := range refs {
		if visited[b.name] {
			continue
		}
		ok := false
		for _, k := range allowed {
			if b.kind == k {
				ok = true
			}
		}
		if !ok {
			continue
		}
		base, err := branchMergeBase(r, tip, b.sha)
		if err != nil {
			return "", err
		}
		if base == "" {
			continue
		}
		commit, err := r.Commit(base)
		if err != nil {
			return "", err
		}
		if selected == "" || commit.When > bestTime {
			selected = b.name
			bestTime = commit.When
		}
	}
	if selected == "" {
		return "", fmt.Errorf("no supported source branch for %s", branch)
	}
	return configurationSource(r, cfg, selected, refs, visited)
}

var branchVersion = regexp.MustCompile(`[vV]?([0-9]+)(?:\.([0-9]+))?(?:\.([0-9]+))?`)

func versionInBranch(name string) (semver.Version, bool) {
	m := branchVersion.FindStringSubmatch(name)
	if m == nil {
		return semver.Version{}, false
	}
	for i := 2; i <= 3; i++ {
		if m[i] == "" {
			m[i] = "0"
		}
	}
	v, err := semver.Parse(strings.Join(m[1:4], "."))
	return v, err == nil
}

var mergeBranchPattern = regexp.MustCompile(`(?i)Merge (?:remote-tracking )?branch ['"]([^'"]+)['"]`)
var pullMergePattern = regexp.MustCompile(`(?i)Merge pull request #[0-9]+ from (?:[^/]+/)?(.+)`)

func mergedBranch(message string) string {
	if m := mergeBranchPattern.FindStringSubmatch(message); m != nil {
		return strings.TrimPrefix(m[1], "origin/")
	}
	if m := pullMergePattern.FindStringSubmatch(message); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}
func workflowCandidates(r *gitrepo.Repository, cfg config.Config, e config.Effective, branch, source, head string, refs []branchRef, history []string, tags []tag, load func(string) (gitrepo.Commit, error)) ([]candidate, error) {
	var out []candidate
	if e.IsRelease {
		for _, name := range []string{branch, source} {
			if v, ok := versionInBranch(name); ok {
				bumped, err := v.Bump(semver.None, e.Label, false)
				if err != nil {
					return nil, err
				}
				out = append(out, candidate{base: v, semantic: v, version: bumped, description: "version in branch name"})
				break
			}
		}
	}
	if e.TrackMergeMessage {
		found := 0
		for _, id := range history {
			c, err := load(id)
			if err != nil {
				return nil, err
			}
			name := mergedBranch(c.Message)
			kind, _, err := cfg.Branch(name)
			if err != nil {
				continue
			}
			o := cfg.Options(kind)
			if o.IsRelease == nil || !*o.IsRelease {
				continue
			}
			v, ok := versionInBranch(name)
			if !ok {
				continue
			}
			inc := semver.None
			if !e.PreventMerged {
				base := id
				if len(c.Parents) == 2 {
					base, err = mergeBase(r, c.Parents[0], c.Parents[1])
					if err != nil {
						return nil, err
					}
				}
				inc, err = increment(r, head, base, e.Label, e.Increment, e.Messages, tags, load)
				if err != nil {
					return nil, err
				}
			}
			bumped, err := v.Bump(inc, e.Label, false)
			if err != nil {
				return nil, err
			}
			out = append(out, candidate{base: v, semantic: v, semanticSHA: id, version: bumped, inc: inc, description: "merge message"})
			found++
			if found == 5 {
				break
			}
		}
	}
	if e.TracksRelease {
		for _, b := range refs {
			o := cfg.Options(b.kind)
			if o.IsRelease == nil || !*o.IsRelease {
				continue
			}
			v, ok := versionInBranch(b.name)
			semanticSHA := ""
			if !ok {
				var err error
				v, semanticSHA, ok, err = releaseTagVersion(r, cfg, b, refs, tags)
				if err != nil {
					return nil, err
				}
				if !ok {
					continue
				}
			}
			base, err := branchMergeBase(r, b.sha, head)
			if err != nil {
				return nil, err
			}
			if base == "" {
				continue
			}
			inc, err := increment(r, head, base, e.Label, e.Increment, e.Messages, tags, load)
			if err != nil {
				return nil, err
			}
			bumped, err := v.Bump(inc, e.Label, false)
			if err != nil {
				return nil, err
			}
			out = append(out, candidate{base: v, semantic: v, semanticSHA: semanticSHA, version: bumped, sha: base, inc: inc, description: "release branch exists"})
		}
	}
	return out, nil
}

func mergeBase(r *gitrepo.Repository, a, b string) (string, error) {
	v, err := r.MergeBase(a, b)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", nil
	}
	return *v, nil
}

func candidateOrder(c candidate) int {
	switch c.description {
	case "configured next-version":
		return 0
	case "mainline":
		return 1
	case "merge message":
		return 2
	case "release branch exists":
		return 4
	case "version in branch name":
		return 5
	case "fallback":
		return 6
	}
	return 3
}

// Match the pinned branch-aware merge-base finder, excluding forward merges.
func branchMergeBase(r *gitrepo.Repository, tip, other string) (string, error) {
	c, err := r.Commit(other)
	if err != nil {
		return "", err
	}
	for _, p := range c.Parents {
		if p == tip {
			other = c.Parents[0]
			break
		}
	}
	base, err := mergeBase(r, tip, other)
	if err != nil || base == "" {
		return base, err
	}
	for {
		ids, err := r.Walk(gitrepo.WalkOptions{Include: other, Exclude: base})
		if err != nil {
			return "", err
		}
		forward := ""
		for _, id := range ids {
			c, err := r.Commit(id)
			if err != nil {
				return "", err
			}
			for _, p := range c.Parents {
				if p == base {
					forward = c.Parents[0]
					break
				}
			}
			if forward != "" {
				break
			}
		}
		if forward == "" {
			break
		}
		next, err := mergeBase(r, tip, forward)
		if err != nil {
			return "", err
		}
		if next == base {
			break
		}
		base, other = next, forward
		if base == "" {
			break
		}
	}
	return base, nil
}

// An unversioned release name can take its target from a matching prerelease
// tag on its own first-parent history, excluding every main branch's trunk.
func releaseTagVersion(r *gitrepo.Repository, cfg config.Config, branch branchRef, refs []branchRef, tags []tag) (semver.Version, string, bool, error) {
	excluded := map[string]bool{}
	for _, b := range refs {
		if !truth(cfg.Options(b.kind).IsMain) {
			continue
		}
		ids, err := r.Walk(gitrepo.WalkOptions{Include: b.sha, FirstParent: true})
		if err != nil {
			return semver.Version{}, "", false, err
		}
		for _, id := range ids {
			excluded[id] = true
		}
	}
	ids, err := r.Walk(gitrepo.WalkOptions{Include: branch.sha, FirstParent: true})
	if err != nil {
		return semver.Version{}, "", false, err
	}
	own := map[string]bool{}
	for _, id := range ids {
		if !excluded[id] {
			own[id] = true
		}
	}
	effective, err := cfg.Effective(branch.name)
	if err != nil {
		return semver.Version{}, "", false, err
	}
	var selected *tag
	for i := range tags {
		t := &tags[i]
		if own[t.ref.Commit] && t.v.IsPreRelease() && t.v.Matches(effective.Label) && (selected == nil || t.v.Compare(selected.v) > 0) {
			selected = t
		}
	}
	if selected == nil {
		return semver.Version{}, "", false, nil
	}
	v := selected.v
	v.Label = ""
	v.Number = nil
	return v, selected.ref.Commit, true, nil
}

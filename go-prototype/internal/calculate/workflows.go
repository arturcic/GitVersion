package calculate

import (
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"regexp"
	"sort"
	"strings"
)

type branchRef struct{ name, sha, kind string }

// configurationOrigins follows the pinned direct source finder: descending
// merge-base time, discard older contained origins, and prefer exact branch tips.
func configurationOrigins(r *gitrepo.Repository, cfg config.Config, branch string, refs []branchRef, visited map[string]bool) ([]string, error) {
	kind, _, err := cfg.Branch(branch)
	if err != nil {
		return nil, err
	}
	var tip string
	for _, b := range refs {
		if b.name == branch {
			tip = b.sha
		}
	}
	type origin struct {
		branch  branchRef
		base    string
		when    int64
		ignored bool
	}
	var origins []origin
	history, err := r.Walk(gitrepo.WalkOptions{Include: tip})
	if err != nil {
		return nil, err
	}
	for _, b := range refs {
		if visited[b.name] {
			continue
		}
		allowed := false
		for _, k := range cfg.SourceKinds(kind) {
			if b.kind == k {
				allowed = true
			}
		}
		if !allowed {
			continue
		}
		base, err := branchMergeBase(r, tip, b.sha)
		if err != nil {
			return nil, err
		}
		if base == "" {
			continue
		}
		c, err := r.CommitMetadata(base)
		if err != nil {
			return nil, err
		}
		o := origin{branch: b, base: base, when: c.When}
		for _, id := range history {
			v, err := r.CommitMetadata(id)
			if err != nil {
				return nil, err
			}
			if v.When > o.when && len(v.Parents) > 1 {
				for _, p := range v.Parents {
					if p == base {
						o.ignored = true
					}
				}
			}
		}
		origins = append(origins, o)
	}
	sort.SliceStable(origins, func(i, j int) bool { return origins[i].when > origins[j].when })
	for i := len(origins) - 1; i > 0; i-- {
		if origins[i].ignored {
			continue
		}
		for j := 0; j < i; j++ {
			if origins[i].base == origins[j].base {
				break
			}
			ids, err := r.Walk(gitrepo.WalkOptions{Include: origins[j].branch.sha})
			if err != nil {
				return nil, err
			}
			found := false
			for _, id := range ids {
				if id == origins[i].base {
					found = true
					break
				}
			}
			if found {
				origins = append(origins[:i], origins[i+1:]...)
				break
			}
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, o := range origins {
		exact := false
		for _, other := range origins {
			if other.base == o.base && other.branch.sha == o.base {
				exact = true
			}
		}
		if exact && o.branch.sha != o.base {
			continue
		}
		if !seen[o.branch.name] {
			out = append(out, o.branch.name)
			seen[o.branch.name] = true
		}
	}
	return out, nil
}

var branchVersion = regexp.MustCompile(`^[vV]?([0-9]+)(?:\.([0-9]+))?(?:\.([0-9]+))?`)

func versionInBranch(name string) (semver.Version, bool) {
	separator := "/"
	if !strings.Contains(name, "/") && strings.Contains(name, "-") {
		separator = "-"
	}
	for _, part := range strings.Split(name, separator) {
		if part == "" {
			return semver.Version{}, false
		}
		m := branchVersion.FindStringSubmatch(part)
		if m == nil || m[2] == "" || m[3] == "" {
			continue
		}
		v, err := semver.Parse(strings.Join(m[1:4], "."))
		if err == nil {
			return v, true
		}
	}
	return semver.Version{}, false
}

func mergedBranch(message string) string {
	parsed, err := ParseMergeMessage(&message, nil)
	if err != nil || parsed.MergedBranch == nil {
		return ""
	}
	return strings.TrimPrefix(*parsed.MergedBranch, "origin/")
}
func configuredNextVersionCandidate(cfg config.Config, label string) (candidate, bool, error) {
	if cfg.Next == nil || !cfg.Strategies["ConfiguredNextVersion"] || !cfg.Next.Matches(label) {
		return candidate{}, false, nil
	}
	v := *cfg.Next
	if !v.IsPreRelease() || v.Label != label {
		var err error
		v, err = v.Bump(semver.None, label, false)
		if err != nil {
			return candidate{}, false, err
		}
	}
	return candidate{base: *cfg.Next, semantic: *cfg.Next, version: v, description: "configured next-version"}, true, nil
}

func workflowCandidates(r *gitrepo.Repository, cfg config.Config, e config.Effective, branch, source, head string, refs []branchRef, history []string, tags []tag, load func(string) (gitrepo.Commit, error), log *commitLog) ([]candidate, error) {
	var out []candidate
	if e.IsRelease && cfg.Strategies["VersionInBranchName"] {
		for _, name := range []string{branch, source} {
			if v, ok := versionInBranch(gitrepo.NewReferenceName(name).WithoutOrigin); ok {
				bumped, err := v.Bump(semver.None, e.Label, false)
				if err != nil {
					return nil, err
				}
				out = append(out, candidate{base: v, semantic: v, version: bumped, description: "version in branch name"})
				break
			}
		}
	}
	if e.TrackMergeMessage && cfg.Strategies["MergeMessage"] {
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
				inc, err = log.increment(head, base, e.Label, e.Increment, e.Messages, tags)
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
	if e.TracksRelease && cfg.Strategies["TrackReleaseBranches"] {
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
			inc, err := log.increment(head, base, e.Label, e.Increment, e.Messages, tags)
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
	c, err := r.CommitMetadata(other)
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
			c, err := r.CommitMetadata(id)
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
	effective, err := cfg.EffectiveAt(branch.name, "main", branch.sha)
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

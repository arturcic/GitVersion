// Package calculate independently implements the declared GitHub Flow, GitFlow
// and Mainline version-calculation subset.
// It reads raw configuration and Git objects; no reference outputs are inputs.
package calculate

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/output"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

type Candidate struct {
	Description string `json:"description"`
	Base        string `json:"base"`
	SHA         string `json:"sha"`
	Increment   string `json:"increment"`
	Version     string `json:"version"`
}
type Trace struct {
	Branch         string      `json:"branch"`
	Label          string      `json:"label"`
	Mode           string      `json:"mode"`
	Candidates     []Candidate `json:"candidates"`
	SemanticSource string      `json:"semantic_source"`
	SemanticSHA    string      `json:"semantic_sha"`
	CountSHA       string      `json:"count_sha"`
	Counted        []string    `json:"counted_commits"`
}
type tag struct {
	v   semver.Version
	ref gitrepo.Tag
}
type candidate struct {
	base, version, semantic semver.Version
	sha, semanticSHA        string
	inc                     semver.Increment
	description             string
	effective               config.Effective
	source                  string
}

var reset = regexp.MustCompile(`(?i)=semver:`)

// .NET's \s includes Unicode separators and NEL; Go's \s is ASCII-only.
const bumpPrefix = `(?i)[+=]semver:[\t\n\v\f\r\x{0085}\p{Z}]?`

var bumps = []*regexp.Regexp{regexp.MustCompile(bumpPrefix + `(none|skip)`), regexp.MustCompile(bumpPrefix + `(fix|patch)`), regexp.MustCompile(bumpPrefix + `(feature|minor)`), regexp.MustCompile(bumpPrefix + `(breaking|major)`)}
var releaseMerge = regexp.MustCompile(`(?i)\breleases?[/\\-]`)

func Calculate(path string) (map[string]any, Trace, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, Trace{Candidates: []Candidate{}, Counted: []string{}}, err
	}
	return CalculateWithConfig(path, cfg)
}

// CalculateWithConfig evaluates a repository using an already parsed configuration.
// Callers must obtain cfg from config.Parse or config.Load. The repository and
// its configuration file are not modified by the supplied configuration.
func CalculateWithConfig(path string, cfg config.Config) (map[string]any, Trace, error) {
	return calculateWithStrategy(path, cfg, nil)
}

// versionStrategy permits a source strategy to supply candidates to the shared pipeline.
type versionStrategy func() ([]candidate, error)

func calculateWithStrategy(path string, cfg config.Config, strategy versionStrategy) (map[string]any, Trace, error) {
	trace := Trace{Candidates: []Candidate{}, Counted: []string{}}
	r, err := gitrepo.Open(path)
	if err != nil {
		return nil, trace, err
	}
	ignored, err := newCommitFilter(r, cfg.Ignore)
	if err != nil {
		return nil, trace, err
	}
	canonical, head := r.Head()
	branch := strings.TrimPrefix(canonical, "refs/heads/")
	kind, _, err := cfg.Branch(branch)
	if err != nil {
		return nil, trace, err
	}
	var effective config.Effective
	format := func(result output.Result) (map[string]any, error) {
		result.Mode = effective.Mode
		// The variables provider uses the original branch's configured weight.
		weight := cfg.Options(kind).Weight
		result.Weight = &weight
		result.TagWeight = &effective.TagWeight
		result.Output = cfg.OutputFor(kind)
		return output.Variables(result)
	}
	trace.Branch = branch
	refs, err := r.References()
	if err != nil {
		return nil, trace, err
	}
	mainTips := []string{}
	branches := []branchRef{}
	for _, ref := range refs {

		local := strings.HasPrefix(ref.Name, "refs/heads/")
		remote := strings.HasPrefix(ref.Name, "refs/remotes/") && !strings.HasSuffix(ref.Name, "/HEAD")
		if !local && !remote {
			continue
		}
		name := ref.Name
		if local {
			name = strings.TrimPrefix(name, "refs/heads/")
		}

		refKind, _, err := cfg.Branch(name)
		if err != nil {
			return nil, trace, err
		}
		branches = append(branches, branchRef{name, ref.SHA, refKind})
		if local && refKind == "main" {
			mainTips = append(mainTips, ref.SHA)
		}
	}
	// Non-Mainline strategies can calculate an explicitly configured branch
	// without a main ref (for example a repository initialized on develop).
	// Inherited branches still require a source through the branch configuration finder.
	if len(mainTips) > 1 {
		return nil, trace, fmt.Errorf("unsupported main-branch topology: require at most one main/master branch")
	}
	finder := branchConfigurationFinder{
		config: cfg, branches: branches,
		sources: func(name string, visited map[string]bool) ([]string, error) {
			return configurationOrigins(r, cfg, name, branches, visited)
		},
		commit: func(name string) (string, bool, error) {
			for _, b := range branches {
				if b.name == name {
					c, err := r.CommitMetadata(b.sha)
					return c.Message, len(c.Parents) > 1, err
				}
			}
			return "", false, nil
		},
	}
	configurations, err := finder.configurations(branch, head)
	if err != nil {
		return nil, trace, err
	}
	if len(configurations) == 0 {
		return nil, trace, fmt.Errorf("no base versions determined from source branches")
	}
	history, err := r.Walk(gitrepo.WalkOptions{Include: head, Sort: gitrepo.Time})
	if err != nil {
		return nil, trace, err
	}
	history, err = ignored.ids(history)
	if err != nil {
		return nil, trace, err
	}
	// RepositoryStore.GetCurrentCommit takes the first branch-log entry. The
	// pinned managed backend orders by time, even when an ancestor is newer
	// than the physical tip. Keep the branch history for tag discovery, but
	// calculate and report against this selected commit.
	if len(history) == 0 {
		return nil, trace, fmt.Errorf("no commits on current branch")
	}
	head = history[0]
	effective, err = cfg.EffectiveOptions(branch, head, configurations[0].options, configurations[0].value.Inherit)
	if err != nil {
		return nil, trace, err
	}
	label := effective.Label
	main := effective.IsMain
	trace.Label = label
	trace.Mode = string(effective.Mode)
	commits := map[string]gitrepo.Commit{}
	reachable := map[string]bool{}
	load := func(id string) (gitrepo.Commit, error) {
		if c, ok := commits[id]; ok {
			return c, nil
		}
		c, err := r.CommitMetadata(id)
		if err == nil {
			commits[id] = c
		}
		return c, err
	}
	log := commitLog{walk: r.Walk, load: load, ignored: ignored}
	for _, id := range history {
		reachable[id] = true
		c, err := load(id)
		if err != nil {
			return nil, trace, err
		}
		if len(c.Parents) > 2 {
			return nil, trace, fmt.Errorf("unsupported octopus merge")
		}
	}
	current := commits[head]
	rawTags, err := r.Tags(refs)
	if err != nil {
		return nil, trace, err
	}
	allTags := []tag{}
	byCommit := map[string][]tag{}
	for _, raw := range rawTags {
		skip, err := cfg.Ignore.TagIgnored(strings.TrimPrefix(raw.Name, "refs/tags/"))
		if err != nil {
			return nil, trace, err
		}
		if skip {
			continue
		}
		allowed, err := ignored.include(raw.Commit)
		if err != nil {
			return nil, trace, err
		}
		if !allowed {
			continue
		}
		v, recognized, err := semver.ParseTag(strings.TrimPrefix(raw.Name, "refs/tags/"))
		if err != nil {
			return nil, trace, err
		}
		if !recognized {
			continue
		}
		if _, err = load(raw.Commit); err != nil {
			return nil, trace, err
		}
		t := tag{v, raw}
		allTags = append(allTags, t)
		byCommit[raw.Commit] = append(byCommit[raw.Commit], t)
	}
	// The C# service orders groups by their lookup commit, which can be a
	// merge target's parent rather than the tagged commit itself. Flatten and
	// deduplicate tags only after ordering those groups.
	eligibleTags := func(effective config.Effective) ([]tag, error) {
		type tagGroup struct {
			id   string
			tags []tag
		}
		groups := []tagGroup{}
		groupIndex := map[string]int{}
		addTag := func(id string, t tag) {
			i, ok := groupIndex[id]
			if !ok {
				i = len(groups)
				groupIndex[id] = i
				groups = append(groups, tagGroup{id: id})
			}
			for _, existing := range groups[i].tags {
				if existing.ref.Name == t.ref.Name {
					return
				}
			}
			groups[i].tags = append(groups[i].tags, t)
		}
		add := func(ids []string, dateLimit bool) {
			for _, id := range ids {
				for _, t := range byCommit[id] {
					if !dateLimit || commits[id].When <= current.When {
						addTag(id, t)
					}
				}
			}
		}
		add(history, true)
		if effective.TrackMergeTarget {
			for _, t := range allTags {
				for _, parent := range commits[t.ref.Commit].Parents {
					if reachable[parent] && commits[parent].When <= current.When {
						addTag(parent, t)
					}
				}
			}
		}
		for _, br := range branches {
			skip, err := cfg.Ignore.BranchIgnored(br.name)
			if err != nil {
				return nil, err
			}
			if skip {
				continue
			}
			o := cfg.Options(br.kind)
			take := (!effective.IsMain && !effective.IsRelease && truth(o.IsMain)) || (effective.TracksRelease && truth(o.IsRelease))
			if take && br.name != branch {
				ids, err := r.Walk(gitrepo.WalkOptions{Include: br.sha, Sort: gitrepo.Time})
				if err != nil {
					return nil, err
				}
				add(ids, false)
			}
		}
		sort.SliceStable(groups, func(i, j int) bool { return commits[groups[i].id].When > commits[groups[j].id].When })
		eligible := []tag{}
		seen := map[string]bool{}
		for _, group := range groups {
			for _, t := range group.tags {
				if !seen[t.ref.Name] {
					eligible = append(eligible, t)
					seen[t.ref.Name] = true
				}
			}
		}
		return eligible, nil
	}
	eligible, err := eligibleTags(effective)
	if err != nil {
		return nil, trace, err
	}

	// Tagged-HEAD policy can override normal deployment calculation.
	if effective.PreventTagged {
		var selected *tag
		for i := range eligible {
			t := &eligible[i]
			if t.ref.Commit == head && (effective.NullLabel || t.v.Matches(label)) && (selected == nil || t.v.Compare(selected.v) > 0) {
				selected = t
			}
		}
		if selected != nil {
			v := selected.v
			trace.SemanticSource = v.String()
			trace.SemanticSHA = head
			trace.CountSHA = head
			if effective.Mode == config.Deployment {
				v.Label = ""
				v.Number = nil
			}
			values, err := format(output.Result{Version: v, SemanticSource: selected.v, SemanticSHA: head, LegacySource: selected.v, CountSHA: head, Branch: branch, SHA: head, CommitTime: current.When, Main: main, Tagged: true})
			return values, trace, err
		}
	}
	candidates := []candidate{}
	for _, configuration := range configurations {
		source := configuration.branch
		effective, err := cfg.EffectiveOptions(branch, head, configuration.options, configuration.value.Inherit)
		if err != nil {
			return nil, trace, err
		}
		label := effective.Label
		eligible, err := eligibleTags(effective)
		if err != nil {
			return nil, trace, err
		}
		localCandidates, err := func() ([]candidate, error) {
			if strategy != nil {
				return strategy()
			}
			candidates := []candidate{}
			next, exists, err := configuredNextVersionCandidate(cfg, label)
			if err != nil {
				return nil, err
			}
			if exists {
				candidates = append(candidates, next)
			}
			if cfg.Strategies["Mainline"] {
				c, err := mainlineCandidate(r, cfg, branch, source, head, branches, allTags, load, label)
				if err != nil {
					return nil, err
				}
				candidates = append(candidates, c)
			}
			if cfg.Strategies["TaggedCommit"] {
				var alternative *tag
				threshold := semver.Version{}
				for _, t := range eligible {
					if !t.v.Matches(label) {
						if alternative == nil || t.v.Compare(alternative.v) > 0 {
							copy := t
							alternative = &copy
						}
						continue
					}
					upper, err := t.v.Bump(semver.Major, "", true)
					if err != nil {
						return nil, err
					}
					if alternative != nil && upper.CompareCore(alternative.v) < 0 {
						upper = alternative.v
					}
					if upper.CompareCore(threshold) < 0 {
						alternative = nil
						continue
					}
					inc, err := log.increment(head, t.ref.Commit, label, effective.Increment, effective.Messages, allTags)
					if err != nil {
						return nil, err
					}
					threshold, err = t.v.Bump(inc, "", true)
					if err != nil {
						return nil, err
					}
					v, err := t.v.Bump(inc, label, false)
					if err != nil {
						return nil, err
					}
					c := candidate{base: t.v, semantic: t.v, version: v, sha: t.ref.Commit, semanticSHA: t.ref.Commit, inc: inc, description: t.ref.Name}
					if alternative != nil && v.CompareCore(alternative.v) < 0 {
						if inc == semver.None {
							n := int64(1)
							c.version.Number = &n
						}
						c.version.Major = alternative.v.Major
						c.version.Minor = alternative.v.Minor
						c.version.Patch = alternative.v.Patch
						c.semantic = alternative.v
						c.semanticSHA = alternative.ref.Commit
						c.inc = semver.None
					}
					candidates = append(candidates, c)
					alternative = nil
				}
			}
			{
				versionBranches := []branchRef{}
				for _, b := range branches {
					skip, err := cfg.Ignore.BranchIgnored(b.name)
					if err != nil {
						return nil, err
					}
					if !skip {
						versionBranches = append(versionBranches, b)
					}
				}
				extra, err := workflowCandidates(r, cfg, effective, branch, source, head, versionBranches, history, allTags, load, &log)
				if err != nil {
					return nil, err
				}
				candidates = append(candidates, extra...)
			}
			if len(candidates) == 0 && cfg.Strategies["Fallback"] {
				base := ""
				for _, t := range eligible {
					if t.v.Matches(label) {
						base = t.ref.Commit
						break
					}
				}
				inc, err := log.increment(head, base, label, effective.Increment, effective.Messages, allTags)
				if err != nil {
					return nil, err
				}
				v, err := (semver.Version{}).Bump(inc, label, false)
				if err != nil {
					return nil, err
				}
				candidates = append(candidates, candidate{version: v, sha: base, inc: inc, description: "fallback"})
			}
			sort.SliceStable(candidates, func(i, j int) bool { return candidateOrder(candidates[i]) < candidateOrder(candidates[j]) })
			return candidates, nil
		}()
		if err != nil {
			return nil, trace, err
		}
		for i := range localCandidates {
			allowed, err := ignored.include(localCandidates[i].sha)
			if err != nil {
				return nil, trace, err
			}
			if !allowed {
				continue
			}
			localCandidates[i].effective = effective
			localCandidates[i].source = source
			candidates = append(candidates, localCandidates[i])
		}
	}
	if len(candidates) == 0 {
		return nil, trace, fmt.Errorf("no base versions determined from enabled strategies")
	}

	best := 0
	for i, c := range candidates {
		trace.Candidates = append(trace.Candidates, Candidate{c.description, c.base.String(), c.sha, c.inc.String(), c.version.String()})
		if c.version.Compare(candidates[best].version) > 0 {
			best = i
		}
	}
	// Equal incremented versions prefer the latest nonexternal source. Counting
	// selection is independent when configuration provides the winning version.
	matches := []int{}
	for i, c := range candidates {
		if c.sha != "" && c.version.Compare(candidates[best].version) == 0 {
			matches = append(matches, i)
		}
	}
	anchor := ""
	if len(matches) > 1 {
		selected := matches[0]
		for _, i := range matches[1:] {
			if commits[candidates[i].sha].When > commits[candidates[selected].sha].When {
				selected = i
			}
		}
		best = selected
		anchor = candidates[best].sha
	} else {
		selected := -1
		for i, c := range candidates {
			if c.sha == "" || (!candidates[best].version.IsPreRelease() && c.base.IsPreRelease()) {
				continue
			}
			if selected < 0 || c.version.Compare(candidates[selected].version) > 0 || (c.version.Compare(candidates[selected].version) == 0 && commits[c.sha].When > commits[candidates[selected].sha].When) {
				selected = i
			}
		}
		if selected >= 0 {
			anchor = candidates[selected].sha
		}
	}
	chosen := candidates[best]
	effective = chosen.effective
	source := chosen.source
	label = effective.Label
	main = effective.IsMain
	trace.Label = label
	trace.Mode = string(effective.Mode)
	if effective.Inherit {
		for _, br := range branches {
			if br.name == source {
				ids, err := r.Walk(gitrepo.WalkOptions{Include: br.sha, Sort: gitrepo.Time})
				if err != nil {
					return nil, trace, err
				}
				reachable = map[string]bool{}
				for _, id := range ids {
					reachable[id] = true
				}
			}
		}
	}
	counted, err := r.Walk(gitrepo.WalkOptions{Include: head, Exclude: anchor, Sort: gitrepo.Topological | gitrepo.Time})
	if err != nil {
		return nil, trace, err
	}
	counted, err = ignored.ids(counted)
	if err != nil {
		return nil, trace, err
	}
	distance := int64(len(counted))
	v := chosen.version
	if effective.Mode == config.Delivery {
		if v.Number == nil {
			return nil, trace, fmt.Errorf("continuous delivery requires numeric prerelease")
		}
		n := *v.Number + distance - 1
		v.Number = &n
	}
	if effective.Mode == config.Deployment {
		v.Label = ""
		v.Number = nil
	}
	// A higher, mismatching prerelease on the configuration-source branch can
	// raise the core version without changing the counting anchor or label.
	var maxTag *tag
	for i := range allTags {
		t := &allTags[i]
		if cfg.Ignore.Before != nil && commits[t.ref.Commit].When <= cfg.Ignore.Before.Unix() {
			continue
		}
		if reachable[t.ref.Commit] && commits[t.ref.Commit].When <= current.When && (maxTag == nil || t.v.Compare(maxTag.v) > 0) {
			maxTag = t
		}
	}
	if maxTag != nil && v.CompareCore(maxTag.v) < 0 {
		v.Major = maxTag.v.Major
		v.Minor = maxTag.v.Minor
		v.Patch = maxTag.v.Patch
		chosen.semantic = maxTag.v
		chosen.semanticSHA = maxTag.ref.Commit
		chosen.inc = semver.None
	}
	trace.SemanticSource = chosen.semantic.String()
	trace.SemanticSHA = chosen.semanticSHA
	trace.CountSHA = anchor
	trace.Counted = counted
	values, err := format(output.Result{Version: v, SemanticSource: chosen.semantic, SemanticSHA: chosen.semanticSHA, Increment: chosen.inc, LegacySource: chosen.base, CountSHA: anchor, Distance: distance, Branch: branch, SHA: head, CommitTime: current.When, Main: main})
	return values, trace, err
}

func (l *commitLog) increment(head, base, label string, defaultIncrement semver.Increment, messages string, tags []tag) (semver.Increment, error) {
	if messages == "Disabled" {
		return defaultIncrement, nil
	}
	var excluded []string
	for _, t := range tags {
		if t.v.Matches(label) {
			excluded = append(excluded, t.ref.Target)
		}
	}
	ids, err := l.incrementHistory(head, base, excluded)
	if err != nil {
		return semver.None, err
	}
	// Accumulate newest-to-oldest. A recognized reset ends the scan and can
	// lower the configured increment floor; newer messages still contribute.
	result := semver.None
	for _, id := range ids {
		c, err := l.load(id)
		if err != nil {
			return result, err
		}
		if messages == "MergeMessageOnly" && len(c.Parents) < 2 {
			continue
		}
		recognized := bumps[0].MatchString(c.Message)
		if !recognized {
			for _, inc := range []semver.Increment{semver.Major, semver.Minor, semver.Patch} {
				if bumps[inc].MatchString(c.Message) {
					recognized = true
					if inc > result {
						result = inc
					}
					break
				}
			}
		}
		if recognized && reset.MatchString(c.Message) {
			return result, nil
		}
	}
	if result < defaultIncrement {
		result = defaultIncrement
	}
	return result, nil
}

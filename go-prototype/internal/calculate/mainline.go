package calculate

import (
	"fmt"
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"sort"
)

type mlOrigin struct {
	version semver.Version
	sha     string
}
type mlOperator struct {
	labelNull     bool
	inc           semver.Increment
	label, anchor string
	force         bool
	alt           *mlOrigin
}

func (op mlOperator) bump(v semver.Version) (semver.Version, error) {
	mode := semver.Standard
	if op.force {
		mode = semver.Force
	}
	var label *string
	if !op.labelNull {
		label = &op.label
	}
	return v.BumpWithMode(op.inc, label, mode)
}

type mlBase struct {
	value          semver.Version
	origin, anchor string
	op             *mlOperator
}

func (b mlBase) version() (semver.Version, error) {
	if b.op == nil {
		return b.value, nil
	}
	v, err := b.op.bump(b.value)
	if err != nil {
		return v, err
	}
	if b.op.alt != nil && v.CompareCore(b.op.alt.version) < 0 {
		v.Major = b.op.alt.version.Major
		v.Minor = b.op.alt.version.Minor
		v.Patch = b.op.alt.version.Patch
		if b.op.inc == semver.None {
			n := int64(1)
			v.Number = &n
		}
	}
	return v, nil
}
func (b mlBase) semantic() (semver.Version, string, semver.Increment) {
	inc := semver.None
	if b.op != nil {
		inc = b.op.inc
		v, err := b.op.bump(b.value)
		if err == nil && b.op.alt != nil && v.CompareCore(b.op.alt.version) < 0 {
			return b.op.alt.version, b.op.alt.sha, semver.None
		}
	}
	return b.value, b.origin, inc
}
func (b *mlBase) apply(op mlOperator) error {
	v, err := b.version()
	if err != nil {
		return err
	}
	_, sha, _ := b.semantic()
	b.value = v
	b.origin = sha
	if op.anchor != "" {
		b.anchor = op.anchor
	}
	b.op = &op
	return nil
}

type mlNode struct {
	id, branch string
	iteration  *mlIteration
	prev, next *mlNode
	child      *mlIteration
	tags       []semver.Version
	effective  *config.Effective
	messageInc semver.Increment
}
type mlIteration struct {
	branch string
	nodes  []*mlNode
	parent *mlNode
}
type mlEngine struct {
	ignored   *commitFilter
	labelNull bool
	r         *gitrepo.Repository
	cfg       config.Config
	refs      []branchRef
	tags      []tag
	load      func(string) (gitrepo.Commit, error)
	target    string
	label     string
	when      int64
	visited   map[string]bool
	err       error
}

func (e *mlEngine) effective(n *mlNode) config.Effective {
	if n.effective != nil {
		return *n.effective
	}
	kind, _, _ := e.cfg.Branch(n.branch)
	o := e.cfg.Options(kind)
	v, _ := e.cfg.EffectiveFrom(n.branch, n.branch)
	if kind != "main" && o.Increment == nil {
		var parent *mlNode
		for p := n.prev; p != nil; p = p.prev {
			if p.branch != n.branch {
				parent = p
				break
			}
		}
		if parent == nil {
			parent = n.iteration.parent
		}
		if parent == nil {
			e.err = fmt.Errorf("Mainline increment remains Inherit on %s", n.branch)
		}
		if parent != nil {
			p := e.effective(parent)
			v.Increment = p.Increment
			if o.NullLabel {
				v.Label = p.Label
				v.NullLabel = p.NullLabel
			}
			if o.IsMain == nil {
				v.IsMain = p.IsMain
			}
			if o.Messages == nil {
				v.Messages = p.Messages
			}
			if o.IsRelease == nil {
				v.IsRelease = p.IsRelease
			}
			if o.TracksRelease == nil {
				v.TracksRelease = p.TracksRelease
			}
			if o.TrackMergeTarget == nil {
				v.TrackMergeTarget = p.TrackMergeTarget
			}
			if o.PreventMerged == nil {
				v.PreventMerged = p.PreventMerged
			}
			if o.PreventWhenMerged == nil {
				v.PreventWhenMerged = p.PreventWhenMerged
			}
		}
	}
	n.effective = &v
	return v
}
func (e *mlEngine) iterationEffective(it *mlIteration) config.Effective {
	if len(it.nodes) > 0 {
		n := &mlNode{branch: it.branch, iteration: it, prev: it.nodes[0]}
		return e.effective(n)
	}
	v, _ := e.cfg.Effective(it.branch)
	return v
}
func (e *mlEngine) tagMap(branch string, fallback map[string][]semver.Version) (map[string][]semver.Version, error) {
	var ref *branchRef
	for i := range e.refs {
		if e.refs[i].name == branch {
			ref = &e.refs[i]
			break
		}
	}
	if ref == nil {
		return fallback, nil
	}
	kind, _, _ := e.cfg.Branch(branch)
	o := e.cfg.Options(kind)
	out := map[string][]semver.Version{}
	own := map[string]bool{}
	add := func(tip string, limited bool) error {
		ids, err := e.r.Walk(gitrepo.WalkOptions{Include: tip, Sort: gitrepo.Time})
		if err != nil {
			return err
		}
		for _, id := range ids {
			if tip == ref.sha {
				own[id] = true
			}
			c, err := e.load(id)
			if err != nil {
				return err
			}
			if limited && c.When > e.when {
				continue
			}
			for _, t := range e.tags {
				if t.ref.Commit == id {
					out[id] = append(out[id], t.v)
				}
			}
		}
		return nil
	}
	if err := add(ref.sha, true); err != nil {
		return nil, err
	}
	for _, b := range e.refs {
		p := e.cfg.Options(b.kind)
		if b.name != branch && ((!(truth(o.IsMain) || truth(o.IsRelease)) && truth(p.IsMain)) || (truth(o.TracksRelease) && truth(p.IsRelease))) {
			if err := add(b.sha, false); err != nil {
				return nil, err
			}
		}
	}
	if truth(o.TrackMergeTarget) {
		for _, t := range e.tags {
			c, err := e.load(t.ref.Commit)
			if err != nil {
				return nil, err
			}
			for _, id := range c.Parents {
				p, err := e.load(id)
				if err != nil {
					return nil, err
				}
				if own[id] && p.When <= e.when {
					out[id] = append(out[id], t.v)
				}
			}
		}
	}
	return out, nil
}
func truth(v *bool) bool { return v != nil && *v }
func (e *mlEngine) transitions(branch, exclude string) (map[string][]branchRef, error) {
	var tip string
	kind, _, _ := e.cfg.Branch(branch)
	for _, b := range e.refs {
		if b.name == branch {
			tip = b.sha
		}
	}
	out := map[string][]branchRef{}
	if tip == "" {
		return out, nil
	}
	for _, b := range e.refs {
		if b.name == branch || b.name == exclude {
			continue
		}
		allowed := false
		for _, k := range e.cfg.SourceKinds(kind) {
			if b.kind == k {
				allowed = true
			}
		}
		if !allowed {
			continue
		}
		id, err := branchMergeBase(e.r, tip, b.sha)
		if err != nil {
			return nil, err
		}
		if id != "" {
			if len(out[id]) == 0 || truth(e.cfg.Options(b.kind).IsMain) {
				out[id] = append(out[id], b)
			}
		}
	}
	for id := range out {
		sort.SliceStable(out[id], func(i, j int) bool {
			return truth(e.cfg.Options(out[id][i].kind).IsMain) && !truth(e.cfg.Options(out[id][j].kind).IsMain)
		})
	}
	return out, nil
}
func (e *mlEngine) build(it *mlIteration, ids []string, tagged map[string][]semver.Version) (bool, error) {
	state := it.branch
	trans, err := e.transitions(state, "")
	if err != nil {
		return false, err
	}
	if tagged == nil {
		tagged, err = e.tagMap(state, nil)
		if err != nil {
			return false, err
		}
	}
	needKnown := false
	defer func() {
		for i, j := 0, len(it.nodes)-1; i < j; i, j = i+1, j-1 {
			it.nodes[i], it.nodes[j] = it.nodes[j], it.nodes[i]
		}
	}()
	for _, id := range ids {
		if e.ignored != nil {
			allowed, err := e.ignored.include(id)
			if err != nil {
				return false, err
			}
			if !allowed {
				continue
			}
		}
		if e.visited[id] {
			continue
		}
		e.visited[id] = true
		if choices := trans[id]; len(choices) > 0 {
			kind, _, _ := e.cfg.Branch(state)
			choice := choices[0]
			if !(truth(e.cfg.Options(kind).IsMain) && !truth(e.cfg.Options(choice.kind).IsMain)) {
				for _, option := range choices {
					if option.name == e.target && option.name != choice.name {
						dummy := &mlNode{branch: e.target, iteration: it}
						if len(it.nodes) > 0 {
							dummy.next = it.nodes[len(it.nodes)-1]
							dummy.next.prev = dummy
						}
						it.nodes = append(it.nodes, dummy)
						break
					}
				}
				old := state
				state = choice.name
				trans, err = e.transitions(state, old)
				if err != nil {
					return false, err
				}
				tagged, err = e.tagMap(state, tagged)
				if err != nil {
					return false, err
				}
			}
		}
		n := &mlNode{id: id, branch: state, iteration: it, tags: tagged[id]}
		if len(it.nodes) > 0 {
			n.next = it.nodes[len(it.nodes)-1]
			n.next.prev = n
		}
		it.nodes = append(it.nodes, n)
		kind, _, _ := e.cfg.Branch(state)
		known := kind == "main" || e.cfg.Options(kind).Increment != nil
		matchLabel, matchNull := e.label, e.labelNull
		if matchNull {
			stateEffective, _ := e.cfg.EffectiveFrom(state, state)
			matchLabel, matchNull = stateEffective.Label, stateEffective.NullLabel
		}
		for _, v := range n.tags {
			if matchNull || v.Matches(matchLabel) {
				if known {
					return true, nil
				}
				needKnown = true
			}
		}
		if needKnown && known {
			return true, nil
		}
		c, err := e.load(id)
		if err != nil {
			return false, err
		}
		if len(c.Parents) != 2 || (e.cfg.Options(kind).TrackMergeMessage != nil && !*e.cfg.Options(kind).TrackMergeMessage) {
			continue
		}
		name := mergedBranch(c.Message)
		if name == "" || name == state {
			continue
		}
		childKind, _, err := e.cfg.Branch(name)
		if err != nil {
			return false, fmt.Errorf("Mainline merge branch: %w", err)
		}
		parentIndex := 1
		if truth(e.cfg.Options(childKind).IsMain) {
			if truth(e.cfg.Options(kind).IsMain) {
				return false, fmt.Errorf("pinned Mainline does not support main-branch-to-main-branch merge")
			}
			parentIndex = 0
			name = it.branch
		}
		base, err := mergeBase(e.r, c.Parents[0], c.Parents[1])
		if err != nil {
			return false, err
		}
		if base == "" {
			return false, fmt.Errorf("merge has no common ancestor")
		}
		merged, err := e.r.Walk(gitrepo.WalkOptions{Include: c.Parents[parentIndex], Sort: gitrepo.Topological})
		if err != nil {
			return false, err
		}
		// GetMergedCommits reverses a Topological|Reverse walk after slicing
		// past the merge base; timestamp sorting is deliberately absent.
		for i, x := range merged {
			if x == base {
				merged = merged[:i]
				break
			}
		}
		child := &mlIteration{branch: name, parent: n}
		done, err := e.build(child, merged, tagged)
		if err != nil {
			return false, err
		}
		n.child = child
		if done {
			return true, nil
		}
		for _, x := range merged {
			e.visited[x] = true
		}
	}
	return false, nil
}

type mlContext struct {
	increment       semver.Increment
	suppress, force bool
	label           string
	labelSet        bool
	anchor          string
	semantic, alt   *mlOrigin
}

func (c *mlContext) addAlt(v mlOrigin) {
	if c.alt == nil || v.version.Compare(c.alt.version) > 0 {
		copy := v
		c.alt = &copy
	}
}
func (c *mlContext) operator() mlOperator {
	return mlOperator{labelNull: !c.labelSet, inc: c.increment, label: c.label, anchor: c.anchor, force: c.force, alt: c.alt}
}
func commitBump(message string) (semver.Increment, bool) {
	if bumps[0].MatchString(message) {
		return semver.None, reset.MatchString(message)
	}
	for _, v := range []semver.Increment{semver.Major, semver.Minor, semver.Patch} {
		if bumps[v].MatchString(message) {
			return v, reset.MatchString(message)
		}
	}
	return semver.None, false
}
func (e *mlEngine) reduce(it *mlIteration) (mlBase, error) {
	result := mlBase{}
	ctx := mlContext{}
	iterationEffective := e.iterationEffective(it)
	for _, n := range it.nodes {
		eff := e.effective(n)
		if e.err != nil {
			return result, e.err
		}
		commit := gitrepo.Commit{}
		if n.id != "" {
			var err error
			commit, err = e.load(n.id)
			if err != nil {
				return result, err
			}
		}
		ctx.semantic = nil
		matchLabel, matchNull := e.label, e.labelNull
		if matchNull {
			matchLabel, matchNull = iterationEffective.Label, iterationEffective.NullLabel
		}
		if matchNull {
			matchLabel, matchNull = eff.Label, eff.NullLabel
		}
		for _, v := range n.tags {
			if matchNull || v.Matches(matchLabel) {
				if ctx.semantic == nil || v.Compare(ctx.semantic.version) > 0 {
					ctx.semantic = &mlOrigin{v, n.id}
				}
			} else {
				ctx.addAlt(mlOrigin{v, n.id})
			}
		}
		inc, resetBump := semver.None, false
		if eff.Messages == "Enabled" || (eff.Messages == "MergeMessageOnly" && len(commit.Parents) > 1) {
			inc, resetBump = commitBump(commit.Message)
		}
		n.messageInc = inc
		if resetBump {
			ctx.increment = semver.None
			ctx.suppress = true
		}
		ctx.increment = max(ctx.increment, inc)
		if !ctx.suppress {
			ctx.increment = max(ctx.increment, eff.Increment)
		}
		if n.prev != nil && n.prev.branch != n.branch {
			ctx.labelSet = false
		}
		if !ctx.labelSet {
			ctx.label = eff.Label
			ctx.labelSet = !eff.NullLabel
		}
		if eff.IsMain {
			ctx.anchor = ""
			if n.prev != nil {
				ctx.anchor = n.prev.id
			}
		}
		predecessorMain := n.prev != nil && e.effective(n.prev).IsMain
		ctx.force = ctx.force || eff.IsMain || (!eff.IsMain && predecessorMain)
		hasChild := n.child != nil && len(n.child.nodes) > 0
		operand := func() { result = mlBase{value: ctx.semantic.version, origin: ctx.semantic.sha, anchor: ctx.anchor} }
		apply := func() error { return result.apply(ctx.operator()) }
		if !hasChild {
			if ctx.semantic != nil {
				ctx.anchor = n.id
				operand()
				if !eff.IsMain {
					ctx.increment = eff.Increment
					ctx.label = eff.Label
					ctx.labelSet = !eff.NullLabel
					if ctx.semantic.version.IsPreRelease() {
						ctx.force = false
					}
				}
				if eff.IsMain && !ctx.semantic.version.IsPreRelease() {
					ctx.label = eff.Label
					ctx.labelSet = !eff.NullLabel
				}
				if n.next == nil && (!eff.IsMain || iterationEffective.IsMain) {
					if ctx.semantic.version.IsPreRelease() {
						ctx.increment = eff.Increment
						ctx.label = eff.Label
						ctx.labelSet = !eff.NullLabel
						ctx.force = false
					} else {
						ctx.force = true
					}
					if err := apply(); err != nil {
						return result, err
					}
				}
			} else {
				if !eff.IsMain && eff.IsRelease && (n.prev == nil || n.prev.branch != n.branch) {
					if v, ok := versionInBranch(n.branch); ok {
						ctx.addAlt(mlOrigin{version: v})
					}
				}
				if eff.IsMain {
					ctx.force = true
					if err := apply(); err != nil {
						return result, err
					}
					ctx.anchor = n.id
				} else if n.next == nil {
					if err := apply(); err != nil {
						return result, err
					}
					ctx.anchor = n.id
					ctx.force = false
				}
			}
		} else if ctx.semantic == nil {
			child, err := e.reduce(n.child)
			if err != nil {
				return result, err
			}
			if !ctx.labelSet && child.op != nil {
				ctx.label = child.op.label
				ctx.labelSet = !child.op.labelNull
			}
			childInc := semver.None
			if child.op != nil {
				childInc = child.op.inc
			}
			consolidated := semver.None
			if !eff.PreventMerged {
				consolidated = max(consolidated, ctx.increment)
			}
			prevent := eff.PreventWhenMerged
			if eff.IsMain {
				prevent = e.iterationEffective(n.child).PreventWhenMerged
			}
			if !prevent {
				consolidated = max(consolidated, childInc)
			}
			if eff.Messages != "Disabled" {
				consolidated = max(consolidated, n.messageInc)
			}
			ctx.increment = consolidated
			if child.anchor != "" {
				ctx.anchor = child.anchor
				// A merge consumes the child operand, not its pending alternative.
				// C# GetBaselineSemVerSource preserves that distinction.
				ctx.semantic = &mlOrigin{child.value, child.origin}
				if eff.IsMain {
					ctx.force = child.op != nil && child.op.force
				}
			}
			if child.anchor == "" {
				if !eff.IsMain && child.value.Compare(semver.Version{}) != 0 {
					ctx.addAlt(mlOrigin{child.value, child.origin})
				}
				if child.op != nil && child.op.alt != nil {
					ctx.addAlt(*child.op.alt)
				}
			}
			if eff.IsMain {
				if ctx.semantic != nil {
					operand()
				}
				if err := apply(); err != nil {
					return result, err
				}
				ctx.anchor = n.id
			} else if n.next == nil {
				if !e.labelNull {
					ctx.label = e.label
					ctx.labelSet = true
				}
				if err := apply(); err != nil {
					return result, err
				}
			}
		}
		// A branch with no own commit still needs its own final operator/label.
		if n.next == nil && n.branch != it.branch {
			ctx.anchor = n.id
			if !iterationEffective.NullLabel {
				ctx.label = iterationEffective.Label
				ctx.labelSet = true
			}
			ctx.force = true
			if eff.IsMain {
				ctx.increment = iterationEffective.Increment
				if iterationEffective.IsRelease {
					if v, ok := versionInBranch(it.branch); ok {
						ctx.addAlt(mlOrigin{version: v})
					}
				}
				if err := apply(); err != nil {
					return result, err
				}
			} else {
				op := ctx.operator()
				op.anchor = ""
				op.inc = semver.None
				op.force = false
				if err := result.apply(op); err != nil {
					return result, err
				}
			}
		}
		if eff.IsMain {
			ctx.increment = semver.None
			ctx.suppress = false
			ctx.labelSet = false
			ctx.alt = nil
		}
	}
	return result, nil
}
func mainlineCandidate(r *gitrepo.Repository, cfg config.Config, branch, source, head string, refs []branchRef, tags []tag, load func(string) (gitrepo.Commit, error), label string) (candidate, error) {
	c, err := load(head)
	if err != nil {
		return candidate{}, err
	}
	effective, err := cfg.EffectiveFrom(branch, source)
	if err != nil {
		return candidate{}, err
	}
	ignored, err := newCommitFilter(r, cfg.Ignore)
	if err != nil {
		return candidate{}, err
	}
	engine := mlEngine{ignored: ignored, labelNull: effective.NullLabel, r: r, cfg: cfg, refs: refs, tags: tags, load: load, target: source, label: label, when: c.When, visited: map[string]bool{}}
	tip := head
	for _, b := range refs {
		if b.name == branch {
			tip = b.sha
		}
	}
	ids, err := r.Walk(gitrepo.WalkOptions{Include: tip, Sort: gitrepo.Time})
	if err != nil {
		return candidate{}, err
	}
	it := &mlIteration{branch: branch}
	_, err = engine.build(it, ids, nil)
	if err != nil {
		return candidate{}, err
	}
	b, err := engine.reduce(it)
	if err != nil {
		return candidate{}, err
	}
	v, err := b.version()
	if err != nil {
		return candidate{}, err
	}
	semantic, sha, inc := b.semantic()
	return candidate{base: b.value, semantic: semantic, version: v, sha: b.anchor, semanticSHA: sha, inc: inc, description: "mainline"}, nil
}

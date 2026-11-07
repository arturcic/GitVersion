package calculate

import "github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"

const commitLogSort = gitrepo.Topological | gitrepo.Time

// commitLog keeps the topological projection of one head for version candidate
// evaluation. The metadata loader is shared with the calculation.
type commitLog struct {
	walk    func(gitrepo.WalkOptions) ([]string, error)
	load    func(string) (gitrepo.Commit, error)
	ignored *commitFilter

	head  string
	order []string

	resetKnown bool
	hasReset   bool
}

func (l *commitLog) project(head string) ([]string, error) {
	if l.order != nil && l.head == head {
		return l.order, nil
	}
	order, err := l.walk(gitrepo.WalkOptions{Include: head, Sort: commitLogSort})
	if err != nil {
		return nil, err
	}
	l.head, l.order, l.resetKnown = head, order, false
	return order, nil
}

// ancestors follows the base through ignored commits and commits outside the
// projection. Tag pruning passes a retained set as a barrier instead.
func (l *commitLog) ancestors(roots []string, retained map[string]bool) (map[string]bool, error) {
	seen := map[string]bool{}
	pending := append([]string(nil), roots...)
	for len(pending) > 0 {
		last := len(pending) - 1
		id := pending[last]
		pending = pending[:last]
		if id == "" || seen[id] || (retained != nil && !retained[id]) {
			continue
		}
		commit, err := l.load(id)
		if err != nil {
			return nil, err
		}
		seen[id] = true
		pending = append(pending, commit.Parents...)
	}
	return seen, nil
}

func (l *commitLog) derive(head, base string, ignored *commitFilter, excluded []string) ([]string, error) {
	order, err := l.project(head)
	if err != nil {
		return nil, err
	}
	order, err = ignored.ids(order)
	if err != nil {
		return nil, err
	}
	baseAncestors, err := l.ancestors([]string{base}, nil)
	if err != nil {
		return nil, err
	}
	return l.prune(order, baseAncestors, excluded)
}

// prune removes commits already covered by tags without crossing the
// boundaries left by the ignore filter.
func (l *commitLog) prune(order []string, baseAncestors map[string]bool, excluded []string) ([]string, error) {
	var hidden map[string]bool
	if len(excluded) != 0 {
		retained := make(map[string]bool, len(order))
		for _, id := range order {
			retained[id] = true
		}
		var err error
		hidden, err = l.ancestors(excluded, retained)
		if err != nil {
			return nil, err
		}
	}
	if len(baseAncestors) == 0 && len(hidden) == 0 {
		return order, nil
	}
	out := make([]string, 0, len(order))
	for _, id := range order {
		if !baseAncestors[id] && !hidden[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

func (l *commitLog) incrementHistory(head, base string, excluded []string) ([]string, error) {
	order, err := l.project(head)
	if err != nil {
		return nil, err
	}
	if !l.resetKnown {
		history, err := l.ignored.ids(order)
		if err != nil {
			return nil, err
		}
		hasReset := false
		for _, id := range history {
			commit, err := l.load(id)
			if err != nil {
				return nil, err
			}
			if reset.MatchString(commit.Message) {
				hasReset = true
				break
			}
		}
		l.resetKnown, l.hasReset = true, hasReset
	}
	if !l.hasReset {
		return l.derive(head, base, l.ignored, excluded)
	}
	// Removing ancestors can change equal-time ties in the walk. A reset makes
	// those ties observable, so preserve the ordering of the bounded walk.
	order, err = l.walk(gitrepo.WalkOptions{Include: head, Exclude: base, Sort: commitLogSort})
	if err != nil {
		return nil, err
	}
	order, err = l.ignored.ids(order)
	if err != nil {
		return nil, err
	}
	return l.prune(order, nil, excluded)
}

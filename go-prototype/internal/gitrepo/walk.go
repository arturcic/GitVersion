package gitrepo

import (
	"fmt"
	"slices"
)

// Sort values match the pinned C# CommitSortStrategies flags. None is a
// date-directed traversal; Time sorts the limited set using compatibility ties.
type Sort uint8

const (
	Topological Sort = 1
	Time        Sort = 2
	Reverse     Sort = 4
)

type WalkOptions struct {
	Include, Exclude string
	Sort             Sort
	FirstParent      bool
}
type node struct {
	id                          string
	when                        int64
	parents                     []*node
	parsed, seen, hidden, added bool
	degree                      int
}
type walk struct {
	repo    *Repository
	options WalkOptions
	nodes   map[string]*node
}

func (w *walk) lookup(id string) *node {
	if n, ok := w.nodes[id]; ok {
		return n
	}
	n := &node{id: id}
	w.nodes[id] = n
	return n
}
func (w *walk) parse(n *node) error {
	if n.parsed {
		return nil
	}
	c, err := w.repo.load(n.id)
	if err != nil {
		return err
	}
	n.when = c.Committer.When.Unix()
	for _, p := range c.ParentHashes {
		n.parents = append(n.parents, w.lookup(p.String()))
	}
	n.parsed = true
	return nil
}
func hideParents(n *node) {
	pending := append([]*node{}, n.parents...)
	for len(pending) > 0 {
		last := len(pending) - 1
		p := pending[last]
		pending = pending[:last]
		if p.hidden {
			continue
		}
		p.hidden = true
		if p.parsed {
			pending = append(pending, p.parents...)
		}
	}
}
func insertDate(list []*node, n *node) []*node {
	i := 0
	for i < len(list) && list[i].when >= n.when {
		i++
	}
	return slices.Insert(list, i, n)
}
func (w *walk) addParents(n *node, list []*node) ([]*node, error) {
	if n.added {
		return list, nil
	}
	n.added = true
	for _, p := range n.parents {
		if n.hidden {
			p.hidden = true
			if err := w.parse(p); err != nil {
				return nil, err
			}
			hideParents(p)
			p.seen = true
			list = insertDate(list, p)
			continue
		}
		if err := w.parse(p); err != nil {
			return nil, err
		}
		if !p.seen {
			p.seen = true
			list = insertDate(list, p)
		}
		if w.options.FirstParent {
			break
		}
	}
	return list, nil
}

// Walk mirrors the pinned managed backend's limiting pass and heap mechanics.
// Using go-git's default log order here would silently change equal-date ties.
func (r *Repository) Walk(o WalkOptions) ([]string, error) {
	return r.walkFrom(o, []string{o.Include})
}

func (r *Repository) walkFrom(o WalkOptions, includes []string) ([]string, error) {
	if o.Sort > 7 {
		return nil, fmt.Errorf("unsupported sort flags %d", o.Sort)
	}
	w := walk{repo: r, options: o, nodes: map[string]*node{}}
	input := []*node{}
	limited := o.Sort != 0
	// libgit2 prepends each seed in the order it was supplied.
	for _, id := range slices.Backward(includes) {
		if id != "" {
			input = append(input, w.lookup(id))
		}
	}
	if o.Exclude != "" {
		n := w.lookup(o.Exclude)
		n.hidden = true
		input = append([]*node{n}, input...)
		limited = true
	}
	seeds := []*node{}
	for _, n := range input {
		if err := w.parse(n); err != nil {
			return nil, err
		}
		if n.hidden {
			hideParents(n)
		}
		if !n.seen {
			n.seen = true
			seeds = append(seeds, n)
		}
	}
	list := append([]*node{}, seeds...)
	result := []*node{}
	slop := 5
	var lastTime int64 = 1<<63 - 1
	for len(list) > 0 {
		n := list[0]
		list = list[1:]
		var err error
		list, err = w.addParents(n, list)
		if err != nil {
			return nil, err
		}
		if n.hidden {
			if limited {
				hideParents(n)
				if len(list) == 0 {
					break
				}
				interesting := lastTime <= list[0].when
				for _, p := range list {
					if !p.hidden || p.when > lastTime {
						interesting = true
						break
					}
				}
				if interesting {
					slop = 5
				} else {
					slop--
				}
				if slop <= 0 {
					break
				}
			}
			continue
		}
		lastTime = n.when
		result = append(result, n)
	}
	if o.Sort&Topological != 0 {
		for _, n := range result {
			n.degree = 1
		}
		for _, n := range result {
			for _, p := range n.parents {
				if p.degree > 0 {
					p.degree++
				}
			}
		}
		q := nodeHeap{byTime: o.Sort&Time != 0}
		for _, n := range result {
			if n.degree == 1 {
				q.push(n)
			}
		}
		if !q.byTime {
			slices.Reverse(q.items)
		}
		result = []*node{}
		for len(q.items) > 0 {
			n := q.pop()
			for _, p := range n.parents {
				if p.degree > 0 {
					p.degree--
					if p.degree == 1 {
						q.push(p)
					}
				}
			}
			n.degree = 0
			if !n.hidden {
				result = append(result, n)
			}
		}
	} else if o.Sort&Time != 0 {
		q := nodeHeap{byTime: true}
		for _, n := range result {
			q.push(n)
		}
		result = []*node{}
		for len(q.items) > 0 {
			n := q.pop()
			if !n.hidden {
				result = append(result, n)
			}
		}
	}
	if o.Sort&Reverse != 0 {
		slices.Reverse(result)
	}
	ids := []string{}
	for _, n := range result {
		if !n.hidden {
			ids = append(ids, n.id)
		}
	}
	return ids, nil
}

// Strict comparisons intentionally leave equal times in heap-dependent order.
type nodeHeap struct {
	items  []*node
	byTime bool
}

func (q *nodeHeap) push(n *node) {
	q.items = append(q.items, n)
	if !q.byTime {
		return
	}
	i := len(q.items) - 1
	for i > 0 {
		p := (i - 1) / 2
		if q.items[p].when >= n.when {
			break
		}
		q.items[i] = q.items[p]
		i = p
	}
	q.items[i] = n
}
func (q *nodeHeap) pop() *node {
	last := len(q.items) - 1
	if !q.byTime {
		n := q.items[last]
		q.items = q.items[:last]
		return n
	}
	result := q.items[0]
	n := q.items[last]
	q.items = q.items[:last]
	if last == 0 {
		return result
	}
	i := 0
	for {
		c := 2*i + 1
		if c >= len(q.items) {
			break
		}
		if c+1 < len(q.items) && q.items[c].when < q.items[c+1].when {
			c++
		}
		if n.when >= q.items[c].when {
			break
		}
		q.items[i] = q.items[c]
		i = c
	}
	q.items[i] = n
	return result
}

// MergeBase returns the same single best common ancestor selected by the pinned
// managed backend. Criss-cross histories may have more than one valid best base.
func (r *Repository) MergeBase(a, b string) (*string, error) {
	if _, err := r.load(a); err != nil {
		return nil, err
	}
	if _, err := r.load(b); err != nil {
		return nil, err
	}
	if a == b {
		return &a, nil
	}
	flags := map[string]int{}
	type entry struct {
		id   string
		when int64
	}
	queue := []entry{}
	candidates := []string{}
	enqueue := func(id string, paint int) error {
		old := flags[id]
		updated := old | paint
		if updated == old {
			return nil
		}
		flags[id] = updated
		c, err := r.load(id)
		if err != nil {
			return err
		}
		e := entry{id, c.Committer.When.Unix()}
		i := 0
		for i < len(queue) && queue[i].when >= e.when {
			i++
		}
		queue = slices.Insert(queue, i, e)
		return nil
	}
	if err := enqueue(a, 1); err != nil {
		return nil, err
	}
	if err := enqueue(b, 2); err != nil {
		return nil, err
	}
	for len(queue) > 0 {
		interesting := false
		for _, e := range queue {
			if flags[e.id]&4 == 0 {
				interesting = true
				break
			}
		}
		if !interesting {
			break
		}
		e := queue[0]
		queue = queue[1:]
		f := flags[e.id]
		paint := f & 3
		if paint == 3 {
			if f&4 == 0 {
				candidates = append(candidates, e.id)
				flags[e.id] = f | 4
			}
			paint |= 4
		}
		c, err := r.load(e.id)
		if err != nil {
			return nil, err
		}
		for _, p := range c.ParentHashes {
			if err := enqueue(p.String(), paint); err != nil {
				return nil, err
			}
		}
	}
	for _, candidate := range candidates {
		redundant := false
		for _, other := range candidates {
			if other == candidate {
				continue
			}
			yes, err := r.isAncestor(candidate, other)
			if err != nil {
				return nil, err
			}
			if yes {
				redundant = true
				break
			}
		}
		if !redundant {
			return &candidate, nil
		}
	}
	return nil, nil
}
func (r *Repository) isAncestor(ancestor, tip string) (bool, error) {
	stack := []string{tip}
	seen := map[string]bool{}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if id == ancestor {
			return true, nil
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		c, err := r.load(id)
		if err != nil {
			return false, err
		}
		for _, p := range c.ParentHashes {
			stack = append(stack, p.String())
		}
	}
	return false, nil
}

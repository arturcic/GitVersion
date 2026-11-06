package fixtures

// Reduce removes steps until no single deletion preserves the caller's failure
// predicate. The predicate must return false for invalid graphs and process
// failures, and true only for the original mismatch category. It must retain
// any execution evidence it needs. This is one-step minimal, not a proof of the
// globally smallest DAG. Configuration, messages and timestamps stay unchanged.
func Reduce(input Recipe, fails func(Recipe) (bool, error)) (Recipe, int, error) {
	current := input
	current.Steps = append([]Step(nil), input.Steps...)
	calls := 1
	ok, err := fails(current)
	if err != nil || !ok {
		return current, calls, err
	}
	for {
		changed := false
		for i, s := range current.Steps {
			if s.Op == "checkpoint" {
				continue
			}
			candidate := current
			candidate.Steps = append(append([]Step(nil), current.Steps[:i]...), current.Steps[i+1:]...)
			calls++
			ok, err := fails(candidate)
			if err != nil {
				return current, calls, err
			}
			if ok {
				current = candidate
				changed = true
				break
			}
		}
		if !changed {
			return current, calls, nil
		}
	}
}

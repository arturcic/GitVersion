package fixtures

import (
	"fmt"
	"math/rand"
)

// Generated returns bounded, reproducible DAGs, each with one final checkpoint.
// Every recipe is retained verbatim by the experiment; a seed alone is not the
// long-term replay contract. Randomness varies graph shape as well as policy.
func Generated(seed int64, count int) ([]Recipe, error) {
	if count < 1 || count > 10000 {
		return nil, fmt.Errorf("generated count must be between 1 and 10000")
	}
	rng := rand.New(rand.NewSource(seed))
	out := make([]Recipe, 0, count)
	messages := []string{"ordinary change", "+semver: patch", "+semver: minor", "+semver: major", "+semver: skip +semver: major", "+semver:\u00a0feature"}
	for i := 0; i < count; i++ {
		r := Recipe{ID: fmt.Sprintf("generated-%04d", i), Description: fmt.Sprintf("seed %d case %d", seed, i), Config: "workflow: GitHubFlow/v1\n"}
		inc := []string{"None", "Patch", "Minor", "Major"}[rng.Intn(4)]
		r.Config += "calculation:\n  branches:\n    main:\n      increment: " + inc + "\n"
		if rng.Intn(5) == 0 {
			r.Config += fmt.Sprintf("  next-version: %d.%d.%d\n", 1+rng.Intn(8), rng.Intn(5), rng.Intn(7))
		}
		ordinal, clock := 0, rng.Intn(4)
		commit := func(op, from string) {
			seconds := int64(ordinal * 60)
			if clock == 1 {
				seconds = 0
			}
			if clock == 2 {
				seconds = int64(rng.Intn(12) * 60)
			}
			r.Steps = append(r.Steps, Step{Op: op, ID: fmt.Sprintf("commit-%02d", ordinal), From: from, Message: messages[rng.Intn(len(messages))], Seconds: &seconds})
			ordinal++
		}
		commit("commit", "")
		tagOrdinal := 0
		tag := func(label string) {
			v := fmt.Sprintf("%d.%d.%d", 1+rng.Intn(5), rng.Intn(6), tagOrdinal)
			tagOrdinal++
			if rng.Intn(3) == 0 {
				v += "-" + label + fmt.Sprintf(".%d", 1+rng.Intn(9))
			}
			if rng.Intn(2) == 0 {
				v = "v" + v
			}
			r.Steps = append(r.Steps, Step{Op: "tag", Name: v, Annotated: rng.Intn(2) == 0})
		}
		if rng.Intn(4) != 0 {
			tag("beta")
		}
		branches := []string{"main"}
		current := "main"
		for j, rounds := 0, 3+rng.Intn(9); j < rounds; j++ {
			if len(branches) < 4 && rng.Intn(4) == 0 {
				current = fmt.Sprintf("feature/topic-%d", len(branches))
				branches = append(branches, current)
				r.Steps = append(r.Steps, Step{Op: "branch", Name: current}, Step{Op: "checkout", Name: current})
			} else if rng.Intn(3) == 0 {
				current = branches[rng.Intn(len(branches))]
				r.Steps = append(r.Steps, Step{Op: "checkout", Name: current})
			}
			commit("commit", "")
			if len(branches) > 1 && rng.Intn(3) == 0 {
				other := branches[rng.Intn(len(branches))]
				if other != current {
					commit("merge", other)
				}
			}
			if rng.Intn(3) == 0 {
				label := "beta"
				if current != "main" && rng.Intn(2) == 0 {
					label = current[len("feature/"):]
				}
				tag(label)
			}
		}
		r.Steps = append(r.Steps, Step{Op: "checkpoint", ID: "head"})
		out = append(out, r)
	}
	return out, nil
}

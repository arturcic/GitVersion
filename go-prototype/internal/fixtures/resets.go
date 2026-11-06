package fixtures

import (
	"fmt"
	"math/rand"
)

// GeneratedResets preserves the E policy generator and adds independent reset
// messages to commits and merges, retaining the seed for every replay.
func GeneratedResets(seed int64, count int) ([]Recipe, error) {
	recipes, err := GeneratedPolicies(seed, count)
	if err != nil {
		return nil, err
	}
	rng := rand.New(rand.NewSource(seed + 2))
	messages := []string{"=semver: none", "=semver: patch", "=semver: minor", "=semver: major", "=semver: unknown", "=semver: skip\n+semver: major", "=semver: unknown\n+semver: patch", "=SeMvEr:\u00a0fix"}
	for i := range recipes {
		recipes[i].ID = "reset-" + recipes[i].ID
		for j := range recipes[i].Steps {
			step := &recipes[i].Steps[j]
			if (step.Op == "commit" || step.Op == "merge") && rng.Intn(3) == 0 {
				step.Message = messages[rng.Intn(len(messages))]
			}
		}
	}
	return recipes, nil
}

// ResetRecipes isolates default reset-message semantics from the first E corpus.
// Expected versions are captured from the pinned C# executable, never calculated
// by fixture construction.
func ResetRecipes() []Recipe {
	var recipes []Recipe
	messages := []string{"=semver: patch", "=semver: none", "=semver: minor", "=semver: major", "+semver: major\n=semver: skip", "=semver: unknown", "=semver: unknown\n+semver: patch", "=SeMvEr:\u00a0fix"}
	for bi, branch := range []string{"main", "feature/topic"} {
		kind := "main"
		if bi == 1 {
			kind = "feature"
		}
		for mi, mode := range []string{"Enabled", "Disabled", "MergeMessageOnly"} {
			config := fmt.Sprintf("workflow: GitHubFlow/v1\ncalculation:\n  branches:\n    main:\n      increment: Major\n    %s:\n      commit-message-incrementing: %s\n", kind, mode)
			if bi == 0 {
				config = fmt.Sprintf("workflow: GitHubFlow/v1\ncalculation: {branches: {main: {increment: Major, commit-message-incrementing: %s}}}\n", mode)
			}
			start := func() []Step {
				steps := []Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}}
				if bi == 1 {
					steps = append(steps, Step{Op: "branch", Name: branch}, Step{Op: "checkout", Name: branch})
				}
				return steps
			}
			for i, message := range messages {
				steps := append(start(), Step{Op: "commit", ID: "old", Message: "+semver: major"}, Step{Op: "checkpoint", ID: "before"}, Step{Op: "commit", ID: "reset", Message: message}, Step{Op: "checkpoint", ID: "reset"}, Step{Op: "commit", ID: "new", Message: "+semver: minor"}, Step{Op: "checkpoint", ID: "after"})
				recipes = append(recipes, Recipe{ID: fmt.Sprintf("reset-linear-%d-%d-%d", bi, mi, i), Description: "reset before/at/after with configured Major floor", Config: config, Steps: steps})
			}
			for i := 0; i < 4; i++ {
				steps := append(start(), Step{Op: "branch", Name: "feature/side"}, Step{Op: "checkout", Name: "feature/side"}, Step{Op: "commit", ID: "side", Message: "+semver: major"}, Step{Op: "checkout", Name: branch}, Step{Op: "commit", ID: "own", Message: "=semver: none"})
				if i == 1 || i == 2 {
					seconds := int64(120)
					steps[len(steps)-3].Seconds = &seconds
					steps[len(steps)-1].Seconds = &seconds
				}
				message := "merge =semver: patch"
				if i == 2 {
					message = "merge without directive"
				}
				steps = append(steps, Step{Op: "merge", ID: "merge", From: "feature/side", Message: message}, Step{Op: "checkpoint", ID: "merged"})
				if i == 3 {
					steps = append(steps, Step{Op: "tag", Name: "2.0.0", Annotated: true})
				}
				steps = append(steps, Step{Op: "commit", ID: "new", Message: "+semver: minor"}, Step{Op: "checkpoint", ID: "after"})
				recipes = append(recipes, Recipe{ID: fmt.Sprintf("reset-merge-%d-%d-%d", bi, mi, i), Description: "merge/reset order, timestamp ties, annotated tag exclusion", Config: config, Steps: steps})
			}
		}
	}
	return recipes
}

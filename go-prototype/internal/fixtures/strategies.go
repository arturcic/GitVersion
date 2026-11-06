package fixtures

import (
	"fmt"
	"strings"
)

// StrategyRecipes enumerates every subset of the seven C# strategy flags.
// Histories activate distinct sources; no expected version is generated here.
func StrategyRecipes() []Recipe {
	names := []string{"Fallback", "ConfiguredNextVersion", "MergeMessage", "TaggedCommit", "TrackReleaseBranches", "VersionInBranchName", "Mainline"}
	var out []Recipe
	for mask := 0; mask < 128; mask++ {
		var selected []string
		for i, n := range names {
			if mask&(1<<i) != 0 {
				selected = append(selected, n)
			}
		}
		for _, scenario := range []string{"empty", "tag", "next", "merge", "develop", "release"} {
			raw := "workflow: GitFlow/v1\ncalculation:\n  strategies: [" + strings.Join(selected, ", ") + "]\n"
			if scenario == "next" {
				raw += "  next-version: 2.4.0\n"
			}
			steps := []Step{{Op: "commit", ID: "root"}}
			if scenario != "empty" {
				steps = append(steps, Step{Op: "tag", Name: "1.2.3"})
			}
			steps = append(steps, Step{Op: "commit", ID: "main"})
			if scenario == "merge" || scenario == "develop" || scenario == "release" {
				steps = append(steps, Step{Op: "branch", Name: "develop"}, Step{Op: "checkout", Name: "develop"}, Step{Op: "commit", ID: "development"}, Step{Op: "branch", Name: "release/1.4.0"}, Step{Op: "checkout", Name: "release/1.4.0"}, Step{Op: "commit", ID: "release"})
				if scenario == "merge" {
					steps = append(steps, Step{Op: "checkout", Name: "main"}, Step{Op: "merge", ID: "merged", From: "release/1.4.0", Message: "Merge branch 'release/1.4.0'"})
				}
				if scenario == "develop" {
					steps = append(steps, Step{Op: "checkout", Name: "develop"}, Step{Op: "commit", ID: "next-development"})
				}
			}
			steps = append(steps, Step{Op: "checkpoint", ID: "head"})
			out = append(out, Recipe{ID: fmt.Sprintf("strategies-%03d-%s", mask, scenario), Description: "strategy subset on " + scenario, Config: raw, Steps: steps})
		}
	}
	return out
}

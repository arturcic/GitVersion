package fixtures

import "fmt"

// MergeControlRecipes observes each merge before a later release tag can hide
// its policy effects. Destination and child prevention controls vary separately.
func MergeControlRecipes() []Recipe {
	var out []Recipe
	for _, flow := range []string{"GitHubFlow/v1", "GitFlow/v1"} {
		targets := []string{"main"}
		if flow == "GitFlow/v1" {
			targets = append(targets, "develop")
		}
		for _, mainline := range []bool{false, true} {
			for _, target := range targets {
				for _, kind := range []string{"feature", "release"} {
					for mask := 0; mask < 16; mask++ {
						name := kind + "/topic"
						if kind == "release" {
							name = "release/2.0.0"
						}
						raw := "workflow: " + flow + "\ncalculation:\n"
						if mainline {
							raw += "  strategies: [Mainline]\n"
						}
						raw += fmt.Sprintf("  branches:\n    %s:\n      track-merge-message: %t\n      track-merge-target: %t\n      prevent-increment: {of-merged-branch: %t}\n    %s:\n      prevent-increment: {when-branch-merged: %t}\n", target, mask&4 != 0, mask&8 != 0, mask&1 != 0, kind, mask&2 != 0)
						steps := []Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}}
						if target == "develop" {
							steps = append(steps, Step{Op: "branch", Name: target}, Step{Op: "checkout", Name: target}, Step{Op: "commit", ID: "development"})
						}
						steps = append(steps, Step{Op: "branch", Name: name}, Step{Op: "checkout", Name: name}, Step{Op: "commit", ID: "work", Message: "+semver: minor"}, Step{Op: "checkout", Name: target}, Step{Op: "commit", ID: "target"}, Step{Op: "merge", ID: "merged", From: name, Message: "Merge branch '" + name + "'"}, Step{Op: "checkpoint", ID: "merged"}, Step{Op: "commit", ID: "after"}, Step{Op: "checkpoint", ID: "after"})
						out = append(out, Recipe{ID: fmt.Sprintf("merge-controls-%03d", len(out)), Description: "before/after untagged merge with independent source and destination controls", Config: raw, Steps: steps})
					}
				}
			}
		}
	}
	return out
}

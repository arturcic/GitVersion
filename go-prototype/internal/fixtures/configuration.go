package fixtures

import (
	"fmt"
	"strings"
)

// ConfigurationRecipes crosses nullable branch controls with workflow strategies.
func ConfigurationRecipes() []Recipe {
	var out []Recipe
	for _, mainline := range []bool{false, true} {
		for _, branch := range []string{"main", "develop", "feature/topic"} {
			kind := strings.Split(branch, "/")[0]
			for mask := 0; mask < 128; mask++ {
				raw := "workflow: GitFlow/v1\ncalculation:\n"
				if mainline {
					raw += "  strategies: [Mainline]\n"
				}
				raw += fmt.Sprintf("  branches:\n    %s:\n      is-main-branch: %t\n      is-release-branch: %t\n      tracks-release-branches: %t\n      track-merge-target: %t\n      track-merge-message: %t\n      prevent-increment:\n        of-merged-branch: %t\n        when-branch-merged: %t\n", kind, mask&1 != 0, mask&2 != 0, mask&4 != 0, mask&8 != 0, mask&16 != 0, mask&32 != 0, mask&64 != 0)
				steps := []Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}, {Op: "branch", Name: "develop"}, {Op: "checkout", Name: "develop"}, {Op: "commit", ID: "dev"}, {Op: "branch", Name: "feature/topic"}, {Op: "checkout", Name: "feature/topic"}, {Op: "commit", ID: "feature", Message: "+semver: minor"}, {Op: "checkout", Name: "develop"}, {Op: "merge", ID: "feature-merge", From: "feature/topic", Message: "Merge branch 'feature/topic'"}, {Op: "branch", Name: "release/1.4.0"}, {Op: "checkout", Name: "release/1.4.0"}, {Op: "commit", ID: "release"}, {Op: "checkout", Name: "main"}, {Op: "merge", ID: "release-merge", From: "release/1.4.0", Message: "Merge branch 'release/1.4.0'"}, {Op: "tag", Name: "1.4.0"}, {Op: "commit", ID: "after-release"}, {Op: "checkout", Name: branch}, {Op: "commit", ID: "next"}, {Op: "checkpoint", ID: "head"}}
				out = append(out, Recipe{ID: fmt.Sprintf("controls-%t-%s-%03d", mainline, kind, mask), Description: "branch classification, tracking and merge prevention controls", Config: raw, Steps: steps})
			}
		}
	}
	for _, next := range []string{"1.4.0-beta.3", "1.4.0-alpha.2", "1.4.0", "1.2.4-topic.2"} {
		for _, branch := range []string{"main", "develop", "release/1.4.0", "feature/topic"} {
			for _, strategies := range []string{"ConfiguredNextVersion, Fallback", "ConfiguredNextVersion, TaggedCommit, Mainline"} {
				steps := []Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}}
				if branch != "main" {
					steps = append(steps, Step{Op: "branch", Name: branch}, Step{Op: "checkout", Name: branch})
				}
				steps = append(steps, Step{Op: "commit", ID: "next"}, Step{Op: "checkpoint", ID: "head"})
				out = append(out, Recipe{ID: fmt.Sprintf("next-prerelease-%03d", len(out)), Description: "configured prerelease matching and strategy competition", Config: fmt.Sprintf("workflow: GitFlow/v1\ncalculation:\n  next-version: %s\n  strategies: [%s]\n", next, strategies), Steps: steps})
			}
		}
	}
	return out
}

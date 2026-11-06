package fixtures

import (
	"fmt"
	"strings"
)

// ConfigurationEdgeRecipes includes direct ports of named C# integration tests.
// The description identifies the original test; SemVer retains its assertion.
func ConfigurationEdgeRecipes() []Recipe {
	var out []Recipe
	for _, tc := range []struct {
		id, method, branch, extra, want string
		merge, empty                    bool
	}{
		{"release-name", "VersionInCurrentBranchNameScenarios.TakesVersionFromNameOfReleaseBranch", "release/2.0.0", "", "2.0.0-beta.1+1", false, false},
		{"feature-name", "VersionInCurrentBranchNameScenarios.DoesNotTakeVersionFromNameOfNonReleaseBranch", "feature/upgrade-power-level-to-9000.0.1", "", "1.1.0-upgrade-power-level-to-9000-0-1.1+1", false, false},
		{"support-name", "VersionInCurrentBranchNameScenarios.TakesVersionFromNameOfBranchThatIsReleaseByConfig", "support/2.0.0", "calculation: {branches: {support: {is-release-branch: true}}}\n", "2.0.0-1", false, false},
		{"release-merged", "VersionInMergedBranchNameScenarios.TakesVersionFromNameOfReleaseBranch", "release/2.0.0", "", "2.1.0-alpha.2", true, false},
		{"support-merged", "VersionInMergedBranchNameScenarios.TakesVersionFromNameOfBranchThatIsReleaseByConfig", "support/2.0.0", "calculation: {branches: {support: {is-release-branch: true}}}\n", "2.1.0-alpha.2", true, false},
		{"release-accidental", "VersionInMergedBranchNameScenarios.DoesNotTakeVersionFromBranchWithAccidentalVersion", "release/downgrade-some-lib-to-3.2.1", "", "1.0.1-2", true, true},
		{"hotfix-accidental", "VersionInMergedBranchNameScenarios.DoesNotTakeVersionFromBranchWithAccidentalVersion", "hotfix/downgrade-some-lib-to-3.2.1", "", "1.0.1-2", true, true},
	} {
		steps := []Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.0"}}
		target := "main"
		if !tc.empty {
			steps = append(steps, Step{Op: "branch", Name: "develop"}, Step{Op: "checkout", Name: "develop"}, Step{Op: "commit", ID: "dev"})
			target = "develop"
		}
		steps = append(steps, Step{Op: "branch", Name: tc.branch}, Step{Op: "checkout", Name: tc.branch})
		if tc.merge {
			steps = append(steps, Step{Op: "commit", ID: "branch"}, Step{Op: "checkout", Name: target}, Step{Op: "merge", ID: "merged", From: tc.branch, Message: "Merge branch '" + tc.branch + "'"})
		}
		steps = append(steps, Step{Op: "checkpoint", ID: "head", SemVer: tc.want})
		out = append(out, Recipe{ID: "csharp-" + tc.id, Description: tc.method, Config: "workflow: GitFlow/v1\n" + tc.extra, Steps: steps})
	}
	for i, list := range []string{"None", "None, TaggedCommit", "TaggedCommit, TaggedCommit", "Mainline, TaggedCommit", "TaggedCommit, Mainline", "None, Mainline", "ConfiguredNextVersion, ConfiguredNextVersion"} {
		r := StrategyRecipes()[2] // tagged main with next-version, exactly the same shape.
		r.ID = fmt.Sprintf("strategy-edge-%02d", i)
		r.Config = strings.Replace(r.Config, "strategies: []", "strategies: ["+list+"]", 1)
		r.Description = "None, duplicate and reordered strategy selections"
		out = append(out, r)
	}
	return out
}

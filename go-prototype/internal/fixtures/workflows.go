package fixtures

import "fmt"

// WorkflowRecipes describes raw Git histories independently of version logic.
func WorkflowRecipes() []Recipe {
	var out []Recipe
	for _, suite := range []struct {
		id, workflow string
		mainline     bool
	}{
		{"gitflow", "GitFlow/v1", false}, {"mainline-gh", "GitHubFlow/v1", true}, {"mainline-gf", "GitFlow/v1", true},
	} {
		raw := "workflow: " + suite.workflow + "\n"
		if suite.mainline {
			raw += "calculation: {strategies: [Mainline]}\n"
		}
		branches := []string{"main", "feature/topic", "release/1.3.0", "pull/42/merge"}
		if suite.workflow == "GitFlow/v1" {
			branches = append(branches, "develop", "hotfix/1.2.4", "support/1.x")
		}
		for i, branch := range branches {
			steps := []Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}}
			if suite.workflow == "GitFlow/v1" && (branch == "develop" || branch == "feature/topic" || branch == "release/1.3.0" || branch == "pull/42/merge") {
				steps = append(steps, Step{Op: "branch", Name: "develop"}, Step{Op: "checkout", Name: "develop"}, Step{Op: "commit", ID: "development"})
			}
			if branch != "main" && branch != "develop" {
				steps = append(steps, Step{Op: "branch", Name: branch}, Step{Op: "checkout", Name: branch})
			}
			steps = append(steps, Step{Op: "checkpoint", ID: "start"}, Step{Op: "commit", ID: "plain"}, Step{Op: "checkpoint", ID: "plain"}, Step{Op: "commit", ID: "minor", Message: "+semver: minor"}, Step{Op: "checkpoint", ID: "minor"}, Step{Op: "tag", Name: "2.0.0", Annotated: true}, Step{Op: "checkpoint", ID: "tagged"}, Step{Op: "commit", ID: "after-tag"}, Step{Op: "checkpoint", ID: "after-tag"})
			out = append(out, Recipe{ID: fmt.Sprintf("%s-branch-%d", suite.id, i), Description: branch + " default workflow", Config: raw, Steps: steps})
		}
		for i, message := range []string{"Merge branch 'feature/topic'", "merge without source name", "Merge branch 'feature/topic'\n\n+semver: major"} {
			steps := []Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "root"}, {Op: "commit", ID: "second"}, {Op: "checkpoint", ID: "second"}, {Op: "tag", Name: "1.2.3"}, {Op: "branch", Name: "feature/topic"}, {Op: "checkout", Name: "feature/topic"}, {Op: "commit", ID: "feature-one"}, {Op: "commit", ID: "feature-two", Message: "+semver: minor"}, {Op: "checkpoint", ID: "feature"}, {Op: "checkout", Name: "main"}, {Op: "commit", ID: "main-one"}, {Op: "checkpoint", ID: "main"}, {Op: "merge", ID: "merge", From: "feature/topic", Message: message}, {Op: "checkpoint", ID: "merged"}, {Op: "commit", ID: "after"}, {Op: "checkpoint", ID: "after"}}
			out = append(out, Recipe{ID: fmt.Sprintf("%s-merge-%d", suite.id, i), Description: "linear accumulation, divergent feature merge and following commit", Config: raw, Steps: steps})
		}
		if suite.workflow == "GitFlow/v1" {
			steps := []Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}, {Op: "branch", Name: "develop"}, {Op: "checkout", Name: "develop"}, {Op: "commit", ID: "dev"}, {Op: "checkpoint", ID: "develop"}, {Op: "branch", Name: "feature/topic"}, {Op: "checkout", Name: "feature/topic"}, {Op: "commit", ID: "feature", Message: "+semver: minor"}, {Op: "checkpoint", ID: "feature"}, {Op: "checkout", Name: "develop"}, {Op: "merge", ID: "feature-merge", From: "feature/topic", Message: "Merge branch 'feature/topic'"}, {Op: "checkpoint", ID: "feature-merged"}, {Op: "branch", Name: "release/1.3.0"}, {Op: "checkout", Name: "release/1.3.0"}, {Op: "commit", ID: "release"}, {Op: "checkpoint", ID: "release"}, {Op: "checkout", Name: "develop"}, {Op: "checkpoint", ID: "release-exists"}, {Op: "commit", ID: "next-development"}, {Op: "checkpoint", ID: "next-development"}, {Op: "checkout", Name: "main"}, {Op: "merge", ID: "release-merge", From: "release/1.3.0", Message: "Merge branch 'release/1.3.0'"}, {Op: "checkpoint", ID: "release-merged"}, {Op: "tag", Name: "1.3.0"}, {Op: "checkpoint", ID: "released"}, {Op: "checkout", Name: "develop"}, {Op: "merge", ID: "back-merge", From: "main", Message: "Merge branch 'main' into develop"}, {Op: "checkpoint", ID: "back-merged"}, {Op: "checkout", Name: "main"}, {Op: "branch", Name: "hotfix/1.3.1"}, {Op: "checkout", Name: "hotfix/1.3.1"}, {Op: "commit", ID: "hotfix"}, {Op: "checkpoint", ID: "hotfix"}, {Op: "checkout", Name: "main"}, {Op: "merge", ID: "hotfix-merge", From: "hotfix/1.3.1", Message: "Merge branch 'hotfix/1.3.1'"}, {Op: "checkpoint", ID: "hotfix-merged"}}
			out = append(out, Recipe{ID: suite.id + "-lifecycle", Description: "feature, release, develop tracking, main back-merge and hotfix lifecycle", Config: raw, Steps: steps})
		}
	}
	return out
}

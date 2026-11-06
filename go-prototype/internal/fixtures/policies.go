package fixtures

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// PolicyRecipes fixes the first E expansion matrix independently of calculator
// code. Raw YAML is the only configuration passed to either implementation.
func PolicyRecipes() ([]Recipe, error) {
	var out []Recipe
	add := func(id, branch, label string, settings map[string]any, output map[string]any) error {
		kind := "main"
		if branch != "main" {
			kind = "feature"
		}
		doc := map[string]any{"workflow": "GitHubFlow/v1", "calculation": map[string]any{"branches": map[string]any{kind: settings}}}
		if output != nil {
			doc["output"] = output
		}
		data, err := yaml.Marshal(doc)
		if err != nil {
			return err
		}
		steps := []Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}}
		if branch != "main" {
			steps = append(steps, Step{Op: "branch", Name: branch}, Step{Op: "checkout", Name: branch})
		}
		steps = append(steps, Step{Op: "checkpoint", ID: "tagged-root"}, Step{Op: "commit", ID: "one"}, Step{Op: "checkpoint", ID: "after"}, Step{Op: "branch", Name: "feature/side"}, Step{Op: "checkout", Name: "feature/side"}, Step{Op: "commit", ID: "side", Message: "+semver: major"}, Step{Op: "checkout", Name: branch}, Step{Op: "merge", ID: "merge", From: "feature/side", Message: "merge +semver: minor"}, Step{Op: "checkpoint", ID: "merged"})
		version := "2.0.0"
		if label != "" {
			version += "-" + label + ".4"
		}
		steps = append(steps, Step{Op: "tag", Name: version, Annotated: true}, Step{Op: "checkpoint", ID: "tagged-head"}, Step{Op: "commit", ID: "after-tag"}, Step{Op: "checkpoint", ID: "after-tag"})
		out = append(out, Recipe{ID: id, Description: "E policy: " + id, Config: string(data), Steps: steps})
		return nil
	}
	for _, branch := range []string{"main", "feature/topic"} {
		kind, label := "main", ""
		if branch != "main" {
			kind, label = "feature", "topic"
		}
		for mi, mode := range []string{"ManualDeployment", "ContinuousDelivery", "ContinuousDeployment"} {
			for _, prevent := range []bool{false, true} {
				if err := add(fmt.Sprintf("policy-mode-%s-%d-%t", kind, mi, prevent), branch, label, map[string]any{"mode": mode, "prevent-increment": map[string]any{"when-current-commit-tagged": prevent}}, nil); err != nil {
					return nil, err
				}
			}
			for _, inc := range []string{"None", "Patch", "Minor", "Major"} {
				if err := add(fmt.Sprintf("policy-increment-%s-%d-%s", kind, mi, strings.ToLower(inc)), branch, label, map[string]any{"mode": mode, "increment": inc}, nil); err != nil {
					return nil, err
				}
			}
		}
		for i, label := range []string{"", "preview", "build.{BranchName}", "42"} {
			if kind == "main" {
				label = strings.ReplaceAll(label, "{BranchName}", "main")
			}
			resolved := strings.ReplaceAll(label, "{BranchName}", strings.TrimPrefix(branch, "feature/"))
			if err := add(fmt.Sprintf("policy-label-%s-%d", kind, i), branch, resolved, map[string]any{"label": label}, map[string]any{"tag-pre-release-weight": 0, "branches": map[string]any{kind: map[string]any{"pre-release-weight": 0}}}); err != nil {
				return nil, err
			}
		}
		for i, mode := range []string{"Enabled", "Disabled", "MergeMessageOnly"} {
			if err := add(fmt.Sprintf("policy-messages-%s-%d", kind, i), branch, label, map[string]any{"commit-message-incrementing": mode}, map[string]any{"tag-pre-release-weight": 123, "branches": map[string]any{kind: map[string]any{"pre-release-weight": 42}}}); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

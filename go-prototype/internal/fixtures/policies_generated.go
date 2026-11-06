package fixtures

import (
	"gopkg.in/yaml.v3"
	"math/rand"
)

// GeneratedPolicies layers independent raw overrides onto the unchanged D DAG
// generator. Keep both corpora: adding support must not replace old coverage.
func GeneratedPolicies(seed int64, count int) ([]Recipe, error) {
	recipes, err := Generated(seed, count)
	if err != nil {
		return nil, err
	}
	rng := rand.New(rand.NewSource(seed + 1))
	for i := range recipes {
		var doc map[string]any
		if err := yaml.Unmarshal([]byte(recipes[i].Config), &doc); err != nil {
			return nil, err
		}
		calc := doc["calculation"].(map[string]any)
		branches := calc["branches"].(map[string]any)
		weights := map[string]any{}
		for _, kind := range []string{"main", "feature"} {
			b, ok := branches[kind].(map[string]any)
			if !ok {
				b = map[string]any{}
				branches[kind] = b
			}
			b["mode"] = []string{"ManualDeployment", "ContinuousDelivery", "ContinuousDeployment"}[rng.Intn(3)]
			b["prevent-increment"] = map[string]any{"when-current-commit-tagged": rng.Intn(2) == 0}
			b["commit-message-incrementing"] = []string{"Enabled", "Disabled", "MergeMessageOnly"}[rng.Intn(3)]
			if kind == "feature" {
				b["increment"] = []string{"Inherit", "None", "Patch", "Minor", "Major"}[rng.Intn(5)]
			}
			if rng.Intn(2) == 0 {
				labels := []string{"", "preview", "42", "rc.0"}
				if kind == "feature" {
					labels = append(labels, "build.{BranchName}")
				}
				b["label"] = labels[rng.Intn(len(labels))]
			}
			weights[kind] = map[string]any{"pre-release-weight": []int{0, 1, 42, 30000, 55000}[rng.Intn(5)]}
		}
		doc["output"] = map[string]any{"tag-pre-release-weight": []int{0, 1, 123, 60000}[rng.Intn(4)], "branches": weights}
		data, err := yaml.Marshal(doc)
		if err != nil {
			return nil, err
		}
		recipes[i].ID = "expanded-" + recipes[i].ID
		recipes[i].Description = "E policy overrides; " + recipes[i].Description
		recipes[i].Config = string(data)
	}
	return recipes, nil
}

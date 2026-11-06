package fixtures

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"strings"
)

// GeneratedWorkflows preserves the established randomized DAG/policy generator
// and independently varies workflows, branch families and parsed merge messages.
func GeneratedWorkflows(seed int64, count int) ([]Recipe, error) {
	recipes, err := GeneratedPolicies(seed, count)
	if err != nil {
		return nil, err
	}
	for i := range recipes {
		var doc map[string]any
		if err := yaml.Unmarshal([]byte(recipes[i].Config), &doc); err != nil {
			return nil, err
		}
		workflow := "GitFlow/v1"
		if i%3 == 1 {
			workflow = "GitHubFlow/v1"
		}
		doc["workflow"] = workflow
		if i%3 != 0 {
			doc["calculation"].(map[string]any)["strategies"] = []string{"Mainline"}
		}
		rename := func(s string) string {
			if workflow != "GitFlow/v1" {
				return s
			}
			switch s {
			case "feature/topic-1":
				return "develop"
			case "feature/topic-2":
				return "release/2.0.0"
			case "feature/topic-3":
				return "hotfix/1.2.4"
			}
			return s
		}
		for j := range recipes[i].Steps {
			s := &recipes[i].Steps[j]
			if s.Op == "branch" || s.Op == "checkout" {
				s.Name = rename(s.Name)
			}
			if s.Op == "merge" {
				s.From = rename(s.From)
				if (i+j)%5 != 0 {
					s.Message = "Merge branch '" + s.From + "'\n\n" + s.Message
				}
			}
			if s.Op == "commit" && (i+j)%11 == 0 {
				s.Message = strings.Replace(s.Message, "+semver:", "=semver:", 1)
			}
		}
		raw, err := yaml.Marshal(doc)
		if err != nil {
			return nil, err
		}
		recipes[i].Config = string(raw)
		recipes[i].ID = fmt.Sprintf("workflow-generated-%04d", i)
		recipes[i].Description = fmt.Sprintf("workflow seed %d case %d, variant %d", seed, i, i%3)
	}
	return recipes, nil
}

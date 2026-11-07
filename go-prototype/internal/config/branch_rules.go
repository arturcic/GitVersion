package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
	"gopkg.in/yaml.v3"
)

func ptrBranch(s string) *string { return &s }

func normalizedBranchName(name string) string {
	name = strings.TrimPrefix(name, "refs/heads/")
	if strings.HasPrefix(name, "refs/remotes/") {
		name = strings.TrimPrefix(strings.TrimPrefix(name, "refs/remotes/"), "origin/")
	}
	return name
}

// Use the frozen workflow's ordered rules. The unknown rule is considered only
// after all named branch configurations, including user-defined ones.
type workflowBranchRule struct {
	kind, expression string
	sources          []string
	pattern          *patterns.Pattern
}

// Embedded rules are immutable and shared; user overrides live only in Config.
var embeddedBranchRules = sync.OnceValue(func() map[string][]workflowBranchRule {
	result := map[string][]workflowBranchRule{}
	for _, workflow := range []string{"GitFlow/v1", "GitHubFlow/v1", "TrunkBased/preview1"} {
		raw, err := workflowPresets.ReadFile("presets/" + workflow + ".yml")
		if err != nil {
			panic(err)
		}
		node, err := readConfigurationMap(raw)
		if err != nil {
			panic(err)
		}
		branches := nodeField(nodeField(node, "calculation"), "branches")
		for i := 0; i < len(branches.Content); i += 2 {
			kind, branch := branches.Content[i].Value, branches.Content[i+1]
			rule := workflowBranchRule{kind: kind, expression: fieldValue(branch, "regex")}
			rule.pattern, err = patterns.Compile(rule.expression, patterns.IgnoreCase)
			if err != nil {
				panic(err)
			}
			for _, source := range nodeField(branch, "source-branches").Content {
				rule.sources = append(rule.sources, source.Value)
			}
			result[workflow] = append(result[workflow], rule)
		}
	}
	return result
})

func (c *Config) initializeBranchRules() error {
	c.branchRegex = map[string]string{}
	c.branchPatterns = map[string]*patterns.Pattern{}
	c.branchSources = map[string][]string{}
	for _, rule := range embeddedBranchRules()[c.Workflow] {
		c.branchOrder = append(c.branchOrder, rule.kind)
		c.branchRegex[rule.kind] = rule.expression
		c.branchPatterns[rule.kind] = rule.pattern
		c.branchSources[rule.kind] = rule.sources
	}
	return nil
}

func (c *Config) parseBranchRules(node *yaml.Node, kind string) error {
	if expression := nodeField(node, "regex"); expression != nil {
		value, err := scalar(expression, "branches."+kind+".regex")
		if err != nil {
			return err
		}
		compiled, err := patterns.Compile(value, patterns.IgnoreCase)
		if err != nil {
			return fmt.Errorf("branches.%s.regex: %w", kind, err)
		}
		c.branchRegex[kind] = value
		c.branchPatterns[kind] = compiled
	}
	if _, exists := c.branchRegex[kind]; !exists {
		return fmt.Errorf("branches.%s requires a regex", kind)
	}
	if sources := nodeField(node, "source-branches"); sources != nil {
		if sources.Kind != yaml.SequenceNode || sources.Anchor != "" {
			return fmt.Errorf("branches.%s.source-branches requires a list", kind)
		}
		c.branchSources[kind] = nil
		for _, source := range sources.Content {
			value, err := scalar(source, "source-branches")
			if err != nil {
				return err
			}
			c.branchSources[kind] = append(c.branchSources[kind], value)
		}
	}
	return nil
}

func (c Config) matchBranchRule(name string) (string, string, error) {
	if name == "" {
		return "", "", fmt.Errorf("empty branch name")
	}
	order := append([]string(nil), c.branchOrder...)
	for _, kind := range order {
		if kind == "unknown" {
			continue
		}
		match, err := c.matchRule(kind, name)
		if err != nil {
			return "", "", err
		}
		if match != nil {
			return kind, branchCapture(match, name), nil
		}
	}
	match, err := c.matchRule("unknown", name)
	if err != nil {
		return "", "", err
	}
	if match != nil {
		return "unknown", branchCapture(match, name), nil
	}
	return "unknown", Sanitize(name, false), nil
}

func (c Config) matchRule(kind, name string) (*patterns.Match, error) {
	p := c.branchPatterns[kind]
	if p == nil {
		return nil, nil
	}
	return p.Find(name)
}

func branchCapture(match *patterns.Match, fallback string) string {
	for _, name := range []string{"BranchName", "Number"} {
		if value, ok := match.Group(name); ok {
			return Sanitize(value, false)
		}
	}
	return Sanitize(fallback, false)
}

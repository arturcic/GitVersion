package config

import (
	"fmt"
	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
	"gopkg.in/yaml.v3"
	"strings"
	"time"
)

type IgnoreSettings struct {
	SHAs, Paths, Branches, Tags []string
	Before                      *time.Time
}

func (i IgnoreSettings) Empty() bool {
	return len(i.SHAs) == 0 && len(i.Paths) == 0 && len(i.Branches) == 0 && len(i.Tags) == 0 && i.Before == nil
}
func parseIgnore(n *yaml.Node) (IgnoreSettings, error) {
	var out IgnoreSettings
	fields, err := mapping(n, "calculation.ignore", []string{"sha", "commits-before", "paths", "branches", "tags"})
	if err != nil {
		return out, err
	}
	for _, key := range []string{"sha", "paths", "branches", "tags"} {
		node := fields[key]
		if node == nil {
			continue
		}
		if node.Kind != yaml.SequenceNode || node.Anchor != "" {
			return out, fmt.Errorf("ignore.%s requires a list", key)
		}
		seen := map[string]bool{}
		for _, item := range node.Content {
			s, e := scalar(item, "ignore."+key)
			if e != nil {
				return out, e
			}
			if seen[s] {
				continue
			}
			seen[s] = true
			if key == "sha" {
				out.SHAs = append(out.SHAs, s)
			} else {
				if _, e := patterns.Compile(s, patterns.IgnoreCase); e != nil {
					return out, fmt.Errorf("invalid regular expression in ignore.%s", key)
				}
				switch key {
				case "paths":
					out.Paths = append(out.Paths, s)
				case "branches":
					out.Branches = append(out.Branches, s)
				case "tags":
					out.Tags = append(out.Tags, s)
				}
			}
		}
	}
	if node := fields["commits-before"]; node != nil {
		if node.Kind != yaml.ScalarNode || (node.Tag != "!!str" && node.Tag != "!!timestamp") || node.Anchor != "" {
			return out, fmt.Errorf("ignore.commits-before requires a date")
		}
		value, err := time.Parse(time.RFC3339Nano, node.Value)
		if err != nil {
			value, err = time.ParseInLocation("2006-01-02T15:04:05", node.Value, time.Local)
		}
		if err != nil || value.Year() < 1 || value.Year() > 9999 {
			return out, fmt.Errorf("invalid ignore.commits-before date")
		}
		out.Before = &value
	}
	return out, nil
}

func (i IgnoreSettings) Validate() error {
	for _, field := range []struct {
		name        string
		expressions []string
	}{{"branches", i.Branches}, {"tags", i.Tags}, {"paths", i.Paths}} {
		for _, expression := range field.expressions {
			if _, err := patterns.Compile(expression, patterns.IgnoreCase); err != nil {
				return fmt.Errorf("invalid regular expression in ignore.%s", field.name)
			}
		}
	}
	return nil
}

func unique(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range values {
		if !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}
	return out
}
func (i IgnoreSettings) WithBranches(values ...string) IgnoreSettings {
	i.Branches = unique(values)
	return i
}
func (i IgnoreSettings) WithTags(values ...string) IgnoreSettings { i.Tags = unique(values); return i }
func matchIgnore(expressions []string, value string) (bool, error) {
	for _, expression := range expressions {
		p, err := patterns.Compile(expression, patterns.IgnoreCase)
		if err != nil {
			return false, err
		}
		m, err := p.Find(value)
		if err != nil {
			return false, err
		}
		if m != nil {
			return true, nil
		}
	}
	return false, nil
}
func (i IgnoreSettings) BranchIgnored(name string) (bool, error) {
	name = strings.TrimPrefix(name, "refs/heads/")
	if strings.HasPrefix(name, "refs/remotes/") {
		name = strings.TrimPrefix(name, "refs/remotes/")
		if slash := strings.IndexByte(name, '/'); slash >= 0 {
			name = name[slash+1:]
		}
	}
	return matchIgnore(i.Branches, name)
}
func (i IgnoreSettings) TagIgnored(name string) (bool, error) { return matchIgnore(i.Tags, name) }
func SerializeIgnore(i IgnoreSettings) ([]byte, error) {
	out := map[string]any{}
	if i.Before != nil {
		out["commits-before"] = i.Before.Format("2006-01-02T15:04:05Z")
	}
	for k, v := range map[string][]string{"sha": i.SHAs, "paths": i.Paths, "branches": i.Branches, "tags": i.Tags} {
		if len(v) > 0 {
			out[k] = v
		}
	}
	return yaml.Marshal(out)
}

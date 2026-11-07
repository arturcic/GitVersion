package config

import (
	"fmt"
	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
	"gopkg.in/yaml.v3"
	"time"
)

type IgnoreSettings struct {
	SHAs, Paths []string
	Before      *time.Time
}

func (i IgnoreSettings) Empty() bool { return len(i.SHAs) == 0 && len(i.Paths) == 0 && i.Before == nil }
func parseIgnore(n *yaml.Node) (IgnoreSettings, error) {
	var out IgnoreSettings
	fields, err := mapping(n, "calculation.ignore", []string{"sha", "commits-before", "paths"})
	if err != nil {
		return out, err
	}
	for _, key := range []string{"sha", "paths"} {
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
					return out, fmt.Errorf("invalid regular expression in ignore.paths")
				}
				out.Paths = append(out.Paths, s)
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
		if err != nil {
			return out, fmt.Errorf("invalid ignore.commits-before date")
		}
		out.Before = &value
	}
	return out, nil
}

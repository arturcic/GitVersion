package config

import (
	"fmt"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"regexp"
)

func boolp(v bool) *bool                        { return &v }
func incp(v semver.Increment) *semver.Increment { return &v }
func (c Config) Kinds() []string {
	kinds := []string{"main", "feature", "release", "pull-request", "unknown"}
	if c.Workflow == "GitFlow/v1" {
		kinds = append(kinds, "develop", "hotfix", "support")
	}
	return kinds
}
func (c Config) Options(kind string) Options {
	switch kind {
	case "main":
		return c.Main
	case "feature":
		return c.Feature
	default:
		return c.Extra[kind]
	}
}
func (c *Config) setOptions(kind string, o Options) {
	switch kind {
	case "main":
		c.Main = o
	case "feature":
		c.Feature = o
	default:
		c.Extra[kind] = o
	}
}
func (c *Config) initializeWorkflow() {
	c.Extra = map[string]Options{}
	c.Main.IsMain = boolp(true)
	c.Main.IsRelease = boolp(false)
	c.Main.PreventMerged = boolp(true)
	c.Main.TracksRelease = boolp(false)
	c.Main.TrackMergeTarget = boolp(false)
	c.Feature.IsMain = boolp(false)
	c.Extra["unknown"] = Options{Mode: Manual, Label: "{BranchName}", IsMain: boolp(false)}
	if c.Workflow == "GitFlow/v1" {
		o := c.Extra["unknown"]
		o.PreventTagged = true
		c.Extra["unknown"] = o
	} else {
		o := c.Extra["unknown"]
		o.TrackMergeMessage = boolp(false)
		c.Extra["unknown"] = o
	}
	releaseInc := semver.Patch
	if c.Workflow == "GitFlow/v1" {
		releaseInc = semver.Minor
	}
	c.Extra["release"] = Options{Mode: Manual, Label: "beta", Increment: incp(releaseInc), Weight: 30000, IsMain: boolp(false), IsRelease: boolp(true), PreventMerged: boolp(true)}
	c.Extra["pull-request"] = Options{Mode: Delivery, Label: "PullRequest{Number}", Weight: 30000, PreventMerged: boolp(true)}
	if c.Workflow == "GitFlow/v1" {
		c.Extra["develop"] = Options{Mode: Delivery, Label: "alpha", Increment: incp(semver.Minor), IsMain: boolp(false), IsRelease: boolp(false), TracksRelease: boolp(true), TrackMergeTarget: boolp(true)}
		c.Extra["hotfix"] = Options{Mode: Manual, Label: "beta", Weight: 30000, IsMain: boolp(false), IsRelease: boolp(true)}
		c.Extra["support"] = Options{Mode: Delivery, Label: "", Increment: incp(semver.Patch), Weight: 55000, PreventTagged: true, IsMain: boolp(true), IsRelease: boolp(false), PreventMerged: boolp(true)}
	}
}

var workflowPatterns = map[string]*regexp.Regexp{
	"develop":      regexp.MustCompile(`^dev(elop)?(ment)?$`),
	"release":      regexp.MustCompile(`^releases?[/\\-](.+)`),
	"hotfix":       regexp.MustCompile(`^hotfix(es)?[/\\-](.+)`),
	"support":      regexp.MustCompile(`^support[/\\-](.+)`),
	"pull-request": regexp.MustCompile(`^(pull-requests|pull|pr)[/\\-]([0-9]*)`),
}

func (c Config) Branch(name string) (kind, capture string, err error) {
	if name == "main" || name == "master" {
		return "main", "", nil
	}
	if m := feature.FindStringSubmatch(name); m != nil {
		return "feature", Sanitize(m[1], false), nil
	}
	for _, k := range c.Kinds() {
		if pattern := workflowPatterns[k]; pattern != nil {
			if m := pattern.FindStringSubmatch(name); m != nil {
				capture = m[len(m)-1]
				if k == "develop" {
					capture = ""
				}
				return k, Sanitize(capture, false), nil
			}
		}
	}
	if name == "" {
		return "", "", fmt.Errorf("empty branch name")
	}
	return "unknown", Sanitize(name, false), nil
}
func (c Config) SourceKinds(kind string) []string {
	switch kind {
	case "unknown":
		kinds := c.Kinds()
		out := make([]string, 0, len(kinds)-1)
		for _, k := range kinds {
			if k != "unknown" {
				out = append(out, k)
			}
		}
		return out
	case "main":
		return nil
	case "develop", "support":
		return []string{"main"}
	case "release", "hotfix":
		return []string{"main", "support"}
	case "feature":
		if c.Workflow == "GitFlow/v1" {
			return []string{"develop", "main", "release", "support", "hotfix"}
		}
		return []string{"main", "release"}
	case "pull-request":
		if c.Workflow == "GitFlow/v1" {
			return []string{"develop", "main", "release", "feature", "support", "hotfix"}
		}
		return []string{"main", "release", "feature"}
	}
	return nil
}

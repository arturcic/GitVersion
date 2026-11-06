// Package config accepts the declared GitHubFlow/v1 and GitFlow/v1 settings,
// including explicit Mainline strategy selection.
// Unknown configuration is an error, including unknown deeply nested fields.
package config

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Workflow      string
	Strategies    map[string]bool
	Extra         map[string]Options
	Next          *semver.Version
	MainIncrement semver.Increment
	Main, Feature Options
	TagWeight     int64
}

type Mode string

const (
	Manual     Mode = "ManualDeployment"
	Delivery   Mode = "ContinuousDelivery"
	Deployment Mode = "ContinuousDeployment"
)

type Options struct {
	IsMain, IsRelease, TracksRelease, TrackMergeTarget, PreventMerged, PreventWhenMerged, TrackMergeMessage *bool
	Mode                                                                                                    Mode
	Label                                                                                                   string
	Increment                                                                                               *semver.Increment
	Weight                                                                                                  int64
	PreventTagged                                                                                           bool
	Messages                                                                                                *string
}
type Effective struct {
	Kind                                                                                                    string
	IsMain, IsRelease, TracksRelease, TrackMergeTarget, PreventMerged, PreventWhenMerged, TrackMergeMessage bool
	Mode                                                                                                    Mode
	Label                                                                                                   string
	Increment                                                                                               semver.Increment
	Weight, TagWeight                                                                                       int64
	PreventTagged, Inherit                                                                                  bool
	Messages                                                                                                string
}

func (c Config) Effective(branch string) (Effective, error) { return c.EffectiveFrom(branch, "main") }
func (c Config) EffectiveFrom(branch, source string) (Effective, error) {
	kind, capture, err := c.Branch(branch)
	if err != nil {
		return Effective{}, err
	}
	o := c.Options(kind)
	parentKind, _, err := c.Branch(source)
	if err != nil {
		return Effective{}, err
	}
	parent := c.Options(parentKind)
	e := Effective{Kind: kind, Mode: o.Mode, Label: Sanitize(strings.ReplaceAll(strings.ReplaceAll(o.Label, "{BranchName}", capture), "{Number}", capture), true), Increment: c.MainIncrement, Weight: o.Weight, TagWeight: c.TagWeight, PreventTagged: o.PreventTagged, Inherit: kind != "main" && o.Increment == nil, Messages: "Enabled"}
	if e.Inherit && parent.Increment != nil {
		e.Increment = *parent.Increment
	}
	if o.Increment != nil {
		e.Increment = *o.Increment
	}
	if e.Inherit && parent.Messages != nil {
		e.Messages = *parent.Messages
	}
	if o.Messages != nil {
		e.Messages = *o.Messages
	}
	value := func(child, ancestor *bool, fallback bool) bool {
		if child != nil {
			return *child
		}
		if e.Inherit && ancestor != nil {
			return *ancestor
		}
		return fallback
	}
	e.IsMain = value(o.IsMain, parent.IsMain, false)
	e.IsRelease = value(o.IsRelease, parent.IsRelease, false)
	e.TracksRelease = value(o.TracksRelease, parent.TracksRelease, false)
	e.TrackMergeTarget = value(o.TrackMergeTarget, parent.TrackMergeTarget, false)
	e.PreventMerged = value(o.PreventMerged, parent.PreventMerged, false)
	e.PreventWhenMerged = value(o.PreventWhenMerged, parent.PreventWhenMerged, false)
	e.TrackMergeMessage = value(o.TrackMergeMessage, parent.TrackMergeMessage, true)
	return e, nil
}

var feature = regexp.MustCompile(`^features?[/\\-](.+)$`)

func Branch(name string) (isMain bool, label string, err error) {
	if name == "main" || name == "master" {
		return true, "", nil
	}
	m := feature.FindStringSubmatch(name)
	if m == nil {
		return false, "", fmt.Errorf("unsupported branch %q: require main/master or feature branch", name)
	}
	// Branch-name captures are sanitized as names before label substitution.
	// Literal label formats permit dots, but the frozen {BranchName} capture does not.
	return false, Sanitize(m[1], false), nil
}

// .NET regex operates on UTF-16 units: a supplementary character becomes two
// dashes. Labels and build metadata permit dots; escaped branch names do not.
func Sanitize(s string, dots bool) string {
	var b strings.Builder
	for _, r := range utf16.Encode([]rune(s)) {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || (dots && r == '.') {
			b.WriteByte(byte(r))
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}
func Load(repo string) (Config, error) {
	var path string
	for _, name := range []string{"GitVersion.yml", "GitVersion.yaml", ".GitVersion.yml", ".GitVersion.yaml"} {
		p := filepath.Join(repo, name)
		info, err := os.Lstat(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return Config{}, err
		}
		if !info.Mode().IsRegular() {
			return Config{}, fmt.Errorf("unsupported nonregular config %s", name)
		}
		if path != "" {
			return Config{}, fmt.Errorf("ambiguous configuration files")
		}
		if info.Size() > 1<<20 {
			return Config{}, fmt.Errorf("configuration exceeds 1 MiB")
		}
		path = p
	}
	if path == "" {
		return Config{}, fmt.Errorf("explicit GitHubFlow/v1 configuration required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(b)
}
func Parse(data []byte) (Config, error) {
	c := Config{Strategies: map[string]bool{"Fallback": true, "ConfiguredNextVersion": true, "MergeMessage": true, "TaggedCommit": true, "TrackReleaseBranches": true, "VersionInBranchName": true}, MainIncrement: semver.Patch, TagWeight: 60000, Main: Options{Mode: Delivery, Label: "", Weight: 55000, PreventTagged: true}, Feature: Options{Mode: Manual, Label: "{BranchName}", Weight: 30000}}
	if len(data) > 1<<20 {
		return c, fmt.Errorf("configuration exceeds 1 MiB")
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := d.Decode(&doc); err != nil {
		return c, err
	}
	var extra yaml.Node
	if err := d.Decode(&extra); err != io.EOF {
		return c, fmt.Errorf("multiple YAML documents or malformed trailing data")
	}
	if len(doc.Content) != 1 {
		return c, fmt.Errorf("empty configuration")
	}
	root, err := mapping(doc.Content[0], "root", []string{"workflow", "calculation", "output"})
	if err != nil {
		return c, err
	}
	workflow, err := scalar(root["workflow"], "workflow")
	if err != nil {
		return c, err
	}
	if workflow != "GitHubFlow/v1" && workflow != "GitFlow/v1" {
		return c, fmt.Errorf("unsupported workflow %q", workflow)
	}
	c.Workflow = workflow
	c.initializeWorkflow()
	if node := root["calculation"]; node != nil {
		calc, err := mapping(node, "calculation", []string{"next-version", "branches", "strategies"})
		if err != nil {
			return c, err
		}
		if n := calc["strategies"]; n != nil {
			if n.Kind != yaml.SequenceNode || n.Anchor != "" {
				return c, fmt.Errorf("strategies requires a list")
			}
			c.Strategies = map[string]bool{}
			for _, v := range n.Content {
				name, err := scalar(v, "strategy")
				if err != nil {
					return c, err
				}
				switch name {
				case "None", "Fallback", "ConfiguredNextVersion", "MergeMessage", "TaggedCommit", "TrackReleaseBranches", "VersionInBranchName", "Mainline":
					c.Strategies[name] = true
				default:
					return c, fmt.Errorf("unknown strategy %q", name)
				}
			}
		}
		if n := calc["next-version"]; n != nil {
			s, err := scalar(n, "next-version")
			if err != nil {
				return c, err
			}
			v, err := semver.Parse(s)
			if err != nil {
				return c, err
			}
			c.Next = &v
		}
		if n := calc["branches"]; n != nil {
			bs, err := mapping(n, "branches", c.Kinds())
			if err != nil {
				return c, err
			}
			for _, kind := range c.Kinds() {
				if n := bs[kind]; n != nil {
					o := c.Options(kind)
					if err := parseOptions(n, kind, &o); err != nil {
						return c, err
					}
					c.setOptions(kind, o)
				}
			}
			if c.Main.Increment != nil {
				c.MainIncrement = *c.Main.Increment
			}
		}
	}
	if n := root["output"]; n != nil {
		out, err := mapping(n, "output", []string{"tag-pre-release-weight", "branches"})
		if err != nil {
			return c, err
		}
		if n := out["tag-pre-release-weight"]; n != nil {
			c.TagWeight, err = integer(n, "tag-pre-release-weight")
			if err != nil {
				return c, err
			}
		}
		if n := out["branches"]; n != nil {
			bs, err := mapping(n, "output.branches", c.Kinds())
			if err != nil {
				return c, err
			}
			for _, kind := range c.Kinds() {
				if n := bs[kind]; n != nil {
					b, err := mapping(n, "output.branches."+kind, []string{"pre-release-weight"})
					if err != nil {
						return c, err
					}
					if n := b["pre-release-weight"]; n != nil {
						w, err := integer(n, "pre-release-weight")
						if err != nil {
							return c, err
						}
						o := c.Options(kind)
						o.Weight = w
						c.setOptions(kind, o)
					}
				}
			}
		}
	}
	return c, nil
}

func parseOptions(n *yaml.Node, kind string, o *Options) error {
	b, err := mapping(n, kind, []string{"mode", "label", "increment", "prevent-increment", "commit-message-incrementing", "is-main-branch", "is-release-branch", "tracks-release-branches", "track-merge-target", "track-merge-message"})
	if err != nil {
		return err
	}
	for _, key := range []string{"mode", "label", "increment", "commit-message-incrementing"} {
		if n := b[key]; n != nil {
			s, err := scalar(n, key)
			if err != nil {
				return err
			}
			switch key {
			case "mode":
				if Mode(s) != Manual && Mode(s) != Delivery && Mode(s) != Deployment {
					return fmt.Errorf("unsupported deployment mode %q", s)
				}
				o.Mode = Mode(s)
			case "label":
				literal := strings.ReplaceAll(s, "{BranchName}", "")
				if kind == "pull-request" {
					literal = strings.ReplaceAll(literal, "{Number}", "")
				}
				if strings.ContainsAny(literal, "{}") || strings.Contains(s, "env:") || ((kind == "main" || kind == "develop") && literal != s) {
					return fmt.Errorf("unsupported label template %q", s)
				}
				o.Label = s
			case "increment":
				if s == "Inherit" && kind != "main" {
					o.Increment = nil
					continue
				}
				i, err := semver.ParseIncrement(s)
				if err != nil {
					return err
				}
				o.Increment = &i
			case "commit-message-incrementing":
				if s != "Enabled" && s != "Disabled" && s != "MergeMessageOnly" {
					return fmt.Errorf("unsupported commit-message-incrementing %q", s)
				}
				o.Messages = &s
			}
		}
	}
	for key, target := range map[string]**bool{"is-main-branch": &o.IsMain, "is-release-branch": &o.IsRelease, "tracks-release-branches": &o.TracksRelease, "track-merge-target": &o.TrackMergeTarget, "track-merge-message": &o.TrackMergeMessage} {
		if n := b[key]; n != nil {
			v, err := boolean(n, key)
			if err != nil {
				return err
			}
			*target = &v
		}
	}
	if n := b["prevent-increment"]; n != nil {
		p, err := mapping(n, "prevent-increment", []string{"when-current-commit-tagged", "of-merged-branch", "when-branch-merged"})
		if err != nil {
			return err
		}
		for key, target := range map[string]**bool{"of-merged-branch": &o.PreventMerged, "when-branch-merged": &o.PreventWhenMerged} {
			if n := p[key]; n != nil {
				v, err := boolean(n, key)
				if err != nil {
					return err
				}
				*target = &v
			}
		}
		if n := p["when-current-commit-tagged"]; n != nil {
			v, err := boolean(n, "when-current-commit-tagged")
			if err != nil {
				return err
			}
			o.PreventTagged = v
		}
	}
	return nil
}

var decimal = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

func integer(n *yaml.Node, path string) (int64, error) {
	if n.Kind != yaml.ScalarNode || n.Tag != "!!int" || n.Anchor != "" || !decimal.MatchString(n.Value) {
		return 0, fmt.Errorf("%s requires a nonnegative decimal integer", path)
	}
	v, err := strconv.ParseInt(n.Value, 10, 64)
	if err != nil || v > math.MaxInt32 {
		return 0, fmt.Errorf("unsupported %s range", path)
	}
	return v, nil
}
func scalar(n *yaml.Node, path string) (string, error) {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag != "!!str" || n.Anchor != "" {
		return "", fmt.Errorf("%s requires a plain string; aliases and anchors are unsupported", path)
	}
	return n.Value, nil
}
func mapping(n *yaml.Node, path string, allowed []string) (map[string]*yaml.Node, error) {
	if n.Kind != yaml.MappingNode || n.Tag != "!!map" || n.Anchor != "" {
		return nil, fmt.Errorf("%s requires a mapping; aliases and anchors are unsupported", path)
	}
	result := map[string]*yaml.Node{}
	for i := 0; i < len(n.Content); i += 2 {
		k, err := scalar(n.Content[i], path+" key")
		if err != nil {
			return nil, err
		}
		ok := false
		for _, a := range allowed {
			if a == k {
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("unsupported configuration field %s.%s", path, k)
		}
		if _, ok := result[k]; ok {
			return nil, fmt.Errorf("duplicate configuration field %s.%s", path, k)
		}
		result[k] = n.Content[i+1]
	}
	return result, nil
}

func boolean(n *yaml.Node, path string) (bool, error) {
	if n.Kind != yaml.ScalarNode || n.Tag != "!!bool" || n.Anchor != "" {
		return false, fmt.Errorf("%s requires a boolean", path)
	}
	return strconv.ParseBool(n.Value)
}

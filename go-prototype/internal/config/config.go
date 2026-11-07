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
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Output            OutputSettings
	Ignore            IgnoreSettings
	Workflow          string
	Strategies        map[string]bool
	Extra             map[string]Options
	Next              *semver.Version
	MainIncrement     semver.Increment
	FallbackIncrement *semver.Increment
	Main, Feature     Options
	DefaultMode       Mode
	TagWeight         int64
	branchOrder       []string
	branchRegex       map[string]string
	branchPatterns    map[string]*patterns.Pattern
	branchSources     map[string][]string
}

type Mode string

const (
	Manual     Mode = "ManualDeployment"
	Delivery   Mode = "ContinuousDelivery"
	Deployment Mode = "ContinuousDeployment"
)

type Options struct {
	CustomVersionFormat *string
	// NullLabel retains an explicit null from programmatic builders. YAML parsing
	// currently accepts explicit string labels only.
	NullLabel                                                                                               bool
	InheritPreventTagged, InheritWeight                                                                     bool
	IsMain, IsRelease, TracksRelease, TrackMergeTarget, PreventMerged, PreventWhenMerged, TrackMergeMessage *bool
	Mode                                                                                                    Mode
	Label                                                                                                   string
	Increment                                                                                               *semver.Increment
	Weight                                                                                                  int64
	PreventTagged                                                                                           bool
	Messages                                                                                                *string
}
type Effective struct {
	NullLabel                                                                                               bool
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
	return c.EffectiveAt(branch, source, "")
}

func (c Config) EffectiveAt(branch, source, sha string) (Effective, error) {
	kind, capture, err := c.Branch(branch)
	if err != nil {
		return Effective{}, err
	}
	options := c.DeclaredOptions(kind)
	inherited := options.Increment == nil
	if inherited {
		parent, _, err := c.Branch(source)
		if err != nil {
			return Effective{}, err
		}
		options = options.Inherit(c.DeclaredOptions(parent))
	}
	return c.projectEffective(branch, kind, capture, sha, options, inherited)
}

func (c Config) projectEffective(branch, kind, capture, sha string, o Options, inherited bool) (Effective, error) {
	shortSHA := sha
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	label := o.Label
	if !o.NullLabel {
		if c.branchRegex != nil {
			expression := c.branchRegex[kind]
			resolved, err := (LabelConfiguration{Label: &o.Label, Regex: &expression}).resolve(ptrBranch(normalizedBranchName(branch)), nil, os.LookupEnv, &LabelCommit{SHA: sha, ShortSHA: shortSHA}, c.branchPatterns[kind])
			if err != nil {
				return Effective{}, err
			}
			label = *resolved
		} else {
			label = strings.NewReplacer("{BranchName}", capture, "{Number}", capture, "{Sha}", sha, "{ShortSha}", shortSHA).Replace(label)
			label = Sanitize(label, true)
		}
	}
	mode := o.Mode
	if mode == "" {
		mode = c.DefaultMode
		if mode == "" {
			mode = Delivery
		}
	}
	increment := c.MainIncrement
	if c.FallbackIncrement != nil {
		increment = *c.FallbackIncrement
	}
	if o.Increment != nil {
		increment = *o.Increment
	}
	messages := "Enabled"
	if o.Messages != nil {
		messages = *o.Messages
	}
	value := func(p *bool, fallback bool) bool {
		if p != nil {
			return *p
		}
		return fallback
	}
	return Effective{Kind: kind, NullLabel: o.NullLabel, Label: label, Mode: mode, Increment: increment, Weight: o.Weight, TagWeight: c.TagWeight,
		PreventTagged: o.PreventTagged, Inherit: inherited, Messages: messages, IsMain: value(o.IsMain, false), IsRelease: value(o.IsRelease, false),
		TracksRelease: value(o.TracksRelease, false), TrackMergeTarget: value(o.TrackMergeTarget, false), PreventMerged: value(o.PreventMerged, false),
		PreventWhenMerged: value(o.PreventWhenMerged, false), TrackMergeMessage: value(o.TrackMergeMessage, true)}, nil
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
func Load(repo string) (Config, error) { return LoadNamed(repo, "") }

func Parse(data []byte) (Config, error) {
	c := Config{Strategies: map[string]bool{"Fallback": true, "ConfiguredNextVersion": true, "MergeMessage": true, "TaggedCommit": true, "TrackReleaseBranches": true, "VersionInBranchName": true}, MainIncrement: semver.Patch, TagWeight: 60000, Main: Options{Label: "", Weight: 55000, PreventTagged: true}, Feature: Options{Mode: Manual, Label: "{BranchName}", Weight: 30000}}
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
	if _, err := normalizeDocument(doc.Content[0], "v7", "configuration"); err != nil {
		return c, err
	}
	root, err := mapping(doc.Content[0], "root", []string{"workflow", "calculation", "output"})
	if err != nil {
		return c, err
	}
	workflow, err := scalar(root["workflow"], "workflow")
	if err != nil {
		return c, err
	}
	if workflow != "GitHubFlow/v1" && workflow != "GitFlow/v1" && workflow != "TrunkBased/preview1" {
		return c, fmt.Errorf("unsupported workflow %q", workflow)
	}
	c.Workflow = workflow
	c.DefaultMode = Delivery
	c.initializeWorkflow()
	if err := c.initializeBranchRules(); err != nil {
		return c, err
	}
	if node := root["calculation"]; node != nil {
		calc, err := mapping(node, "calculation", []string{"mode", "increment", "next-version", "semantic-version-format", "branches", "strategies", "ignore"})
		if err != nil {
			return c, err
		}
		if n := calc["ignore"]; n != nil {
			c.Ignore, err = parseIgnore(n)
			if err != nil {
				return c, err
			}
		}
		if n := calc["increment"]; n != nil {
			value, err := scalar(n, "calculation.increment")
			if err != nil {
				return c, err
			}
			if value != "Inherit" {
				increment, err := semver.ParseIncrement(value)
				if err != nil {
					return c, err
				}
				c.FallbackIncrement = &increment
			}
		}
		if n := calc["mode"]; n != nil {
			s, err := scalar(n, "calculation.mode")
			if err != nil {
				return c, err
			}
			if s != string(Manual) && s != string(Delivery) && s != string(Deployment) {
				return c, fmt.Errorf("unsupported deployment mode %q", s)
			}
			c.DefaultMode = Mode(s)
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
		versionFormat := semver.Strict
		if n := calc["semantic-version-format"]; n != nil {
			format, err := scalar(n, "semantic-version-format")
			if err != nil {
				return c, err
			}
			versionFormat = semver.ParseMode(format)
			if versionFormat != semver.Strict && versionFormat != semver.Loose {
				return c, validation("VALUE", "calculation.semantic-version-format", "Expected Strict or Loose.")
			}
		}
		if n := calc["next-version"]; n != nil && n.Tag != "!!null" {
			s, err := scalar(n, "next-version")
			if err != nil {
				return c, err
			}
			if s != "" {
				var v semver.Version
				if versionFormat == semver.Loose {
					v, err = semver.ParseDetailed(s, nil, versionFormat)
					if err == nil {
						// Check the normalized version against the calculator's numeric limits.
						_, err = semver.Parse(v.String())
					}
				} else {
					v, err = semver.Parse(s)
				}
				if err != nil {
					return c, validation("VALUE", "calculation.next-version", "Expected a semantic version.")
				}
				c.Next = &v
			}
		}
		if n := calc["branches"]; n != nil {
			bs, err := mapping(n, "branches", nil)
			if err != nil {
				return c, err
			}
			for i := 0; i < len(n.Content); i += 2 {
				kind := n.Content[i].Value
				if n := bs[kind]; n != nil {
					o := c.DeclaredOptions(kind)
					if _, exists := c.branchRegex[kind]; !exists {
						o = Options{Label: "{BranchName}", PreventTagged: true, InheritPreventTagged: true, InheritWeight: true}
						c.branchOrder = append(c.branchOrder, kind)
					}
					if err := c.parseBranchRules(n, kind); err != nil {
						return c, err
					}
					if err := parseOptions(n, kind, c.branchRegex[kind], &o); err != nil {
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
	for kind, sources := range c.branchSources {
		for _, source := range sources {
			if _, exists := c.branchRegex[source]; !exists {
				return c, fmt.Errorf("branches.%s.source-branches: referenced branch configuration %q does not exist", kind, source)
			}
		}
	}
	if n := root["output"]; n != nil {
		out, err := mapping(n, "output", append([]string{"tag-pre-release-weight", "branches", "update-build-number"}, outputSettingNames...))
		if err != nil {
			return c, err
		}
		if setting := out["update-build-number"]; setting != nil {
			if _, err := boolean(setting, "output.update-build-number"); err != nil {
				return c, err
			}
		}
		if err := parseOutputSettings(out, &c.Output); err != nil {
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
					b, err := mapping(n, "output.branches."+kind, []string{"pre-release-weight", "custom-version-format"})
					if err != nil {
						return c, err
					}
					o := c.DeclaredOptions(kind)
					if n := b["custom-version-format"]; n != nil && n.Tag != "!!null" {
						value, err := scalar(n, "output.branches."+kind+".custom-version-format")
						if err != nil {
							return c, err
						}
						o.CustomVersionFormat = &value
					}
					if n := b["pre-release-weight"]; n != nil {
						w, err := integer(n, "pre-release-weight")
						if err != nil {
							return c, err
						}
						o.Weight = w
						o.InheritWeight = false
					}
					c.setOptions(kind, o)
				}
			}
		}
	}
	return c, nil
}

func parseOptions(n *yaml.Node, kind, expression string, o *Options) error {
	b, err := mapping(n, kind, []string{"mode", "label", "increment", "prevent-increment", "commit-message-incrementing", "is-main-branch", "is-release-branch", "tracks-release-branches", "track-merge-target", "track-merge-message", "regex", "source-branches"})
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
				literal := strings.NewReplacer("{Sha}", "", "{ShortSha}", "").Replace(s)
				pattern, err := patterns.Compile(expression, patterns.IgnoreCase)
				if err != nil {
					return err
				}
				for _, name := range pattern.GroupNames() {
					literal = strings.ReplaceAll(literal, "{"+name+"}", "")
				}
				if strings.ContainsAny(literal, "{}") || strings.Contains(s, "env:") {
					return fmt.Errorf("unsupported label template %q", s)
				}
				o.Label = s
			case "increment":
				if s == "Inherit" {
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
			o.InheritPreventTagged = false
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
		ok := allowed == nil
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

package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/formatting"
	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
)

// LabelConfiguration is the effective label portion of a selected branch's
// configuration. Selection and version calculation remain separate operations.
type LabelConfiguration struct{ Label, Regex *string }
type LabelCommit struct{ SHA, ShortSHA string }

func (c LabelConfiguration) Resolve(branch, override *string, environment formatting.EnvironmentLookup, commit *LabelCommit) (*string, error) {
	if environment == nil {
		return nil, fmt.Errorf("environment cannot be null")
	}
	if commit == nil {
		return nil, fmt.Errorf("current commit cannot be null")
	}
	if c.Label == nil {
		return nil, nil
	}
	effective := branch
	if override != nil {
		effective = override
	}
	placeholders := map[string]any{}
	if c.Regex != nil && strings.TrimSpace(*c.Regex) != "" && effective != nil && *effective != "" {
		regex, err := patterns.Compile(*c.Regex, patterns.IgnoreCase)
		if err != nil {
			return nil, err
		}
		match, err := regex.Find(*effective)
		if err != nil {
			return nil, err
		}
		if match != nil {
			for name, group := range match.Groups {
				if _, err := strconv.ParseInt(name, 10, 32); err != nil {
					placeholders[name] = Sanitize(group.Value, false)
				}
			}
		}
	}
	if _, ok := placeholders["Sha"]; !ok {
		placeholders["Sha"] = commit.SHA
	}
	if _, ok := placeholders["ShortSha"]; !ok {
		placeholders["ShortSha"] = commit.ShortSHA
	}
	result, err := formatting.FormatWith(*c.Label, placeholders, environment)
	if err != nil {
		return nil, err
	}
	result = Sanitize(result, true)
	return &result, nil
}

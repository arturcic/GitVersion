package calculate

import (
	"fmt"
	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
	"strings"
	"time"
)

// FilterCommit is the immutable commit view used by version-source filters.
// Paths contains the commit's diff paths, not its entire tree.
type FilterCommit struct {
	SHA   string
	When  time.Time
	Paths []string
}
type BaseVersionSource struct{ Commit *FilterCommit }
type VersionFilter interface {
	ExcludeVersion(*BaseVersionSource) (bool, *string, error)
	ExcludeCommit(*FilterCommit) (bool, *string, error)
}
type MinDateVersionFilter struct{ Minimum time.Time }

func (f MinDateVersionFilter) ExcludeVersion(v *BaseVersionSource) (bool, *string, error) {
	if v == nil {
		return false, nil, fmt.Errorf("baseVersion cannot be null")
	}
	return f.ExcludeCommit(v.Commit)
}
func (f MinDateVersionFilter) ExcludeCommit(c *FilterCommit) (bool, *string, error) {
	if c == nil || !c.When.Before(f.Minimum) {
		return false, nil, nil
	}
	reason := "Source was ignored due to commit date being outside of configured range"
	return true, &reason, nil
}

type ShaVersionFilter struct{ prefixes []string }

func NewShaVersionFilter(prefixes []string) (ShaVersionFilter, error) {
	if prefixes == nil {
		return ShaVersionFilter{}, fmt.Errorf("shaList cannot be null")
	}
	return ShaVersionFilter{append([]string{}, prefixes...)}, nil
}
func (f ShaVersionFilter) ExcludeVersion(v *BaseVersionSource) (bool, *string, error) {
	if v == nil {
		return false, nil, fmt.Errorf("baseVersion cannot be null")
	}
	return f.ExcludeCommit(v.Commit)
}
func (f ShaVersionFilter) ExcludeCommit(c *FilterCommit) (bool, *string, error) {
	if c == nil {
		return false, nil, nil
	}
	for _, prefix := range f.prefixes {
		if strings.HasPrefix(strings.ToLower(c.SHA), strings.ToLower(prefix)) {
			reason := "Sha " + c.SHA + " was ignored due to commit having been excluded by configuration"
			return true, &reason, nil
		}
	}
	return false, nil, nil
}

type PathFilter struct{ expressions []*patterns.Pattern }

func NewPathFilter(paths []string) (PathFilter, error) {
	out := PathFilter{}
	for _, path := range paths {
		p, err := patterns.Compile(path, patterns.IgnoreCase)
		if err != nil {
			return PathFilter{}, err
		}
		out.expressions = append(out.expressions, p)
	}
	return out, nil
}
func (f PathFilter) ExcludeVersion(v *BaseVersionSource) (bool, *string, error) {
	if v == nil {
		return false, nil, fmt.Errorf("baseVersion cannot be null")
	}
	return f.ExcludeCommit(v.Commit)
}
func (f PathFilter) ExcludeCommit(c *FilterCommit) (bool, *string, error) {
	if c == nil {
		return false, nil, nil
	}
	for _, path := range c.Paths {
		matched := false
		for _, expression := range f.expressions {
			m, err := expression.Find(path)
			if err != nil {
				return false, nil, err
			}
			if m != nil {
				matched = true
				break
			}
		}
		if !matched {
			return false, nil, nil
		}
	}
	reason := "Source was ignored due to all commit paths matching ignore regex"
	return true, &reason, nil
}

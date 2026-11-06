// Package semver implements the declared GitVersion subset, including its
// prerelease-number increment rules (which differ from ordinary SemVer bumping).
package semver

import (
	"cmp"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type Increment int

const (
	None Increment = iota
	Patch
	Minor
	Major
)

func (i Increment) String() string {
	switch i {
	case None:
		return "None"
	case Patch:
		return "Patch"
	case Minor:
		return "Minor"
	case Major:
		return "Major"
	}
	return "Invalid"
}
func ParseIncrement(s string) (Increment, error) {
	for _, v := range []Increment{None, Patch, Minor, Major} {
		if s == v.String() {
			return v, nil
		}
	}
	return None, fmt.Errorf("unsupported increment %q", s)
}

type Version struct {
	Major, Minor, Patch int64
	Label               string
	Number              *int64
}

var strict = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-((?:0|[1-9][0-9]*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
var suffix = regexp.MustCompile(`^(.*?)\.?([0-9]+)?$`)

// Parse accepts strict three-component versions. Metadata-bearing semantic tags
// are recognized but rejected explicitly until their formatting is supported.
func Parse(s string) (Version, error) {
	m := strict.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("invalid semantic version %q", s)
	}
	if m[5] != "" {
		return Version{}, fmt.Errorf("unsupported build metadata in semantic version %q", s)
	}
	v := Version{}
	for i, p := range []*int64{&v.Major, &v.Minor, &v.Patch} {
		n, err := strconv.ParseInt(m[i+1], 10, 64)
		if err != nil || n > math.MaxInt32 {
			return v, fmt.Errorf("unsupported version component range")
		}
		*p = n
	}
	if m[4] != "" {
		p := suffix.FindStringSubmatch(m[4])
		v.Label = p[1]
		if strings.HasSuffix(v.Label, "-") {
			v.Label = m[4]
		} else if p[2] != "" {
			n, err := strconv.ParseInt(p[2], 10, 64)
			if err != nil || n > math.MaxInt32-60000 {
				return v, fmt.Errorf("unsupported prerelease number range")
			}
			v.Number = &n
		}
	}
	return v, nil
}

// ParseTag separates irrelevant non-version tags from recognized but unsupported
// semantic versions; the latter must never silently disappear from calculation.
func ParseTag(s string) (Version, bool, error) {
	if strings.HasPrefix(s, "v") || strings.HasPrefix(s, "V") {
		s = s[1:]
	}
	if !strict.MatchString(s) {
		return Version{}, false, nil
	}
	v, err := Parse(s)
	return v, true, err
}
func (v Version) IsPreRelease() bool { return v.Label != "" || v.Number != nil }
func (v Version) Matches(label string) bool {
	return !v.IsPreRelease() || strings.EqualFold(v.Label, label)
}
func (v Version) Core() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }
func (v Version) PreRelease() string {
	if v.Number == nil {
		return v.Label
	}
	if v.Label == "" {
		return strconv.FormatInt(*v.Number, 10)
	}
	return v.Label + "." + strconv.FormatInt(*v.Number, 10)
}
func (v Version) String() string {
	if p := v.PreRelease(); p != "" {
		return v.Core() + "-" + p
	}
	return v.Core()
}
func (v Version) CompareCore(w Version) int {
	if c := cmp.Compare(v.Major, w.Major); c != 0 {
		return c
	}
	if c := cmp.Compare(v.Minor, w.Minor); c != 0 {
		return c
	}
	return cmp.Compare(v.Patch, w.Patch)
}
func (v Version) Compare(w Version) int {
	if c := v.CompareCore(w); c != 0 {
		return c
	}
	if v.IsPreRelease() != w.IsPreRelease() {
		if v.IsPreRelease() {
			return -1
		}
		return 1
	}
	if c := strings.Compare(strings.ToLower(v.Label), strings.ToLower(w.Label)); c != 0 {
		return c
	}
	if v.Number == nil {
		if w.Number == nil {
			return 0
		}
		return -1
	}
	if w.Number == nil {
		return 1
	}
	return cmp.Compare(*v.Number, *w.Number)
}
func (v Version) Bump(i Increment, label string, force bool) (Version, error) {
	if i < None || i > Major {
		return Version{}, fmt.Errorf("invalid increment")
	}
	out := v
	if i == None || (v.IsPreRelease() && !force) {
		if v.Number != nil {
			n := *v.Number + 1
			out.Number = &n
		}
	} else {
		switch i {
		case Patch:
			out.Patch++
		case Minor:
			out.Minor++
			out.Patch = 0
		case Major:
			out.Major++
			out.Minor = 0
			out.Patch = 0
		}
		if out.Number != nil {
			n := int64(1)
			out.Number = &n
		}
	}
	if !v.IsPreRelease() || out.Label != label {
		n := int64(1)
		out.Number = &n
		out.Label = label
	}
	if out.Major > math.MaxInt32 || out.Minor > math.MaxInt32 || out.Patch > math.MaxInt32 || (out.Number != nil && *out.Number > math.MaxInt32-60000) {
		return Version{}, fmt.Errorf("unsupported incremented version range")
	}
	return out, nil
}

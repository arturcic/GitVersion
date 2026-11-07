// Package patterns provides the pinned GitVersion built-in regex contracts.
// Text is matched as UTF-16 code units, as in .NET. The engine's Unicode casing
// follows Go rather than culture-sensitive .NET casing; that remains a parity
// boundary for custom patterns, not a claim of complete regex compatibility.
package patterns

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/dlclark/regexp2/v2"
)

type Options = regexp2.RegexOptions

const (
	IgnoreCase              = regexp2.IgnoreCase
	Multiline               = regexp2.Multiline
	Singleline              = regexp2.Singleline
	IgnorePatternWhitespace = regexp2.IgnorePatternWhitespace
)

type Pattern struct{ re *regexp2.Regexp }
type Group struct {
	Value    string
	Captures []string
}
type Match struct {
	Value  string
	Groups map[string]Group
}

func (m Match) Group(name string) (string, bool) {
	g, ok := m.Groups[name]
	return g.Value, ok && len(g.Captures) > 0
}

// Compile bounds backtracking time before a pattern is shared with any callers.
func Compile(pattern string, options Options) (*Pattern, error) {
	var text strings.Builder
	for _, r := range pattern {
		if r > 0xffff {
			for _, u := range utf16.Encode([]rune{r}) {
				fmt.Fprintf(&text, `\u%04X`, u)
			}
		} else {
			text.WriteRune(r)
		}
	}
	re, err := regexp2.Compile(text.String(), options)
	if err != nil {
		return nil, err
	}
	re.MatchTimeout = 2 * time.Second
	return &Pattern{re: re}, nil
}
func units(s string) []rune {
	us := utf16.Encode([]rune(s))
	rs := make([]rune, len(us))
	for i, u := range us {
		rs[i] = rune(u)
	}
	return rs
}
func decode(rs []rune) string {
	us := make([]uint16, len(rs))
	for i, r := range rs {
		us[i] = uint16(r)
	}
	return string(utf16.Decode(us))
}
func (p *Pattern) result(m *regexp2.Match) Match {
	result := Match{Value: decode(m.Runes()), Groups: map[string]Group{}}
	for _, g := range m.Groups() {
		group := Group{Value: decode(g.Runes()), Captures: []string{}}
		for _, capture := range g.Captures {
			group.Captures = append(group.Captures, decode(capture.Runes()))
		}
		result.Groups[g.Name] = group
		result.Groups[strconv.Itoa(p.re.GroupNumberFromName(g.Name))] = group
	}
	return result
}
func (p *Pattern) Find(s string) (*Match, error) {
	m, err := p.re.FindRunesMatch(units(s))
	if err != nil || m == nil {
		return nil, err
	}
	result := p.result(m)
	return &result, nil
}
func (p *Pattern) FindAll(s string) ([]Match, error) {
	out := []Match{}
	m, err := p.re.FindRunesMatch(units(s))
	for err == nil && m != nil {
		out = append(out, p.result(m))
		m, err = p.re.FindNextMatch(m)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

//go:embed builtins.json
var builtins []byte
var compiled = sync.OnceValue(func() map[string]*Pattern {
	var definitions map[string]struct {
		Pattern string   `json:"pattern"`
		Options []string `json:"options"`
	}
	if err := json.Unmarshal(builtins, &definitions); err != nil {
		panic(err)
	}
	out := map[string]*Pattern{}
	for name, d := range definitions {
		var options Options
		for _, o := range d.Options {
			switch o {
			case "IgnoreCase":
				options |= IgnoreCase
			case "Multiline":
				options |= Multiline
			case "Singleline":
				options |= Singleline
			case "IgnorePatternWhitespace":
				options |= IgnorePatternWhitespace
			default:
				panic("invalid built-in regex option " + o)
			}
		}
		p, err := Compile(d.Pattern, options)
		if err != nil {
			panic(fmt.Sprintf("invalid built-in %s: %v", name, err))
		}
		out[name] = p
	}
	return out
})

func Builtin(name string) (*Pattern, error) {
	p, ok := compiled()[name]
	if !ok {
		return nil, fmt.Errorf("unknown built-in pattern %q", name)
	}
	return p, nil
}

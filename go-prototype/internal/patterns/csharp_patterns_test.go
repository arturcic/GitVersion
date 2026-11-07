package patterns

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestCSharpRegexPatterns(t *testing.T) {
	var corpus struct {
		Revision string `json:"revision"`
		Methods  map[string][]struct {
			ID, Pattern, Operation, Input, Group string
			Match                                bool
			Capture                              *string
			Captures                             []string
		} `json:"methods"`
	}
	data, err := os.ReadFile("../../testdata/csharp-regex-patterns.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Methods) != 35 {
		t.Fatal("source inventory differs")
	}
	names := []string{}
	total := 0
	for name, cases := range corpus.Methods {
		names = append(names, name)
		total += len(cases)
	}
	sort.Strings(names)
	if total != 122 {
		t.Fatal("parameter inventory differs")
	}
	for _, method := range names {
		t.Run(method, func(t *testing.T) {
			for _, tc := range corpus.Methods[method] {
				t.Run(tc.ID, func(t *testing.T) {
					p, err := Builtin(tc.Pattern)
					if err != nil {
						t.Fatal(err)
					}
					if tc.Operation == "all" {
						matches, err := p.FindAll(tc.Input)
						if err != nil {
							t.Fatal(err)
						}
						got := []string{}
						for _, m := range matches {
							got = append(got, m.Value)
						}
						if (len(matches) > 0) != tc.Match || !reflect.DeepEqual(got, tc.Captures) {
							t.Fatalf("matches: want %v, got %v", tc.Captures, got)
						}
						return
					}
					m, err := p.Find(tc.Input)
					if err != nil {
						t.Fatal(err)
					}
					if (m != nil) != tc.Match {
						t.Fatalf("match: want %t, got %+v", tc.Match, m)
					}
					if m == nil {
						return
					}
					if tc.Operation == "group" && tc.Group != "" {
						got, ok := m.Group(tc.Group)
						if !ok || tc.Capture == nil || got != *tc.Capture {
							t.Fatalf("group %s: got %q present=%t", tc.Group, got, ok)
						}
					} else if tc.Capture != nil && m.Value != *tc.Capture {
						t.Fatalf("capture: want %q, got %q", *tc.Capture, m.Value)
					}
				})
			}
		})
	}
}

func TestUTF16PatternCapturesAndFailures(t *testing.T) {
	p, err := Compile(`^(?<value>..)$`, 0)
	if err != nil {
		t.Fatal(err)
	}
	m, err := p.Find("😀")
	if err != nil || m == nil {
		t.Fatal("UTF16 match missing", err)
	}
	value, ok := m.Group("value")
	if !ok || value != "😀" || m.Value != "😀" {
		t.Fatal("supplementary capture changed", m)
	}
	p, err = Compile(`^😀$`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if m, err := p.Find("😀"); err != nil || m == nil || m.Value != "😀" {
		t.Fatal("supplementary literal changed", m, err)
	}
	p, err = Compile(``, 0)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := p.FindAll("ab")
	if err != nil || len(matches) != 3 {
		t.Fatal("empty matches must terminate", len(matches), err)
	}
	if _, err := Compile(`[`, 0); err == nil {
		t.Fatal("invalid pattern accepted")
	}
	if _, err := Builtin("missing"); err == nil {
		t.Fatal("missing pattern accepted")
	}
}

package calculate

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCSharpVersionFilters(t *testing.T) {
	for _, file := range []string{"min-date-version-filter", "sha-version-filter", "path-filter"} {
		var corpus struct {
			Methods map[string][]struct {
				ID, Filter, Minimum string
				Commit              *struct {
					SHA, When string
					Paths     []string
				}
				Prefixes, Paths []string
				NullVersion     bool `json:"null_version"`
				Error, Excluded bool
				ReasonNonempty  bool `json:"reason_nonempty"`
			}
		}
		raw, err := os.ReadFile("../../testdata/csharp-" + file + ".json")
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &corpus); err != nil {
			t.Fatal(err)
		}
		if len(corpus.Methods) != 4 {
			t.Fatal("original method inventory changed")
		}
		for method, cases := range corpus.Methods {
			t.Run(file+"/"+method, func(t *testing.T) {
				for _, c := range cases {
					t.Run(c.ID, func(t *testing.T) {
						var f VersionFilter
						switch c.Filter {
						case "MinDateVersionFilter":
							minimum, e := time.Parse(time.RFC3339, c.Minimum)
							if e != nil {
								t.Fatal(e)
							}
							f = MinDateVersionFilter{Minimum: minimum}
						case "ShaVersionFilter":
							value, e := NewShaVersionFilter(c.Prefixes)
							if e != nil {
								t.Fatal(e)
							}
							f = value
						case "PathFilter":
							value, e := NewPathFilter(c.Paths)
							if e != nil {
								t.Fatal(e)
							}
							f = value
						default:
							t.Fatal(c.Filter)
						}
						var version *BaseVersionSource
						if !c.NullVersion {
							version = &BaseVersionSource{}
							if c.Commit != nil {
								when, e := time.Parse(time.RFC3339, c.Commit.When)
								if e != nil {
									t.Fatal(e)
								}
								version.Commit = &FilterCommit{SHA: c.Commit.SHA, When: when, Paths: c.Commit.Paths}
							}
						}
						excluded, reason, e := f.ExcludeVersion(version)
						if (e != nil) != c.Error {
							t.Fatal("error", e)
						}
						if c.Error {
							return
						}
						if excluded != c.Excluded {
							t.Fatalf("exclude: got %t want %t", excluded, c.Excluded)
						}
						if c.ReasonNonempty {
							if reason == nil || strings.TrimSpace(*reason) == "" {
								t.Fatal("missing exclusion reason")
							}
						} else if reason != nil {
							t.Fatal("reason should be null", *reason)
						}
					})
				}
			})
		}
	}
}
func TestVersionFilterBoundaries(t *testing.T) {
	instant := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, offset := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
		t.Run("date-"+offset.String(), func(t *testing.T) {
			got, reason, err := (MinDateVersionFilter{instant}).ExcludeCommit(&FilterCommit{When: instant.Add(offset)})
			if err != nil || got != (offset < 0) || (reason != nil) != got {
				t.Fatal(got, reason, err)
			}
		})
	}
	for _, c := range []struct {
		name     string
		prefixes []string
		sha      string
		want     bool
	}{{"short-insensitive", []string{"ABc"}, "abc123", true}, {"no-substring", []string{"bc"}, "abc123", false}, {"empty-prefix", []string{""}, "abc", true}, {"empty-list", []string{}, "abc", false}, {"prefix-longer", []string{"abc1234"}, "abc", false}} {
		t.Run("sha-"+c.name, func(t *testing.T) {
			f, err := NewShaVersionFilter(c.prefixes)
			if err != nil {
				t.Fatal(err)
			}
			got, reason, err := f.ExcludeCommit(&FilterCommit{SHA: c.sha})
			if err != nil || got != c.want || (reason != nil) != got {
				t.Fatal(got, reason, err)
			}
		})
	}
	t.Run("null-prefix-list", func(t *testing.T) {
		if _, err := NewShaVersionFilter(nil); err == nil {
			t.Fatal("accepted nil SHA list")
		}
	})
	for _, c := range []struct {
		name            string
		patterns, paths []string
		want            bool
	}{{"all-different-patterns", []string{"^docs/", "[.]md$"}, []string{"docs/a.txt", "README.md"}, true}, {"one-unmatched", []string{"^docs/"}, []string{"docs/a", "src/a"}, false}, {"empty-diff", []string{}, []string{}, true}, {"empty-patterns", []string{}, []string{"a"}, false}, {"insensitive", []string{"^DOCS/"}, []string{"docs/a"}, true}, {"case-override", []string{"(?-i)^DOCS/"}, []string{"docs/a"}, false}} {
		t.Run("path-"+c.name, func(t *testing.T) {
			f, err := NewPathFilter(c.patterns)
			if err != nil {
				t.Fatal(err)
			}
			got, reason, err := f.ExcludeCommit(&FilterCommit{Paths: c.paths})
			if err != nil || got != c.want || (reason != nil) != got {
				t.Fatal(got, reason, err)
			}
		})
	}
	t.Run("invalid-pattern", func(t *testing.T) {
		if _, err := NewPathFilter([]string{"["}); err == nil {
			t.Fatal("accepted invalid pattern")
		}
	})
}

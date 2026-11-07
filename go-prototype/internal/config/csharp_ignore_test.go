package config

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestCSharpIgnoreConfiguration(t *testing.T) {
	var corpus struct {
		Methods map[string][]struct {
			ID, Operation, YAML, Field, Contains string
			NotContains                          string `json:"not_contains"`
			Values                               []string
			ContainsAll                          []string `json:"contains_all"`
			Error                                bool
			Expected                             map[string]any
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-ignore-configuration.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Methods) != 18 {
		t.Fatal("original inventory changed")
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, c := range cases {
				t.Run(c.ID, func(t *testing.T) {
					var settings IgnoreSettings
					strategies := map[string]bool{}
					var err error
					if c.Operation == "decode" {
						doc, e := ReadLegacyDocument([]byte(c.YAML))
						err = e
						settings = doc.Ignore
						strategies = doc.Strategies
					} else {
						if c.Field == "Branches" {
							settings = settings.WithBranches(c.Values...)
						} else {
							settings = settings.WithTags(c.Values...)
						}
						if c.Operation == "validate" {
							err = settings.Validate()
						}
					}
					if (err != nil) != c.Error {
						t.Fatal(err)
					}
					if c.Error {
						if c.Contains != "" && !strings.Contains(err.Error(), c.Contains) {
							t.Fatal(err)
						}
						if c.NotContains != "" && strings.Contains(err.Error(), c.NotContains) {
							t.Fatal("diagnostic leaked expression")
						}
						return
					}
					if c.Operation == "serialize" {
						out, e := SerializeIgnore(settings)
						if e != nil {
							t.Fatal(e)
						}
						for _, v := range c.ContainsAll {
							if !strings.Contains(string(out), v) {
								t.Fatal("missing serialized assertion", v, string(out))
							}
						}
					}
					for field, want := range c.Expected {
						if field == "IsEmpty" {
							if settings.Empty() != want.(bool) {
								t.Fatal("empty state", settings)
							}
							continue
						}
						if field == "Before" {
							if want == nil {
								if settings.Before != nil {
									t.Fatal("expected nil date")
								}
							} else {
								expected := time.Date(2015, 10, 23, 12, 23, 15, 0, time.Local)
								if settings.Before == nil || !settings.Before.Equal(expected) {
									t.Fatal("date", settings.Before)
								}
							}
							continue
						}
						var got []string
						switch field {
						case "Shas":
							got = settings.SHAs
						case "Paths":
							got = settings.Paths
						case "Branches":
							got = settings.Branches
						case "Tags":
							got = settings.Tags
						case "Strategies":
							for name, on := range strategies {
								if on {
									got = append(got, name)
								}
							}
							sort.Strings(got)
						default:
							t.Fatal("unhandled original assertion", field)
						}
						expected := []string{}
						for _, v := range want.([]any) {
							expected = append(expected, v.(string))
						}
						if field == "Strategies" {
							sort.Strings(expected)
						}
						if len(got) != len(expected) || len(got) > 0 && !reflect.DeepEqual(got, expected) {
							t.Fatalf("%s: got %v want %v", field, got, expected)
						}
					}
				})
			}
		})
	}
}

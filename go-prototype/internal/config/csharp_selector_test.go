package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCSharpConfigurationVersionSelector(t *testing.T) {
	var corpus struct {
		Methods map[string][]struct {
			ID       string
			Value    *string
			Expected *bool
			Error    bool
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-configuration-selector.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Methods) != 3 {
		t.Fatal("source method inventory changed")
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, c := range cases {
				t.Run(c.ID, func(t *testing.T) {
					if method == "IdentifiesExplicitV6Selection" {
						if IsExplicitV6(c.Value) != *c.Expected {
							t.Fatal("explicit selection differs")
						}
						return
					}
					version, err := SelectConfigurationVersion(c.Value)
					if (err != nil) != c.Error {
						t.Fatal(err)
					}
					if c.Error {
						for _, text := range []string{*c.Value, "v6", "v7"} {
							if !strings.Contains(err.Error(), text) {
								t.Fatal("missing original diagnostic", text, err)
							}
						}
						return
					}
					expected := "v7"
					if *c.Expected {
						expected = "v6"
					}
					if version != expected {
						t.Fatal(version, expected)
					}
				})
			}
		})
	}
}

package commandline

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func TestCSharpQuotedStringHelpers(t *testing.T) {
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct {
				Cases []struct {
					ID        string
					Arguments []json.RawMessage
				}
			}
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-argument-parsers.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	for method, item := range corpus.Suites["QuotedStringHelpersTests"].Methods {
		t.Run(method, func(t *testing.T) {
			for _, c := range item.Cases {
				t.Run(c.ID, func(t *testing.T) {
					var input string
					if err := json.Unmarshal(c.Arguments[0], &input); err != nil {
						t.Fatal(err)
					}
					if method == "UnquoteTextTests" {
						var expected string
						if err := json.Unmarshal(c.Arguments[1], &expected); err != nil {
							t.Fatal(err)
						}
						if got := UnquoteText(input); got != expected {
							t.Fatalf("%q != %q", got, expected)
						}
					} else {
						var separator string
						var expected []string
						if err := json.Unmarshal(c.Arguments[1], &separator); err != nil || len(separator) != 1 {
							t.Fatal("invalid separator", err)
						}
						if err := json.Unmarshal(c.Arguments[2], &expected); err != nil {
							t.Fatal(err)
						}
						if got := SplitUnquoted(input, separator[0]); !slices.Equal(got, expected) {
							t.Fatalf("%q != %q", got, expected)
						}
					}
				})
			}
		})
	}
}

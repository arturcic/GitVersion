package config

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
)

func TestCSharpConfigurationPorts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/csharp-configuration-ports.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []fixtures.ConfigurationPort
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 6 {
		t.Fatal("configuration port inventory differs")
	}
	defaults, err := Parse([]byte("workflow: GitFlow/v1\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			c, err := Parse([]byte(tc.Config))
			if err != nil {
				t.Fatal(err)
			}
			if len(tc.Expected) == 0 {
				t.Fatal("port has no assertions")
			}
			for path, want := range tc.Expected {
				var got any
				parts := strings.Split(path, ".")
				switch path {
				case "calculation.next-version":
					if c.Next == nil {
						t.Fatal("next-version is missing")
					}
					got = c.Next.String()
				case "calculation.strategies":
					names := []string{}
					for name, on := range c.Strategies {
						if on {
							names = append(names, name)
						}
					}
					sort.Strings(names)
					got = names
				default:
					if len(parts) != 4 || parts[1] != "branches" {
						t.Fatalf("unhandled assertion path %s", path)
					}
					o := c.Options(parts[2])
					switch parts[3] {
					case "increment":
						if o.Increment == nil {
							t.Fatal("increment is missing")
						}
						got = o.Increment.String()
					case "mode":
						got = string(o.Mode)
					case "label":
						got = o.Label
					case "source-branches":
						got = c.SourceKinds(parts[2])
					default:
						t.Fatalf("unhandled assertion path %s", path)
					}
				}
				actual, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				expected, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				if string(actual) != string(expected) {
					t.Errorf("%s: want %s, got %s", path, expected, actual)
				}
			}
			if tc.ID == "OverwritesDefaultsWithProvidedConfig" {
				if reflect.DeepEqual(c.Options("develop").Increment, defaults.Options("develop").Increment) {
					t.Error("override did not change default develop increment")
				}
				if c.Options("develop").Mode == defaults.Feature.Mode {
					t.Error("override has default feature deployment mode")
				}
			}
		})
	}
}

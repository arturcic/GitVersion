package output

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

func TestCSharpOutputProjections(t *testing.T) {
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct {
				Cases []struct {
					ID        string
					Arguments []any
				}
			}
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-wix-and-output.json", &corpus); err != nil {
		t.Fatal(err)
	}
	for _, suite := range []string{"CommitDateTests", "AssemblyFileVersionTests", "InformationalVersionBuilderTests"} {
		for method, item := range corpus.Suites[suite].Methods {
			t.Run(method, func(t *testing.T) {
				for _, tc := range item.Cases {
					t.Run(tc.ID, func(t *testing.T) {
						args := tc.Arguments
						number := func(index int) int64 { return int64(args[index].(float64)) }
						var input VariableInput
						var settings config.OutputSettings
						var key, expected string
						switch suite {
						case "CommitDateTests":
							format := args[0].(string)
							date := time.Date(2017, 10, 6, 0, 0, 0, 0, time.UTC)
							input.Date = &date
							settings.CommitDateFormat = &format
							key, expected = "CommitDate", args[1].(string)
						case "AssemblyFileVersionTests":
							n := number(4)
							input.Version = semver.Version{Major: number(1), Minor: number(2), Patch: number(3), Label: "Test", Number: &n}
							settings.AssemblyFileScheme = args[0].(string)
							key = "AssemblySemFileVer"
							expected, _ = args[5].(string)
						case "InformationalVersionBuilderTests":
							version := fmt.Sprintf("%d.%d.%d", number(3), number(4), number(5))
							if tag, ok := args[6].(string); ok {
								version += "-" + tag
							}
							var err error
							input.Version, err = semver.Parse(version)
							if err != nil {
								t.Fatal(err)
							}
							input.Branch, input.SHA, input.ShortSHA = args[0].(string), args[1].(string), args[2].(string)
							input.CountSHA, _ = args[7].(string)
							if args[8] != nil {
								input.Distance = number(8)
								input.Build = &input.Distance
							}
							key, expected = "InformationalVersion", args[9].(string)
						}
						got, err := FromSemanticVersion(input, settings, os.LookupEnv)
						if err != nil || got[key] != expected {
							t.Fatalf("%s = %q, want %q, error %v", key, got[key], expected, err)
						}
					})
				}
			})
		}
	}
}

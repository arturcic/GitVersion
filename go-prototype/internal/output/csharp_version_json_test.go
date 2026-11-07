package output

import (
	"os"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

func TestCSharpVersionJSON(t *testing.T) {
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct {
				Cases []struct{ ID, Approved string }
			}
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-msbuild-output.json", &corpus); err != nil {
		t.Fatal(err)
	}
	t.Run("Json", func(t *testing.T) {
		for _, tc := range corpus.Suites["JsonVersionBuilderTests"].Methods["Json"].Cases {
			t.Run(tc.ID, func(t *testing.T) {
				v, err := semver.Parse("1.2.0-unstable4")
				if err != nil {
					t.Fatal(err)
				}
				legacySource := semver.Version{Major: 1, Minor: 1, Patch: 2}
				count := int64(5)
				date := time.Date(2014, 3, 6, 23, 59, 59, 0, time.UTC)
				values, err := FromSemanticVersion(VariableInput{
					Version: v, LegacySource: &legacySource, LegacyIncrement: semver.Minor, CountSHA: "versionSourceSha",
					Branch: "feature1", SHA: "commitSha", ShortSHA: "commitShortSha", Date: &date, Distance: count, Build: &count,
				}, config.OutputSettings{}, os.LookupEnv)
				if err != nil {
					t.Fatal(err)
				}
				actual, err := MarshalVariables(values)
				if err != nil || string(actual) != tc.Approved {
					t.Fatalf("version JSON differs from pinned approval: %v\n%s", err, actual)
				}
			})
		}
	})
}

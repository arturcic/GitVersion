package calculate

import (
	"path/filepath"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
)

func TestResetOverridesOlderMessagesAndConfiguredFloor(t *testing.T) {
	for _, tc := range []struct {
		name, message       string
		major, minor, patch int64
	}{
		{"patch", "=semver: patch", 1, 2, 4},
		{"none", "=semver: none", 1, 2, 3},
		{"minor", "=semver: minor", 1, 3, 0},
		{"major", "=semver: major", 2, 0, 0},
		{"skip-precedence", "+semver: major\n=semver: skip", 1, 2, 3},
		{"unrecognized", "=semver: unknown", 2, 0, 0},
		{"marker-with-bump", "=semver: unknown\n+semver: patch", 1, 2, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := fixtures.Recipe{ID: "reset", Config: "workflow: GitHubFlow/v1\ncalculation: {branches: {main: {increment: Major}}}\n", Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}, {Op: "commit", ID: "old", Message: "+semver: major"}, {Op: "commit", ID: "reset", Message: tc.message}, {Op: "checkpoint", ID: "head"}}}
			cps, err := fixtures.Generate([]fixtures.Recipe{r}, filepath.Join(t.TempDir(), "fixtures"))
			if err != nil {
				t.Fatal(err)
			}
			got, _, err := Calculate(cps[0].Path)
			if err != nil {
				t.Fatal(err)
			}
			if got["Major"] != tc.major || got["Minor"] != tc.minor || got["Patch"] != tc.patch || got["CommitCountSourceDistance"] != int64(2) {
				t.Fatalf("reset result: %+v", got)
			}
		})
	}
}

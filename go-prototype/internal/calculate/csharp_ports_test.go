package calculate

import (
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingMainDoesNotBypassSourceRequirements(t *testing.T) {
	for _, tc := range []struct{ name, branch, extra, want string }{
		{"inherited-orphan", "feature/orphan", "", "no base versions determined from source branches"},
		{"mainline-inherited-orphan", "feature/orphan", "calculation: {strategies: [Mainline]}\n", "no base versions determined from source branches"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := fixtures.Recipe{ID: tc.name, InitialBranch: tc.branch, Config: "workflow: GitFlow/v1\n" + tc.extra, Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "head"}}}
			cps, err := fixtures.Generate([]fixtures.Recipe{r}, filepath.Join(t.TempDir(), "fixtures"))
			if err != nil {
				t.Fatal(err)
			}
			before, err := fixtures.State(cps[0].Path)
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = Calculate(cps[0].Path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %s, got %v", tc.want, err)
			}
			after, err := fixtures.State(cps[0].Path)
			if err != nil || after != before {
				t.Fatal("rejection changed repository state", err)
			}
		})
	}
}

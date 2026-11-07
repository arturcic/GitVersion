package calculate

import (
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"testing"
)

func TestBranchVersionExtractionRequiresStrictVersion(t *testing.T) {
	for _, tc := range []struct{ branch, want string }{
		{"hotfix/4795-reads-tag-from-other-branch", ""},
		{"release/7.0", ""}, {"release/7", ""}, {"release/7.0.0", "7.0.0"},
		{"release-7.0.0", "7.0.0"}, {"release/42/v7.0.0-extra", "7.0.0"},
		{"release//7.0.0", ""}, {"release/01.2.3", ""},
	} {
		t.Run(tc.branch, func(t *testing.T) {
			v, ok := versionInBranch(tc.branch)
			if ok != (tc.want != "") || ok && v.String() != tc.want {
				t.Fatalf("want %q got %v/%t", tc.want, v, ok)
			}
		})
	}
}
func TestUnknownBranchUsesWorkflowDefaults(t *testing.T) {
	for _, workflow := range []string{"GitFlow/v1", "GitHubFlow/v1"} {
		t.Run(workflow, func(t *testing.T) {
			c, err := config.Parse([]byte("workflow: " + workflow + "\n"))
			if err != nil {
				t.Fatal(err)
			}
			e, err := c.Effective("codex/topic")
			if err != nil {
				t.Fatal(err)
			}
			if e.Kind != "unknown" || e.Label != "codex-topic" || e.Mode != config.Manual || e.PreventTagged != (workflow == "GitFlow/v1") || !e.Inherit {
				t.Fatal("unknown branch defaults", e)
			}
		})
	}
}

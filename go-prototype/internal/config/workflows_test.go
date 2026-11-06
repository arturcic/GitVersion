package config

import (
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"testing"
)

func TestWorkflowDefaultsAndInheritance(t *testing.T) {
	for _, tc := range []struct {
		flow, branch, source, label string
		inc                         semver.Increment
		main, release, tracks       bool
		weight                      int64
	}{
		{"GitFlow/v1", "develop", "main", "alpha", semver.Minor, false, false, true, 0},
		{"GitFlow/v1", "feature/topic", "develop", "topic", semver.Minor, false, false, true, 30000},
		{"GitFlow/v1", "release/1.3.0", "main", "beta", semver.Minor, false, true, false, 30000},
		{"GitFlow/v1", "hotfix/1.2.4", "main", "beta", semver.Patch, false, true, false, 30000},
		{"GitFlow/v1", "support/1.x", "main", "", semver.Patch, true, false, false, 55000},
		{"GitFlow/v1", "pull/42/merge", "develop", "PullRequest42", semver.Minor, false, false, true, 30000},
		{"GitHubFlow/v1", "pull/42/merge", "main", "PullRequest42", semver.Patch, true, false, false, 30000},
	} {
		t.Run(tc.flow+"/"+tc.branch, func(t *testing.T) {
			c, err := Parse([]byte("workflow: " + tc.flow + "\n"))
			if err != nil {
				t.Fatal(err)
			}
			e, err := c.EffectiveFrom(tc.branch, tc.source)
			if err != nil {
				t.Fatal(err)
			}
			if e.Label != tc.label || e.Increment != tc.inc || e.IsMain != tc.main || e.IsRelease != tc.release || e.TracksRelease != tc.tracks || e.Weight != tc.weight {
				t.Fatalf("workflow defaults %+v", e)
			}
		})
	}
}
func TestWorkflowOverridesAndStrategyValidation(t *testing.T) {
	c, err := Parse([]byte("workflow: GitFlow/v1\ncalculation: {strategies: [ConfiguredNextVersion, Mainline], branches: {develop: {mode: ManualDeployment, increment: Patch, label: preview}}}\noutput: {branches: {develop: {pre-release-weight: 17}}}\n"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := c.Effective("develop")
	if err != nil || !c.Mainline || !c.MainlineNext || e.Mode != Manual || e.Increment != semver.Patch || e.Label != "preview" || e.Weight != 17 || c.Feature.Weight != 30000 {
		t.Fatalf("override isolation %+v, %v", e, err)
	}
	for _, value := range []string{"[]", "[Mainline, Mainline]", "Mainline", "[Other]", "[ConfiguredNextVersion]", "[null]"} {
		if _, err := Parse([]byte("workflow: GitFlow/v1\ncalculation: {strategies: " + value + "}\n")); err == nil {
			t.Fatalf("invalid strategies accepted: %s", value)
		}
	}
}

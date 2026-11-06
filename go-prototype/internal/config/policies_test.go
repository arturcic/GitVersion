package config

import (
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"testing"
)

func TestExpandedPolicyEffectiveSettings(t *testing.T) {
	data := `workflow: GitHubFlow/v1
calculation:
  branches:
    main:
      increment: Major
      commit-message-incrementing: Disabled
    feature:
      mode: ContinuousDelivery
      label: 'preview.{BranchName}'
      prevent-increment:
        when-current-commit-tagged: true
output:
  tag-pre-release-weight: 0
  branches:
    feature:
      pre-release-weight: 0
`
	c, err := Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	e, err := c.Effective("feature/a.b😀")
	if err != nil {
		t.Fatal(err)
	}
	if e.Mode != Delivery || e.Label != "preview.a-b--" || e.Increment != semver.Major || e.Weight != 0 || e.TagWeight != 0 || !e.PreventTagged || !e.Inherit || e.Messages != "Disabled" {
		t.Fatalf("incorrect effective settings %+v", e)
	}
	patch := semver.Patch
	c.Feature.Increment = &patch
	e, err = c.Effective("feature/topic")
	if err != nil {
		t.Fatal(err)
	}
	if e.Inherit || e.Increment != semver.Patch || e.Messages != "Enabled" {
		t.Fatalf("explicit increment must stop source-branch inheritance: %+v", e)
	}
	c, err = Parse([]byte("workflow: GitHubFlow/v1\ncalculation: {branches: {main: {label: ''}, feature: {label: '', increment: Inherit, prevent-increment: {when-current-commit-tagged: false}}}}\n"))
	if err != nil {
		t.Fatal(err)
	}
	e, err = c.Effective("feature/topic")
	if err != nil || e.Label != "" || e.PreventTagged || !e.Inherit {
		t.Fatalf("lost empty/false/inherit: %+v %v", e, err)
	}
}

func TestExpandedPolicyRejectsInvalidValues(t *testing.T) {
	for _, tc := range []struct{ name, section string }{
		{"main-inherit", "calculation: {branches: {main: {increment: Inherit}}}"},
		{"mode", "calculation: {branches: {feature: {mode: Mainline}}}"},
		{"message-mode", "calculation: {branches: {feature: {commit-message-incrementing: Sometimes}}}"},
		{"string-bool", "calculation: {branches: {main: {prevent-increment: {when-current-commit-tagged: 'false'}}}}"},
		{"null-bool", "calculation: {branches: {main: {prevent-increment: {when-current-commit-tagged: null}}}}"},
		{"unknown-prevent", "calculation: {branches: {main: {prevent-increment: {of-merged-branch: true}}}}"},
		{"main-capture", "calculation: {branches: {main: {label: '{BranchName}'}}}"},
		{"unknown-placeholder", "calculation: {branches: {feature: {label: '{env:BRANCH}'}}}"},
		{"number-label", "calculation: {branches: {feature: {label: 42}}}"},
		{"negative-weight", "output: {tag-pre-release-weight: -1}"},
		{"overflow-weight", "output: {tag-pre-release-weight: 2147483648}"},
		{"string-weight", "output: {tag-pre-release-weight: '12'}"},
		{"float-weight", "output: {tag-pre-release-weight: 1.5}"},
		{"hex-weight", "output: {tag-pre-release-weight: 0x10}"},
		{"misplaced-weight", "calculation: {branches: {main: {pre-release-weight: 42}}}"},
		{"misplaced-mode", "output: {branches: {main: {mode: ManualDeployment}}}"},
		{"duplicate-weight", "output: {tag-pre-release-weight: 0, tag-pre-release-weight: 1}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte("workflow: GitHubFlow/v1\n" + tc.section)); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
	for _, s := range []string{"0", "1", "2147483647"} {
		if _, err := Parse([]byte("workflow: GitHubFlow/v1\noutput: {tag-pre-release-weight: " + s + "}")); err != nil {
			t.Fatalf("valid boundary %s: %v", s, err)
		}
	}
}

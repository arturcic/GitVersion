package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestStrategySelection(t *testing.T) {
	names := []string{"Fallback", "ConfiguredNextVersion", "MergeMessage", "TaggedCommit", "TrackReleaseBranches", "VersionInBranchName", "Mainline"}
	for mask := 0; mask < 128; mask++ {
		t.Run(fmt.Sprintf("subset-%03d", mask), func(t *testing.T) {
			var selected []string
			for i, n := range names {
				if mask&(1<<i) != 0 {
					selected = append(selected, n)
				}
			}
			c, err := Parse([]byte("workflow: GitFlow/v1\ncalculation: {strategies: [" + strings.Join(selected, ",") + "]}"))
			if err != nil {
				t.Fatal(err)
			}
			for i, n := range names {
				if c.Strategies[n] != (mask&(1<<i) != 0) {
					t.Fatalf("strategy %s selection changed", n)
				}
			}
		})
	}
	for _, value := range []string{"[None]", "[None, TaggedCommit]", "[Mainline, Mainline]", "[TaggedCommit, Mainline]"} {
		if _, err := Parse([]byte("workflow: GitHubFlow/v1\ncalculation: {strategies: " + value + "}")); err != nil {
			t.Fatal(value, err)
		}
	}
}

func TestBranchControlOverrides(t *testing.T) {
	for _, value := range []bool{false, true} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			c, err := Parse([]byte(fmt.Sprintf("workflow: GitFlow/v1\ncalculation:\n  branches:\n    feature:\n      is-main-branch: %t\n      is-release-branch: %t\n      tracks-release-branches: %t\n      track-merge-target: %t\n      track-merge-message: %t\n      prevent-increment: {of-merged-branch: %t, when-branch-merged: %t}\n", value, value, value, value, value, value, value)))
			if err != nil {
				t.Fatal(err)
			}
			e, err := c.EffectiveFrom("feature/topic", "develop")
			if err != nil {
				t.Fatal(err)
			}
			if e.IsMain != value || e.IsRelease != value || e.TracksRelease != value || e.TrackMergeTarget != value || e.TrackMergeMessage != value || e.PreventMerged != value || e.PreventWhenMerged != value {
				t.Fatalf("explicit child controls lost: %+v", e)
			}
			parent, err := c.Effective("develop")
			if err != nil || parent.IsMain || !parent.TracksRelease || !parent.TrackMergeTarget {
				t.Fatalf("parent modified: %+v %v", parent, err)
			}
		})
	}
	for _, key := range []string{"is-main-branch", "is-release-branch", "tracks-release-branches", "track-merge-target", "track-merge-message"} {
		for _, value := range []string{"null", "'true'", "1", "[]", "{}"} {
			if _, err := Parse([]byte("workflow: GitFlow/v1\ncalculation: {branches: {main: {" + key + ": " + value + "}}}")); err == nil {
				t.Fatal("invalid control accepted", key, value)
			}
		}
	}
}

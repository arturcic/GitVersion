package calculate

import (
	"errors"
	"reflect"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

func inheritanceConfig(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.Parse([]byte(`workflow: GitFlow/v1
calculation:
  branches:
    custom:
      regex: '^custom/(?<BranchName>.+)'
      source-branches: [feature, main]
`))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestBranchFinderRetainsIntermediateSettings(t *testing.T) {
	cfg := inheritanceConfig(t)
	child := cfg.Extra["custom"]
	child.NullLabel = true
	cfg.Extra["custom"] = child
	cfg.Main.Mode = config.Deployment
	cfg.Main.Label = "stable"
	yes, no := true, false
	cfg.Main.TrackMergeTarget = &yes
	cfg.Main.PreventMerged = &yes
	cfg.Main.TrackMergeMessage = &yes
	cfg.Feature.Label = "preview"
	cfg.Feature.TrackMergeTarget = &no
	cfg.Feature.IsRelease = &yes
	cfg.Feature.TracksRelease = &yes
	cfg.Feature.PreventMerged = &no
	cfg.Feature.PreventWhenMerged = &yes
	cfg.Feature.TrackMergeMessage = &no
	messages, format := "Disabled", "{SemVer}-parent"
	cfg.Feature.Messages = &messages
	cfg.Feature.CustomVersionFormat = &format
	graph := map[string][]string{"custom/topic": {"feature/parent"}, "feature/parent": {"main"}}
	finder := branchConfigurationFinder{config: cfg, sources: func(branch string, _ map[string]bool) ([]string, error) { return graph[branch], nil }}
	got, err := finder.configurations("custom/topic", "")
	if err != nil {
		t.Fatal(err)
	}
	want := config.Effective{Kind: "custom", Mode: config.Manual, Label: "preview", Increment: semver.Patch, Weight: 30000, TagWeight: 60000, Inherit: true, IsRelease: true, TracksRelease: true, PreventWhenMerged: true, Messages: "Disabled"}
	if len(got) != 1 || got[0].branch != "main" || !reflect.DeepEqual(got[0].value, want) {
		t.Fatalf("recursive source/settings: want main %+v, got %+v", want, got)
	}
	if got[0].options.CustomVersionFormat == nil || *got[0].options.CustomVersionFormat != format {
		t.Fatal("intermediate custom version format lost")
	}
}

func TestBranchFinderKeepsExplicitChildValues(t *testing.T) {
	cfg := inheritanceConfig(t)
	o := cfg.Extra["custom"]
	inc := semver.None
	o.Increment = &inc
	o.Mode = config.Delivery
	o.Label = ""
	o.InheritWeight = false
	o.PreventTagged = false
	o.InheritPreventTagged = false
	cfg.Extra["custom"] = o
	finder := branchConfigurationFinder{config: cfg, sources: func(string, map[string]bool) ([]string, error) {
		return nil, errors.New("explicit increment must stop source traversal")
	}}
	got, err := finder.configurations("custom/topic", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].branch != "custom/topic" {
		t.Fatalf("explicit leaf differs: %+v", got)
	}
	e := got[0].value
	if e.Increment != semver.None || e.Mode != config.Delivery || e.Label != "" || e.NullLabel || e.Weight != 0 || e.PreventTagged || e.Inherit {
		t.Fatalf("explicit zero/empty settings lost: %+v", e)
	}
}

func TestBranchFinderBreaksCyclesWithoutLeakingSettings(t *testing.T) {
	for _, name := range []string{"cycle-only", "valid-sibling"} {
		t.Run(name, func(t *testing.T) {
			cfg := inheritanceConfig(t)
			fallback := semver.Major
			cfg.FallbackIncrement = &fallback
			graph := map[string][]string{"custom/topic": {"feature/parent"}, "feature/parent": {"custom/topic"}}
			if name == "valid-sibling" {
				graph["custom/topic"] = append(graph["custom/topic"], "main")
			}
			f := branchConfigurationFinder{config: cfg, sources: func(b string, _ map[string]bool) ([]string, error) { return graph[b], nil }}
			got, err := f.configurations("custom/topic", "")
			if err != nil {
				t.Fatal(err)
			}
			if name == "cycle-only" {
				if len(got) != 0 {
					t.Fatalf("cycle produced a fallback configuration: %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].branch != "main" || got[0].value.Mode != config.Delivery || got[0].value.Increment != semver.Patch {
				t.Fatalf("cycle changed independent source settings: %+v", got)
			}
		})
	}
}

func TestBranchFinderPropagatesLookupErrors(t *testing.T) {
	for _, name := range []string{"source-candidates", "pull-request-tip"} {
		t.Run(name, func(t *testing.T) {
			cause := errors.New("repository read failed")
			f := branchConfigurationFinder{config: inheritanceConfig(t), sources: func(string, map[string]bool) ([]string, error) {
				if name == "source-candidates" {
					return nil, cause
				}
				return []string{"main"}, nil
			}, commit: func(string) (string, bool, error) { return "", false, cause }}
			if _, err := f.configurations("pull/2/merge", ""); !errors.Is(err, cause) {
				t.Fatalf("lookup error lost: %v", err)
			}
		})
	}
}

func TestContextualPullRequestTargetPrefersFetchedRemote(t *testing.T) {
	const local = "refs/heads/support/v1.0.x"
	const remote = "refs/remotes/origin/support/v1.0.x"
	finder := branchConfigurationFinder{
		config: inheritanceConfig(t),
		branches: []branchRef{
			{name: local, kind: "support"},
			{name: "refs/remotes/other/support/v1.0.x", kind: "support"},
			{name: remote, kind: "support"},
		},
		commit: func(string) (string, bool, error) {
			return "Merge pull request 2 from hotfix/v1.0.2 into support/v1.0.x", true, nil
		},
	}
	for _, tc := range []struct{ fetched, want string }{{"", local}, {"origin", remote}, {"missing", local}} {
		finder.fetchedRemote = tc.fetched
		got, err := finder.pullRequestTarget("refs/pull/2/merge")
		if err != nil || got != tc.want {
			t.Errorf("fetched %q: got %q, want %q (%v)", tc.fetched, got, tc.want, err)
		}
	}
}

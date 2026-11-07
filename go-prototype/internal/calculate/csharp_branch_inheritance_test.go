package calculate

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

func TestCSharpEffectiveBranchConfigurationFinder(t *testing.T) {
	var corpus struct {
		Revision string
		Methods  map[string][]struct {
			ID        string
			Arguments []string
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-branch-inheritance.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Methods) != 17 {
		t.Fatal("original source inventory differs")
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.ID, func(t *testing.T) {
					cfg, err := config.Parse([]byte("workflow: GitFlow/v1\n"))
					if err != nil {
						t.Fatal(err)
					}
					branch := "develop"
					graph := map[string][]string{}
					branches := []branchRef{}
					message := ""
					wantBranches := []string{branch}
					var wantMode *config.Mode
					var wantLabel *string
					var wantIncrement *semver.Increment
					increment := func(value string) *semver.Increment {
						if value == "Inherit" {
							return nil
						}
						v, err := semver.ParseIncrement(value)
						if err != nil {
							t.Fatal(err)
						}
						return &v
					}
					mode := func(value string) *config.Mode { v := config.Mode(value); return &v }
					label := func(value string) *string { return &value }
					setIncrement := func(kind, value string) { o := cfg.Extra[kind]; o.Increment = increment(value); cfg.Extra[kind] = o }
					switch method {
					case "When_getting_configurations_of_a_branch_without_versioning_mode_Given_fallback_configuration_with_versioning_mode_Then_result_should_have_versioning_mode":
						branch = "main"
						wantBranches = []string{branch}
						cfg.DefaultMode = config.Mode(tc.Arguments[0])
						cfg.Main.Mode = ""
						wantMode = mode(tc.Arguments[0])
					case "When_getting_configurations_of_a_branch_with_versioning_mode_Given_fallback_configuration_without_versioning_mode_Then_result_should_have_versioning_mode":
						cfg.DefaultMode = ""
						cfg.Main.Mode = config.Mode(tc.Arguments[0])
						o := cfg.Extra["develop"]
						o.Mode = ""
						o.Increment = nil
						cfg.Extra["develop"] = o
						graph[branch] = []string{"main"}
						wantBranches = []string{"main"}
						wantMode = mode(tc.Arguments[0])
					case "When_getting_configurations_of_a_branch_with_versioning_mode_Given_parent_configuration_with_versioning_mode_Then_result_should_not_have_versioning_mode_of_parent":
						cfg.DefaultMode = ""
						cfg.Main.Mode = config.Mode(tc.Arguments[0])
						o := cfg.Extra["develop"]
						o.Mode = config.Delivery
						o.Increment = nil
						cfg.Extra["develop"] = o
						graph[branch] = []string{"main"}
						wantBranches = []string{"main"}
						wantMode = mode("ContinuousDelivery")
					case "When_getting_configurations_of_a_branch_with_tag_alpha_Given_branch_which_inherits_from_parent_branch_Then_result_should_have_tag_alpha":
						cfg.Main.Label = ""
						o := cfg.Extra["develop"]
						o.Increment = nil
						o.Label = "alpha"
						cfg.Extra["develop"] = o
						graph[branch] = []string{"main"}
						wantBranches = []string{"main"}
						wantLabel = label("alpha")
					case "When_getting_configurations_of_a_branch_without_tag_Given_branch_which_inherits_from_parent_branch_Then_result_should_have_tag_from_parent":
						cfg.Main.Label = ""
						o := cfg.Extra["develop"]
						o.Increment = nil
						o.NullLabel = true
						cfg.Extra["develop"] = o
						graph[branch] = []string{"main"}
						wantBranches = []string{"main"}
						wantLabel = label("")
					case "UsesFirstBranchConfigWhenMultipleMatch":
						branch = tc.Arguments[0]
						wantBranches = []string{branch}
						cfg, err = cfg.WithBranchDefinitions([]config.BranchDefinition{
							{Name: "release/latest", Regex: "release/latest", Options: config.Options{Increment: increment("None"), Label: "latest"}},
							{Name: "release", Regex: `releases?[\/-]`, Options: config.Options{Increment: increment("Patch"), Label: "not-latest"}},
						})
						if err != nil {
							t.Fatal(err)
						}
						wantIncrement = increment(tc.Arguments[1])
						wantLabel = label(tc.Arguments[2])
					case "When_getting_configurations_of_an_orphaned_branch_Given_fallback_configuration_with_increment_inherit_Then_result_should_be_empty":
						cfg.FallbackIncrement = nil
						setIncrement("develop", "Inherit")
						wantBranches = nil
					case "When_getting_configurations_of_an_orphaned_branch_Given_fallback_configuration_with_increment_Then_result_should_have_fallback_increment":
						cfg.FallbackIncrement = increment(tc.Arguments[0])
						setIncrement("develop", "Inherit")
						wantIncrement = increment(tc.Arguments[0])
					case "When_getting_configurations_of_an_unknown_branch_Given_fallback_and_unknown_configuration_with_increment_inherit_Then_result_should_be_empty":
						branch = "unknown"
						cfg.FallbackIncrement = nil
						setIncrement("unknown", "Inherit")
						wantBranches = nil
					case "When_getting_configurations_of_an_unknown_branch_Given_fallback_configuration_with_increment_and_unknown_configuration_with_increment_inherit_Then_result_should_have_fallback_increment":
						branch = "unknown"
						wantBranches = []string{branch}
						cfg.FallbackIncrement = increment(tc.Arguments[0])
						setIncrement("unknown", "Inherit")
						wantIncrement = increment(tc.Arguments[0])
					case "When_getting_configurations_of_an_unknown_branch_Given_fallback_configuration_with_increment_and_develop_branch_with_increment_Then_result_should_have_develop_increment":
						branch = "unknown"
						cfg.FallbackIncrement = increment(tc.Arguments[0])
						setIncrement("develop", tc.Arguments[1])
						graph[branch] = []string{"develop"}
						wantBranches = []string{"develop"}
						wantIncrement = increment(tc.Arguments[1])
						if wantIncrement == nil {
							wantIncrement = cfg.FallbackIncrement
						}
						if wantIncrement == nil {
							wantBranches = nil
						}
					case "When_getting_configurations_of_an_unknown_branch_Given_fallback_configuration_with_increment_and_develop_branch_with_increment_inherit_Then_result_should_have_fallback_increment":
						branch = "unknown"
						cfg.FallbackIncrement = increment(tc.Arguments[0])
						setIncrement("develop", "Inherit")
						graph[branch] = []string{"develop"}
						wantBranches = []string{"develop"}
						wantIncrement = cfg.FallbackIncrement
						if wantIncrement == nil {
							wantBranches = nil
						}
					case "When_getting_configurations_of_an_unknown_branch_Given_fallback_and_unknown_configuration_with_increment_inherit_and_develop_branch_with_increment_Then_result_should_have_develop_branch_increment":
						branch = "unknown"
						cfg.FallbackIncrement = nil
						setIncrement("develop", tc.Arguments[0])
						graph[branch] = []string{"develop"}
						wantBranches = []string{"develop"}
						wantIncrement = increment(tc.Arguments[0])
					case "Pull_request_merge_target_selects_local_support_branch_from_multiple_candidates":
						branch = "refs/pull/2/merge"
						message = "Merge pull request 2 from hotfix/v1.0.2 into support/v1.0.x"
						graph[branch] = []string{"refs/heads/main"}
						wantBranches = []string{"refs/heads/support/v1.0.x"}
						wantIncrement = increment("Patch")
						for _, name := range []string{"refs/heads/main", "refs/remotes/origin/support/v1.0.x", "refs/heads/support/v1.0.x"} {
							kind, _, err := cfg.Branch(name)
							if err != nil {
								t.Fatal(err)
							}
							branches = append(branches, branchRef{name: name, kind: kind})
						}
						cfg.Main.Increment = increment("Minor")
					case "Pull_request_merge_target_accepts_remote_only_branch":
						branch = "refs/pull/2/merge"
						message = "Merge pull request 2 from hotfix/v1.0.2 into support/v1.0.x"
						graph[branch] = []string{"refs/heads/main"}
						wantBranches = []string{"refs/remotes/origin/support/v1.0.x"}
						for _, name := range []string{"refs/heads/main", "refs/remotes/origin/support/v1.0.x"} {
							kind, _, err := cfg.Branch(name)
							if err != nil {
								t.Fatal(err)
							}
							branches = append(branches, branchRef{name: name, kind: kind})
						}
						cfg.Main.Increment = increment("Minor")
					case "Pull_request_target_falls_back_to_all_source_candidates_when_it_cannot_disambiguate":
						message = tc.Arguments[0]
						branch = tc.Arguments[1]
						graph[branch] = []string{"refs/heads/main", "refs/heads/support/v1.0.x"}
						wantBranches = []string{"refs/heads/main", "refs/heads/support/v1.0.x"}
						for _, name := range wantBranches {
							kind, _, err := cfg.Branch(name)
							if err != nil {
								t.Fatal(err)
							}
							branches = append(branches, branchRef{name: name, kind: kind})
						}
						cfg.Main.Increment = increment("Minor")
					case "Pull_request_target_continues_inheriting_from_selected_branch":
						branch = "refs/pull/2/merge"
						message = "Merge pull request 2 from hotfix/v1.0.2 into support/v1.0.x"
						graph[branch] = []string{"refs/heads/main"}
						graph["refs/heads/support/v1.0.x"] = []string{"refs/heads/main"}
						wantBranches = []string{"refs/heads/main"}
						wantIncrement = increment("Minor")
						cfg.Main.Increment = increment("Minor")
						setIncrement("support", "Inherit")
						for _, name := range []string{"refs/heads/main", "refs/heads/support/v1.0.x"} {
							kind, _, err := cfg.Branch(name)
							if err != nil {
								t.Fatal(err)
							}
							branches = append(branches, branchRef{name: name, kind: kind})
						}

					default:
						t.Fatal("unhandled original method", method)
					}
					finder := branchConfigurationFinder{config: cfg, branches: branches, sources: func(name string, visited map[string]bool) ([]string, error) { return graph[name], nil }, commit: func(string) (string, bool, error) { return message, message != "", nil }}
					got, err := finder.configurations(branch, "")
					if err != nil {
						t.Fatal(err)
					}
					names := []string{}
					for _, value := range got {
						names = append(names, value.branch)
					}
					if !slices.Equal(names, wantBranches) {
						t.Fatalf("original branch identity/cardinality/order: want %v, got %v", wantBranches, names)
					}
					if wantMode != nil && (len(got) != 1 || got[0].value.Mode != *wantMode) {
						t.Fatalf("original deployment mode assertion: want %s, got %+v", *wantMode, got)
					}
					if wantLabel != nil && (len(got) != 1 || got[0].value.Label != *wantLabel || got[0].value.NullLabel) {
						t.Fatalf("original label assertion: want %q, got %+v", *wantLabel, got)
					}
					if wantIncrement != nil && (len(got) != 1 || got[0].value.Increment != *wantIncrement) {
						t.Fatalf("original increment assertion: want %v, got %+v", *wantIncrement, got)
					}
					if strings.Contains(method, "not_have_versioning_mode_of_parent") && tc.Arguments[0] != "ContinuousDelivery" && got[0].value.Mode == config.Mode(tc.Arguments[0]) {
						t.Fatal("explicit child deployment mode replaced by parent")
					}
				})
			}
		})
	}
}

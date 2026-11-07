package config

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
)

func TestCSharpConfigurationExtensionRouting(t *testing.T) {
	data, err := os.ReadFile("../../testdata/csharp-executor-routing.json")
	if err != nil {
		t.Fatal(err)
	}
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
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	methods := corpus.Suites["ConfigurationExtensionsTests"].Methods
	for _, method := range slices.Sorted(maps.Keys(methods)) {
		t.Run(method, func(t *testing.T) {
			for _, tc := range methods[method].Cases {
				t.Run(tc.ID, func(t *testing.T) {
					switch method {
					case "EnsureGetEffectiveConfigurationWithoutBranchUsesEmptyBranchConfiguration":
						const format = "{SemVer}-custom"
						cfg := Config{Output: OutputSettings{CustomFormat: format}}
						if actual, weight := cfg.OutputFor("").CustomFormat, cfg.Options("").Weight; actual != format || weight != 0 {
							t.Fatalf("effective format = %q, prerelease weight = %d", actual, weight)
						}
					case "EnsureIsReleaseBranchWithReferenceNameWorksAsExpected":
						cfg, err := ParseProvided([]byte("workflow: GitFlow/v1"), "v7")
						if err != nil {
							t.Fatal(err)
						}
						arg := tc.Arguments
						ref := gitrepo.ReferenceFromBranchName(arg[0].(string))
						if ref.Canonical != arg[1] || ref.Friendly != arg[2] || ref.WithoutOrigin != arg[3] ||
							ref.IsLocalBranch != arg[4] || ref.IsPullRequest != arg[5] || ref.IsRemoteBranch != arg[6] || ref.IsTag != arg[7] {
							t.Fatalf("reference %+v differs from original case %v", ref, arg)
						}
						kind, _, err := cfg.Branch(ref.Canonical)
						if err != nil {
							t.Fatal(err)
						}
						setting := cfg.Options(kind).IsRelease
						if actual := setting != nil && *setting; actual != arg[8] {
							t.Fatalf("%s matched %s (release = %v), want release = %v", ref.Canonical, kind, actual, arg[8])
						}
					default:
						t.Fatal("missing branch configuration assertions for", method)
					}
				})
			}
		})
	}
}

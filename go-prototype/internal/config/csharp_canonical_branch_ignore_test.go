package config

import (
	"testing"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestCSharpCanonicalBranchIgnore(t *testing.T) {
	var corpus struct {
		Suites struct {
			BranchRepositoryTests struct {
				Methods map[string]struct {
					Cases []struct {
						ID                  string
						Arguments, Patterns []string
					}
				}
			}
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-wix-and-output.json", &corpus); err != nil {
		t.Fatal(err)
	}
	for method, item := range corpus.Suites.BranchRepositoryTests.Methods {
		t.Run(method, func(t *testing.T) {
			for _, tc := range item.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					ignored, err := (IgnoreSettings{Branches: tc.Patterns}).BranchIgnored(tc.Arguments[0])
					if err != nil || !ignored {
						t.Fatalf("%s was not ignored: %v", tc.Arguments[0], err)
					}
				})
			}
		})
	}
}

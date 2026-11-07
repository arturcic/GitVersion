package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

type ciCase struct {
	ID, Ref     string
	Expected    []string
	Environment map[string]string
	Arguments   []any
}

func ciCases(t *testing.T) map[string]struct {
	Methods map[string]struct{ Cases []ciCase }
} {
	t.Helper()
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct{ Cases []ciCase }
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-shallow-ci.json", &corpus); err != nil {
		t.Fatal(err)
	}
	return corpus.Suites
}

func TestCSharpPullRequestInput(t *testing.T) {
	item := ciCases(t)["PullRequestInBuildAgentTest"].Methods["VerifyPullRequestInput"]
	t.Run("VerifyPullRequestInput", func(t *testing.T) {
		for _, tc := range item.Cases {
			t.Run(tc.ID, func(t *testing.T) {
				args := tc.Arguments
				ref := gitrepo.NewReferenceName(args[0].(string))
				if ref.Friendly != args[1] || ref.IsLocalBranch != args[2] ||
					ref.IsPullRequest != args[3] || ref.IsRemoteBranch != args[4] {
					t.Fatalf("reference %+v does not match %v", ref, args)
				}
			})
		}
	})
}

func TestCSharpShallowAndCI(t *testing.T) {
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
	for suite, details := range ciCases(t) {
		for method, item := range details.Methods {
			if method == "VerifyPullRequestInput" {
				continue
			}
			t.Run(method, func(t *testing.T) {
				for _, tc := range item.Cases {
					t.Run(tc.ID, func(t *testing.T) {
						for key, value := range tc.Environment {
							t.Setenv(key, value)
						}
						if suite == "GitVersionExecutorTests" {
							t.Setenv("TF_BUILD", "true")
						}
						path, sha := ciRepository(t, t.TempDir(), suite, method, tc.Ref)
						args := []string{path, "--output", "json", "--no-fetch", "--no-cache"}
						allowShallow := strings.Contains(method, "AllowShallow")
						if allowShallow {
							args = append(args, "--allow-shallow")
						}
						before, err := fixtures.State(path)
						if err != nil {
							t.Fatal(err)
						}
						var stdout, stderr bytes.Buffer
						err = execute(args, &stdout, &stderr)
						if strings.HasSuffix(method, "_ThrowException") {
							if err == nil || err.Error() != tc.Expected[0] || stdout.Len() != 0 {
								t.Fatalf("expected %q, got %v, stdout %s", tc.Expected[0], err, &stdout)
							}
						} else {
							if err != nil || stderr.Len() != 0 {
								t.Fatalf("calculation: %v, %s", err, &stderr)
							}
							var values map[string]any
							if err := json.Unmarshal(stdout.Bytes(), &values); err != nil || values["Sha"] != sha {
								t.Fatalf("expected SHA %s, got %v, error %v", sha, values, err)
							}
							if suite != "GitVersionExecutorTests" {
								if values["FullSemVer"] != tc.Expected[0] {
									t.Fatalf("FullSemVer = %v, want %s", values["FullSemVer"], tc.Expected[0])
								}
							} else if len(tc.Expected) != 0 {
								field := "SemVer"
								if strings.Contains(method, "LimitedCloneDepth") {
									field = "MajorMinorPatch"
								}
								if values[field] != tc.Expected[0] || values["VersionSourceSemVer"] != tc.Expected[1] {
									t.Fatalf("incorrect version fields: %v", values)
								}
							}
						}
						after, err := fixtures.State(path)
						if err != nil || before != after {
							t.Fatalf("calculation changed the fixture: %v", err)
						}
					})
				}
			})
		}
	}
}

func ciRepository(t *testing.T, directory, suite, method, ref string) (string, string) {
	t.Helper()
	if strings.Contains(method, "Shallow") {
		return shallowCIRepository(t, directory, strings.Contains(method, "LimitedCloneDepth"))
	}
	var steps []fixtures.Step
	var configuration string
	var selected string
	switch suite {
	case "GitVersionExecutorTests":
		steps = []fixtures.Step{
			{Op: "commit", ID: "root", Message: "Init commit"},
			{Op: "branch", Name: "feature/1.0"}, {Op: "checkout", Name: "feature/1.0"},
			{Op: "commit", ID: "selected", Message: "feat: a new commit"},
			{Op: "branch", Name: "support/1.0"},
		}
		if strings.HasSuffix(method, "_ReturnSemver") {
			steps = append(steps, fixtures.Step{Op: "tag", Name: "1.0.1"})
		}
		selected = "selected"
	case "TagCheckoutInBuildAgentTests":
		steps = []fixtures.Step{
			{Op: "commit", ID: "root"}, {Op: "tag", Name: "0.1.0"},
			{Op: "branch", Name: "develop"}, {Op: "checkout", Name: "develop"}, {Op: "commit", ID: "develop"},
			{Op: "branch", Name: "release/0.2.0"}, {Op: "checkout", Name: "release/0.2.0"}, {Op: "commit", ID: "release"},
			{Op: "checkout", Name: "main"}, {Op: "merge", ID: "merge", From: "release/0.2.0"},
			{Op: "commit", ID: "selected"}, {Op: "tag", Name: "0.2.0"},
		}
		selected = "selected"
	case "PullRequestInBuildAgentTest":
		steps = []fixtures.Step{
			{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.3"},
			{Op: "branch", Name: "FeatureBranch"}, {Op: "checkout", Name: "FeatureBranch"},
			{Op: "commit", ID: "first"}, {Op: "commit", ID: "second"},
			{Op: "branch", Name: "checkout-tip"}, {Op: "checkout", Name: "checkout-tip"},
			{Op: "commit", ID: "selected"},
		}
		if strings.HasPrefix(ref, "refs/merge-requests/") {
			configuration = "workflow: GitFlow/v1\ncalculation:\n  branches:\n    pull-request:\n      regex: ^merge-requests/(?<Number>\\d+)/(head|merge)$\noutput: {}\n"
		}
		selected = "selected"
	default:
		t.Fatal("unknown source suite", suite)
	}
	steps = append(steps, fixtures.Step{Op: "checkpoint", ID: selected}, fixtures.Step{Op: "checkout", Name: "main"}, fixtures.Step{Op: "checkpoint", ID: "remote"})
	cps, err := fixtures.Generate([]fixtures.Recipe{{ID: "ci-history", Config: configuration, Steps: steps}}, filepath.Join(directory, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	tip := plumbing.NewHash(cps[0].Head)
	remote := cps[1].Path
	origin, err := git.PlainOpen(remote)
	if err != nil {
		t.Fatal(err)
	}
	if ref != "" {
		if err := origin.Storer.RemoveReference(plumbing.NewBranchReferenceName("checkout-tip")); err != nil {
			t.Fatal(err)
		}
	}
	local := localCICopy(t, directory, remote)
	if ref != "" {
		if err := origin.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(ref), tip)); err != nil {
			t.Fatal(err)
		}
	}
	repo, err := git.PlainOpen(local)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Checkout(&git.CheckoutOptions{Hash: tip, Force: true}); err != nil {
		t.Fatal(err)
	}
	return local, tip.String()
}

func shallowCIRepository(t *testing.T, directory string, extend bool) (string, string) {
	t.Helper()
	var steps []fixtures.Step
	for i := range 5 {
		id := "seed-" + string(rune('a'+i))
		steps = append(steps, fixtures.Step{Op: "commit", ID: id}, fixtures.Step{Op: "checkpoint", ID: id})
	}
	if extend {
		steps = append(steps, fixtures.Step{Op: "commit", ID: "initial", Message: "Initial commit"},
			fixtures.Step{Op: "commit", ID: "tagged"}, fixtures.Step{Op: "tag", Name: "1.0.0"},
			fixtures.Step{Op: "commit", ID: "major", Message: "+semver:major"},
			fixtures.Step{Op: "checkpoint", ID: "head"})
	}
	cps, err := fixtures.Generate([]fixtures.Recipe{{ID: "shallow-ci", Steps: steps}}, filepath.Join(directory, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	head := cps[len(cps)-1]
	local := localCICopy(t, directory, head.Path)
	if err := os.WriteFile(filepath.Join(local, ".git", "shallow"), []byte(cps[4].Head+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, cp := range cps[:4] {
		if err := os.Remove(filepath.Join(local, ".git", "objects", cp.Head[:2], cp.Head[2:])); err != nil {
			t.Fatal(err)
		}
	}
	return local, head.Head
}

func localCICopy(t *testing.T, directory, remote string) string {
	t.Helper()
	local := filepath.Join(directory, "checkout")
	if err := fixtures.Copy(remote, local); err != nil {
		t.Fatal(err)
	}
	repo, err := git.PlainOpen(local)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{remote}}); err != nil {
		t.Fatal(err)
	}
	return local
}

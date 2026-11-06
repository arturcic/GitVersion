//go:build integration || platform

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPinnedGoCalculationParity(t *testing.T) {
	dotnet := os.Getenv("PARITY_DOTNET")
	if dotnet == "" {
		t.Fatal("set PARITY_DOTNET to the pinned host")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, ".local", "gitversion-go")
	binaryHash, err := parity.FileHash(exe)
	if err != nil {
		t.Fatal("build .local/gitversion-go before acceptance:", err)
	}
	parent := filepath.Join(root, ".local", "calculation-evidence")
	if err = os.MkdirAll(parent, 0755); err != nil {
		t.Fatal(err)
	}
	workspace, err := os.MkdirTemp(parent, "run-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", workspace)
	recipes, err := fixtures.Load(filepath.Join(root, "testdata", "recipes.json"))
	if err != nil {
		t.Fatal(err)
	}
	recipes = append(recipes, calculationVariants()...)
	recipePath := filepath.Join(workspace, "recipes.json")
	if err = parity.WriteJSON(recipePath, recipes); err != nil {
		t.Fatal(err)
	}
	command := parity.Command{Executable: exe, Arguments: []string{"calculate", "--repository", "{repo}", "--output", "json", "--trace"}}
	commandPath := filepath.Join(workspace, "candidate.json")
	if err = parity.WriteJSON(commandPath, command); err != nil {
		t.Fatal(err)
	}
	common := []string{"--spec", filepath.Join(root, "baseline.json"), "--recipes", recipePath, "--baseline", filepath.Join(root, ".local", "baseline"), "--lock", filepath.Join(root, ".local", "baseline.lock.json"), "--dotnet", dotnet}
	capture := filepath.Join(workspace, "capture")
	verify := filepath.Join(workspace, "go")
	args := append([]string{"capture"}, common...)
	args = append(args, "--out", capture)
	if err = run(t.Context(), args); err != nil {
		t.Fatal(err)
	}
	args = append([]string{"verify"}, common...)
	args = append(args, "--expected", capture, "--candidate", commandPath, "--out", verify)
	runErr := run(t.Context(), args)
	var report Report
	if err = parity.ReadJSON(filepath.Join(verify, "report.json"), &report); err != nil {
		t.Fatal(err)
	}
	recipeHash, err := parity.FileHash(recipePath)
	if err != nil {
		t.Fatal(err)
	}
	lockHash, err := parity.FileHash(filepath.Join(root, ".local", "baseline.lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	provenance := map[string]any{"candidate_sha256": binaryHash, "recipes_sha256": recipeHash, "baseline_lock_sha256": lockHash, "passed": report.Passed, "failed": report.Failed, "expected": 63, "complete": false}
	if err = parity.WriteJSON(filepath.Join(workspace, "provenance.json"), provenance); err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		for _, c := range report.Cases {
			if c.Status != "passed" {
				t.Logf("%s: %s %+v", c.ID, c.Error, c.Differences)
			}
		}
		t.Fatal(runErr)
	}
	if report.Passed != 63 || report.Failed != 0 || report.Calculator != "external-candidate" {
		t.Fatalf("incomplete calculation comparison %+v", report)
	}
	// Fresh C# output for the original cases must still match the immutable
	// portable reference, independently of the new candidate and extra cases.
	var golden Golden
	if err = parity.ReadJSON(filepath.Join(root, "testdata", "reference.json"), &golden); err != nil {
		t.Fatal(err)
	}
	if len(golden.Outputs) != 41 {
		t.Fatal("original reference inventory changed")
	}
	for id, expected := range golden.Outputs {
		actual, err := os.ReadFile(filepath.Join(capture, id, "stdout.json"))
		if err != nil {
			t.Fatal(err)
		}
		diff, err := parity.Compare(expected, actual)
		if err != nil || len(diff) > 0 {
			t.Fatalf("frozen C# reference changed %s %+v %v", id, diff, err)
		}
	}
	// Run an ordinary relocated fixture with no tools on PATH and no baseline
	// environment selectors. The production Go binary receives only the repo.
	cp := filepath.Join(capture, "fixtures", "linear-main--two-after")
	isolated, err := parity.Run(t.Context(), command, cp, filepath.Join(workspace, "without-reference-tools"), map[string]string{"PATH": ""}, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err = saveResult(filepath.Join(workspace, "without-reference-tools"), isolated); err != nil {
		t.Fatal(err)
	}
	if err = isolated.Validate(); err != nil {
		t.Fatal(err)
	}
	diff, err := parity.Compare(golden.Outputs["linear-main--two-after"], isolated.Stdout)
	if err != nil || len(diff) != 0 {
		t.Fatalf("standalone calculation differs %+v %v", diff, err)
	}
	var trace map[string]any
	if err = json.Unmarshal(isolated.Stderr, &trace); err != nil {
		t.Fatal("trace is not separate JSON:", err)
	}
	if trace["semantic_source"] != "1.2.0" || len(trace["counted_commits"].([]any)) != 2 {
		t.Fatalf("trace lost provenance: %+v", trace)
	}
	after, err := parity.FileHash(exe)
	if err != nil || after != binaryHash {
		t.Fatal("candidate changed during acceptance")
	}
	provenance["complete"] = true
	if err = parity.WriteJSON(filepath.Join(workspace, "provenance.json"), provenance); err != nil {
		t.Fatal(err)
	}
}

func calculationVariants() []fixtures.Recipe {
	const config = "workflow: GitHubFlow/v1\n"
	c := func(id, msg string) fixtures.Step { return fixtures.Step{Op: "commit", ID: id, Message: msg} }
	tag := func(v string) fixtures.Step { return fixtures.Step{Op: "tag", Name: v} }
	cp := func(id string) fixtures.Step { return fixtures.Step{Op: "checkpoint", ID: id} }
	branch := func(name string) []fixtures.Step {
		return []fixtures.Step{{Op: "branch", Name: name}, {Op: "checkout", Name: name}}
	}
	feature := func(name, version, tagged string) []fixtures.Step {
		steps := []fixtures.Step{c("root", ""), tag(version)}
		steps = append(steps, branch(name)...)
		steps = append(steps, c("work", ""))
		if tagged != "" {
			steps = append(steps, tag(tagged), cp("tagged"))
		}
		return append(steps, c("after", ""), cp("after"))
	}
	return []fixtures.Recipe{
		{ID: "variant-alternative-number", Config: config + "calculation:\n  branches:\n    main:\n      increment: None\n", Steps: append(append([]fixtures.Step{c("root", ""), tag("1.2.0")}, branch("feature/foo")...), c("one", ""), tag("2.0.0-foo.8"), c("two", ""), tag("3.0.0-bar.5"), cp("head"))},
		{ID: "variant-unicode-bump", Config: config, Steps: []fixtures.Step{c("root", ""), tag("2.3.4"), c("bump", "Change +semver:\u00a0minor"), cp("head")}},
		{ID: "variant-configured-tag-anchor", Config: config + "calculation:\n  next-version: 9.0.0\n", Steps: []fixtures.Step{c("root", ""), tag("1.2.0"), cp("tagged"), c("after", ""), cp("after")}},
		{ID: "variant-linear", Config: config, Steps: []fixtures.Step{c("root", ""), tag("11.22.33"), cp("tagged"), c("one", ""), c("two", ""), c("three", ""), cp("after")}},
		{ID: "variant-feature-name", Config: config, Steps: feature("features/team.fix😀", "7.8.9", "")},
		{ID: "variant-next", Config: config + "calculation:\n  next-version: 5.7.8\n", Steps: []fixtures.Step{c("root", ""), cp("root"), c("one", ""), c("two", ""), cp("after")}},
		{ID: "variant-no-increment", Config: config + "calculation:\n  branches:\n    main:\n      increment: None\n", Steps: []fixtures.Step{c("root", ""), tag("9.8.7"), c("one", ""), cp("after")}},
		{ID: "variant-inherited-major", Config: config + "calculation:\n  branches:\n    main:\n      increment: Major\n", Steps: feature("feature/build42", "2.5.6", "")},
		{ID: "variant-competing-tags", Config: config, Steps: []fixtures.Step{c("root", ""), tag("2.8.0"), c("one", ""), tag("1.5.0"), c("two", ""), cp("after")}},
		{ID: "variant-equal-candidates", Config: config, Steps: []fixtures.Step{c("root", ""), tag("1.2.0"), c("one", ""), tag("v1.2.0"), c("two", ""), cp("after")}},
		{ID: "variant-message-precedence", Config: config, Steps: []fixtures.Step{c("root", ""), tag("4.5.6"), c("skip", "+semver: skip +semver: major"), cp("skip"), c("minor", "+semver: feature"), cp("minor"), c("major", "+semver: breaking"), cp("major")}},
		{ID: "variant-feature-number", Config: config, Steps: feature("feature/topic", "8.9.0", "9.0.0-topic.8")},
		{ID: "variant-main-numeric", Config: config, Steps: []fixtures.Step{c("root", ""), tag("3.4.5-2"), cp("tagged"), c("one", ""), cp("after")}},
		{ID: "variant-main-unrelated", Config: config, Steps: []fixtures.Step{c("root", ""), tag("1.2.0"), c("one", ""), tag("9.0.0-beta.2"), cp("tagged")}},
		{ID: "variant-unmerged-main-tag", Config: config, Steps: append(append([]fixtures.Step{c("root", ""), tag("1.0.0")}, branch("feature/new")...), c("work", ""), fixtures.Step{Op: "checkout", Name: "main"}, c("fix", ""), tag("1.1.0"), fixtures.Step{Op: "checkout", Name: "feature/new"}, cp("feature"))},
	}
}

package parity

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
)

// A real subprocess exercises exec semantics without relying on shell quoting.
func TestProcessHelper(t *testing.T) {
	marker := -1
	for i, a := range os.Args {
		if a == "--" {
			marker = i
			break
		}
	}
	if marker < 0 {
		return
	}
	args := os.Args[marker+1:]
	if len(args) < 2 {
		os.Exit(90)
	}
	switch args[0] {
	case "success":
		if os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("GIT_BRANCH") != "" || os.Getenv("GIT_CONFIG_COUNT") != "" || os.Getenv("SECRET_TEST_TOKEN") != "" || os.Getenv("TZ") != "UTC" {
			os.Exit(91)
		}
		if len(args) != 3 || args[2] != "literal spaces and $symbols" {
			os.Exit(92)
		}
		if _, err := os.Stat(filepath.Join(args[1], ".git")); err != nil {
			os.Exit(93)
		}
		fmt.Print(`{"SemVer":"1.2.3","Major":1}`)
	case "failure":
		fmt.Fprint(os.Stderr, "intentional failure")
		os.Exit(7)
	case "invalid":
		fmt.Print(`{"Major":1,"Major":2}`)
	case "flood":
		fmt.Print(strings.Repeat("x", maxOutputBytes+1))
	case "timeout":
		time.Sleep(5 * time.Second)
	case "mutate":
		if err := os.WriteFile(filepath.Join(args[1], "root.txt"), []byte("changed"), 0644); err != nil {
			os.Exit(94)
		}
		fmt.Print(`{"Major":1}`)
	default:
		os.Exit(95)
	}
	os.Exit(0)
}

func runnerFixture(t *testing.T) string {
	t.Helper()
	r := fixtures.Recipe{ID: "runner", Config: "workflow: GitHubFlow/v1\n", Steps: []fixtures.Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "head"}}}
	cp, err := fixtures.Generate([]fixtures.Recipe{r}, filepath.Join(t.TempDir(), "source"))
	if err != nil {
		t.Fatal(err)
	}
	return cp[0].Path
}

func TestRunIsolatedProcess(t *testing.T) {
	for _, key := range []string{"GITHUB_ACTIONS", "GIT_BRANCH", "GIT_CONFIG_COUNT", "SECRET_TEST_TOKEN"} {
		t.Setenv(key, "polluted")
	}
	source := runnerFixture(t)
	before, err := fixtures.State(source)
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), Command{exe, []string{"-test.run=^TestProcessHelper$", "--", "success", "{repo}", "literal spaces and $symbols"}}, source, filepath.Join(t.TempDir(), "path with spaces"), nil, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err = result.Validate(); err != nil {
		t.Fatalf("%v stderr=%s", err, result.Stderr)
	}
	d, err := Compare([]byte(`{"SemVer":"1.2.3","Major":1}`), result.Stdout)
	if err != nil || len(d) != 0 {
		t.Fatalf("unexpected output %s %v", result.Stdout, err)
	}
	after, err := fixtures.State(source)
	if err != nil || before != after {
		t.Fatalf("original fixture changed: %v", err)
	}
}

func TestRunRejectsFailedMalformedTimedOutAndMutatingCandidates(t *testing.T) {
	source := runnerFixture(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"failure", "invalid", "timeout", "mutate", "flood"} {
		t.Run(mode, func(t *testing.T) {
			timeout := 5 * time.Second
			if mode == "timeout" {
				timeout = 50 * time.Millisecond
			}
			r, err := Run(context.Background(), Command{exe, []string{"-test.run=^TestProcessHelper$", "--", mode, "{repo}"}}, source, filepath.Join(t.TempDir(), "run"), nil, timeout)
			if err != nil {
				t.Fatal(err)
			}
			if err = r.Validate(); err == nil {
				t.Fatal("accepted candidate", mode)
			}
			switch mode {
			case "flood":
				if !strings.Contains(r.Error, "output exceeded") || len(r.Stdout) != maxOutputBytes {
					t.Fatalf("output was not bounded: %d %s", len(r.Stdout), r.Error)
				}
			case "failure":
				if r.ExitCode != 7 || !strings.Contains(string(r.Stderr), "intentional failure") {
					t.Fatalf("lost process evidence %+v %s", r, r.Stderr)
				}
			case "timeout":
				if !strings.Contains(r.Error, "deadline exceeded") {
					t.Fatalf("lost timeout: %+v", r)
				}
			case "mutate":
				if r.Before == r.After || !strings.Contains(r.Error, "repository mutated") {
					t.Fatalf("missed mutation %+v", r)
				}
			}
		})
	}
}

func TestRunRejectsNonpositiveTimeout(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Second} {
		if _, err := Run(context.Background(), Command{}, "unused", filepath.Join(t.TempDir(), "run"), nil, timeout); err == nil {
			t.Fatal("accepted nonpositive timeout", timeout)
		}
	}
}

func TestResolveRequiresExplicitArgumentAndAbsoluteExecutable(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []Command{{"relative", []string{"{repo}"}}, {exe, []string{"--path={repo}"}}, {exe, []string{"no-placeholder"}}} {
		if _, err := Resolve(c, t.TempDir()); err == nil {
			t.Fatalf("accepted invalid command %+v", c)
		}
	}
	c := Command{exe, []string{"{repo}"}}
	r, err := Resolve(c, "/fixture")
	if err != nil || r.Arguments[0] != "/fixture" || c.Arguments[0] != "{repo}" {
		t.Fatalf("resolve mutated original or failed: %+v %v", r, err)
	}
}

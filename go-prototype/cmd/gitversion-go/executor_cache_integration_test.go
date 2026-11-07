//go:build platform

package main

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformExecutorCacheParity(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	corpus := loadExecutorCacheCorpus(t)
	baseDir := filepath.Join(root, ".local/baseline")
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	files, err := parity.FileSet(baseDir)
	if err != nil || !reflect.DeepEqual(files, baseline.Files) {
		t.Fatal("baseline file set mismatch", err)
	}
	specHash, err := parity.FileHash(filepath.Join(root, "baseline.json"))
	if err != nil || specHash != baseline.SpecSHA256 {
		t.Fatal("baseline specification mismatch", err)
	}
	candidate := os.Getenv("PARITY_CANDIDATE")
	candidateHash, err := parity.FileHash(candidate)
	if err != nil {
		t.Fatal(err)
	}
	report, err := os.MkdirTemp(os.Getenv("PARITY_EVIDENCE_DIR"), "executor-cache-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", report)
	comparisons := 0
	source := filepath.Join(report, "seed")
	prepareExecutorCacheRepo(t, source)
	compare := func(t *testing.T, parser, layout, directory, source string, actions func(t *testing.T, run func(label string, args ...string) string, repo string, paths func() []string)) {
		t.Helper()
		all := map[string]map[string]string{}
		for _, implementation := range []string{"csharp", "go"} {
			outDir := filepath.Join(report, directory, implementation)
			if err := os.MkdirAll(outDir, 0755); err != nil {
				t.Fatal(err)
			}
			repo := filepath.Join(outDir, "r")
			if err := fixtures.Copy(source, repo); err != nil {
				t.Fatal(err)
			}
			env := maps.Clone(baseline.Spec.Environment)
			env["GITVERSION_ARGUMENT_PARSER_VERSION"] = parser
			env["GITVERSION_CONFIGURATION_VERSION"] = layout
			all[implementation] = map[string]string{}
			run := func(label string, args ...string) string {
				t.Helper()
				exe := candidate
				if implementation == "csharp" {
					exe = os.Getenv("PARITY_DOTNET")
					args = append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, args...)
				}
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, exe, args...)
				cmd.Dir, cmd.Env = repo, parity.Environment(outDir, env)
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				err := cmd.Run()
				if writeErr := parity.WriteJSON(filepath.Join(outDir, label+".json"), map[string]string{
					"stdout": stdout.String(), "stderr": stderr.String(),
				}); writeErr != nil {
					t.Fatal(writeErr)
				}
				if err != nil {
					t.Fatalf("%s %s: %v\n%s\n%s", implementation, label, err, &stdout, &stderr)
				}
				all[implementation][label] = stdout.String()
				return stdout.String()
			}
			paths := func() []string {
				t.Helper()
				cacheDir := filepath.Join(repo, ".git", "gitversion_cache")
				entries, err := os.ReadDir(cacheDir)
				if err != nil {
					t.Fatal(err)
				}
				var paths []string
				for _, entry := range entries {
					paths = append(paths, filepath.Join(cacheDir, entry.Name()))
				}
				return paths
			}
			actions(t, run, repo, paths)
		}
		keys := func(m map[string]string) []string { return slices.Sorted(maps.Keys(m)) }
		if !slices.Equal(keys(all["csharp"]), keys(all["go"])) {
			t.Fatal("different execution checkpoints")
		}
		for _, label := range keys(all["csharp"]) {
			ref, actual := all["csharp"][label], all["go"][label]
			if !strings.HasPrefix(strings.TrimSpace(ref), "{") {
				if strings.TrimSpace(actual) != strings.TrimSpace(ref) {
					t.Fatalf("%s scalar output differs: %q vs %q", label, ref, actual)
				}
			} else if differences, err := parity.Compare([]byte(ref), []byte(actual)); err != nil || len(differences) != 0 {
				t.Fatalf("%s JSON differs: %v, %v", label, differences, err)
			}
			comparisons++
		}
	}

	cases := corpus.Suites["GitVersionExecutorTests"].Methods["ConfigChangeInvalidatesCache"].Cases
	for index, item := range cases {
		name := item.Arguments[0].(string)
		t.Run(name, func(t *testing.T) {
			compare(t, "v7", "v6", fmt.Sprintf("c%d", index), source,
				func(t *testing.T, run func(string, ...string) string, repo string, paths func() []string) {
					check := func(label, assembly string, args ...string) {
						t.Helper()
						values, err := parity.Object([]byte(run(label, append([]string{"--no-fetch", "--no-normalize"}, args...)...)))
						if err != nil || values["AssemblySemVer"] != assembly {
							t.Fatalf("%s: assembly = %v, want %s, error %v", label, values["AssemblySemVer"], assembly, err)
						}
					}
					check("initial", item.Assemblies[0])
					entries := paths()
					if len(entries) != 1 {
						t.Fatalf("expected one cache entry, got %v", entries)
					}
					if err := os.WriteFile(entries[0], corpus.CachePayload, 0600); err != nil {
						t.Fatal(err)
					}
					check("cached", item.Assemblies[1])
					check("bypassed", item.Assemblies[0], "--no-cache")
					if err := os.WriteFile(filepath.Join(repo, name), []byte("next-version: 5.0.0"), 0600); err != nil {
						t.Fatal(err)
					}
					check("config-change", item.Assemblies[2])
				})
		})
	}

	history := makeCLIHistory(t, report, "1.0.0", true)
	requested, head := history[0], history[1]
	for _, parser := range []string{"v6", "v7"} {
		t.Run("explicit-commit-"+parser, func(t *testing.T) {
			compare(t, parser, "v7", parser, head.Path,
				func(t *testing.T, run func(string, ...string) string, repo string, paths func() []string) {
					args := []string{"--no-fetch", "--no-normalize", "--show-variable", "Sha"}
					commit := "--commit"
					if parser == "v6" {
						args = []string{repo, "-nofetch", "-nonormalize", "-showvariable", "Sha"}
						commit = "-c"
					}
					if got := strings.TrimSpace(run("head", args...)); got != head.Head || len(paths()) != 1 {
						t.Fatalf("unexpected cached HEAD %q", got)
					}
					if got := strings.TrimSpace(run("requested", append(args, commit, requested.Head)...)); got != requested.Head {
						t.Fatalf("explicit commit = %q, want %s", got, requested.Head)
					}
				})
		})
	}
	t.Run("tag-prefix-override", func(t *testing.T) {
		repo, err := git.PlainOpen(source)
		if err != nil {
			t.Fatal(err)
		}
		head, err := repo.Head()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.CreateTag("prefix2.0.0", head.Hash(), nil); err != nil {
			t.Fatal(err)
		}
		compare(t, "v7", "v6", "prefix", source,
			func(t *testing.T, run func(string, ...string) string, _ string, _ func() []string) {
				defaults := run("default", "--no-fetch", "--no-normalize", "--show-variable", "SemVer")
				override := run("override", "--no-fetch", "--no-normalize", "--show-variable", "SemVer", "--override-config", "tag-prefix=prefix")
				if strings.TrimSpace(defaults) != "0.0.1-1" || strings.TrimSpace(override) != "2.0.0" {
					t.Fatalf("tag prefix override returned %q -> %q", defaults, override)
				}
			})
	})
	if !t.Failed() {
		if err := parity.WriteJSON(filepath.Join(report, "report.json"), map[string]any{
			"comparisons": comparisons, "baseline_revision": baseline.Spec.Revision,
			"candidate_sha256": candidateHash, "baseline_files": files, "spec_sha256": specHash,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

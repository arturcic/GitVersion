package main

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

type executorCacheCorpus struct {
	CachePayload json.RawMessage `json:"cache_payload"`
	Suites       map[string]struct {
		Methods map[string]struct {
			Cases []struct {
				ID         string
				Arguments  []any
				Assemblies []string
			}
		}
	}
}

func loadExecutorCacheCorpus(t *testing.T) executorCacheCorpus {
	t.Helper()
	var corpus executorCacheCorpus
	if err := parity.ReadJSON("../../testdata/csharp-executor-cache.json", &corpus); err != nil {
		t.Fatal(err)
	}
	return corpus
}

func TestCSharpExecutorCache(t *testing.T) {
	corpus := loadExecutorCacheCorpus(t)
	for _, suite := range slices.Sorted(maps.Keys(corpus.Suites)) {
		methods := corpus.Suites[suite].Methods
		for _, method := range slices.Sorted(maps.Keys(methods)) {
			t.Run(method, func(t *testing.T) {
				for _, test := range methods[method].Cases {
					if method == "CommitArgumentUsesRequestedCommitWhenCacheExists" && test.Arguments[0].(bool) {
						continue
					}
					t.Run(test.ID, func(t *testing.T) {
						t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
						t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v6")
						if method == "CommitArgumentUsesRequestedCommitWhenCacheExists" {
							t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
							dir := t.TempDir()
							checkpoints := makeCLIHistory(t, dir, "1.0.0", true)
							requested, head := checkpoints[0], checkpoints[1]
							t.Chdir(head.Path)
							show, commit := "--show-variable", "--commit"
							check := func(want string, extra ...string) {
								t.Helper()
								got, _ := executeCacheCLI(t, extra...)
								if strings.TrimSpace(got) != want {
									t.Fatalf("Sha = %q, want %s", got, want)
								}
							}
							check(head.Head, show, "Sha")
							singleCacheEntry(t, head.Path)
							check(requested.Head, commit, requested.Head, show, "Sha")
							check(head.Head, show, "Sha")
							if got := cacheEntries(t, head.Path); len(got) != 2 {
								t.Fatalf("HEAD and requested commit did not receive distinct entries: %v", got)
							}
							return
						}
						dir := t.TempDir()
						if method == "GetProjectRootDirectoryNoWorktree" {
							if _, err := git.PlainInit(dir, false); err != nil {
								t.Fatal(err)
							}
							verifyExecutorProjectRoot(t, dir)
							return
						}
						if method == "GetProjectRootDirectoryWorkingDirectoryWithWorktree" {
							worktree, _ := supportWorktreeFixture(t, dir)
							verifyExecutorProjectRoot(t, worktree)
							return
						}
						prepareExecutorCacheRepo(t, dir)
						switch method {
						case "ConfigurationVersionChangeInvalidatesCache":
							executeCacheCLI(t, dir)
							v6 := singleCacheEntry(t, dir)
							t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
							executeCacheCLI(t, dir)
							paths := cacheEntries(t, dir)
							if len(paths) != 2 || !slices.Contains(paths, v6) {
								t.Fatalf("expected separate v6 and v7 cache entries, got %v", paths)
							}
						case "ResolvedConfiguration_IsCreatedOncePerExecution":
							configuration := []byte("output:\n  update-build-number: false\n")
							calls := 0
							loader := func(repo, custom string, overrides []byte, version string, warn func(string)) (config.Config, []byte, error) {
								calls++
								if repo != dir || custom != "" || !bytes.Equal(overrides, configuration) {
									t.Fatalf("unexpected loader request: %q, %q, %q", repo, custom, overrides)
								}
								resolved, err := config.ParseProvided(configuration, "v7")
								return resolved, configuration, err
							}
							opts := cliArguments{TargetPath: dir, OverrideConfiguration: configuration, Output: map[string]bool{"json": true}}
							result, err := executeCalculation(opts, io.Discard, gitrepo.OpenOptions{}, false, loader)
							if err != nil {
								t.Fatal(err)
							}
							var stdout bytes.Buffer
							if err := writeRootOutput(opts, result.values, result.updateBuildNumber, &stdout); err != nil {
								t.Fatal(err)
							}
							if calls != 1 || result.updateBuildNumber || stdout.Len() == 0 {
								t.Fatalf("configuration loads = %d, update build number = %v, output = %q", calls, result.updateBuildNumber, &stdout)
							}
						case "CacheFileIsMissing":
							_, diagnostics := executeCacheCLI(t, dir, "--log-file", "console")
							if !regexp.MustCompile(`Cache file[^\n]* not found\.`).MatchString(diagnostics) {
								t.Fatalf("cache miss missing from log: %q", diagnostics)
							}
							singleCacheEntry(t, dir)
						case "CacheFileExistsOnDisk", "CacheFileExistsOnDiskWhenOverrideConfigIsSpecifiedVersionShouldBeDynamicallyCalculatedWithoutSavingInCache",
							"ConfigChangeInvalidatesCache", "NoCacheBypassesCache":
							assertAssembly := func(index int, args ...string) string {
								t.Helper()
								stdout, diagnostics := executeCacheCLI(t, append([]string{dir, "--log-file", "console"}, args...)...)
								got, err := parity.Object([]byte(stdout))
								if err != nil || got["AssemblySemVer"] != test.Assemblies[index] {
									t.Fatalf("AssemblySemVer = %v, want %s, decode error %v", got["AssemblySemVer"], test.Assemblies[index], err)
								}
								return diagnostics
							}
							assertAssembly(0)
							path := singleCacheEntry(t, dir)
							if err := os.WriteFile(path, corpus.CachePayload, 0600); err != nil {
								t.Fatal(err)
							}
							switch method {
							case "CacheFileExistsOnDisk":
								diagnostics := assertAssembly(1)
								if !strings.Contains(strings.ToLower(diagnostics), "loading version variables from disk cache file") {
									t.Fatalf("cache hit missing from log: %q", diagnostics)
								}
							case "CacheFileExistsOnDiskWhenOverrideConfigIsSpecifiedVersionShouldBeDynamicallyCalculatedWithoutSavingInCache":
								cacheDir := filepath.Dir(path)
								stamp := time.Unix(1700000000, 0)
								if err := os.Chtimes(cacheDir, stamp, stamp); err != nil {
									t.Fatal(err)
								}
								assertAssembly(1, "--override-config", "tag-prefix=prefix")
								info, err := os.Stat(cacheDir)
								if err != nil || !info.ModTime().Equal(stamp) || singleCacheEntry(t, dir) != path {
									t.Fatalf("override changed cache directory: %v, %v", info, err)
								}
								contents, err := os.ReadFile(path)
								if err != nil || !bytes.Equal(contents, corpus.CachePayload) {
									t.Fatalf("override changed the existing cache entry: %v", err)
								}
							case "ConfigChangeInvalidatesCache":
								assertAssembly(1)
								if err := os.WriteFile(filepath.Join(dir, test.Arguments[0].(string)), []byte("next-version: 5.0.0"), 0600); err != nil {
									t.Fatal(err)
								}
								assertAssembly(2)
							case "NoCacheBypassesCache":
								assertAssembly(1)
								assertAssembly(2, "--no-cache")
								contents, err := os.ReadFile(path)
								if err != nil || !bytes.Equal(contents, corpus.CachePayload) {
									t.Fatalf("--no-cache changed the existing cache entry: %v", err)
								}
							}
						default:
							t.Fatal("missing case assertions for", method)
						}
					})
				}
			})
		}
	}
}

func prepareExecutorCacheRepo(t *testing.T, dir string) {
	t.Helper()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("main"))); err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	const filename = "history.txt"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte("initial\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add(filename); err != nil {
		t.Fatal(err)
	}
	signature := &object.Signature{Name: "Fixture", Email: "fixture@example.test", When: time.Unix(1700000000, 0).UTC()}
	if _, err := worktree.Commit("initial", &git.CommitOptions{Author: signature, Committer: signature}); err != nil {
		t.Fatal(err)
	}
}

func verifyExecutorProjectRoot(t *testing.T, path string) {
	t.Helper()
	root, err := gitrepo.WorkingTreeRoot(path)
	if err != nil || root != path {
		t.Fatalf("project root = %q, %v; want %q", root, err, path)
	}
}

func executeCacheCLI(t *testing.T, args ...string) (string, string) {
	t.Helper()
	var stdout, diagnostics bytes.Buffer
	if err := execute(args, &stdout, &diagnostics); err != nil {
		t.Fatalf("CLI %v: %v (%s)", args, err, &diagnostics)
	}
	return stdout.String(), diagnostics.String()
}

func cacheEntries(t *testing.T, root string) []string {
	t.Helper()
	dir := filepath.Join(root, ".git", "gitversion_cache")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "go-v1-") {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	return paths
}

func singleCacheEntry(t *testing.T, root string) string {
	t.Helper()
	paths := cacheEntries(t, root)
	if len(paths) != 1 {
		t.Fatalf("expected one cache entry, got %v", paths)
	}
	return paths[0]
}

func TestCalculationCacheRecoversFromTruncatedEntry(t *testing.T) {
	dir := t.TempDir()
	prepareExecutorCacheRepo(t, dir)
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	want, _ := executeCacheCLI(t, dir)
	path := singleCacheEntry(t, dir)
	if err := os.WriteFile(path, []byte(`{"FullSemVer":`), 0600); err != nil {
		t.Fatal(err)
	}
	got, _ := executeCacheCLI(t, dir)
	if got != want {
		t.Fatalf("cache recovery changed output: %q vs %q", got, want)
	}
	if _, err := loadCalculationCache(path); err != nil {
		t.Fatalf("cache entry was not replaced: %v", err)
	}
}

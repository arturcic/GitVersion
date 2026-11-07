package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/gitrepo"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

type repositoryExecutionCase struct {
	ID, Expected string
	Arguments    []string
}

func TestCSharpRepositoryExecutor(t *testing.T) {
	var corpus struct {
		Suites struct {
			GitVersionExecutorTests struct {
				Methods map[string]struct {
					Cases []struct {
						ID       string
						Expected []string
					}
				}
			}
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-repository-execution.json", &corpus); err != nil {
		t.Fatal(err)
	}
	for method, item := range corpus.Suites.GitVersionExecutorTests.Methods {
		if strings.HasPrefix(method, "GetDotGitDirectory") {
			continue
		}
		t.Run(method, func(t *testing.T) {
			for _, tc := range item.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
					dir := t.TempDir()
					switch method {
					case "WorkingDirectoryWithoutGit", "WorkingDirectoryWithoutCommits":
						if method == "WorkingDirectoryWithoutCommits" {
							if _, err := git.PlainInit(dir, false); err != nil {
								t.Fatal(err)
							}
						}
						values, _, err := calculateCLI(dir, "", "", "", nil, io.Discard, gitrepo.OpenOptions{})
						if err == nil || !strings.Contains(err.Error(), tc.Expected[0]) || values != nil {
							t.Fatalf("expected %q and no values, got %v, %v", tc.Expected[0], values, err)
						}
					case "CalculateVersionFromWorktreeHead":
						path, head := supportWorktreeFixture(t, dir)
						values, _, err := calculateCLI(path, "", "", "", nil, io.Discard, gitrepo.OpenOptions{})
						if err != nil || values["SemVer"] != tc.Expected[0] || values["VersionSourceSemVer"] != tc.Expected[1] || values["Sha"] != head {
							t.Fatalf("worktree values = %v, error %v", values, err)
						}
					case "CustomVersionUsesBranchSpecificFormat":
						cps, err := fixtures.Generate([]fixtures.Recipe{{
							ID: "branch-format", Steps: []fixtures.Step{
								{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.0.0"},
								{Op: "branch", Name: "feature/custom-format"}, {Op: "checkout", Name: "feature/custom-format"},
								{Op: "commit", ID: "feature"}, {Op: "checkpoint", ID: "head"},
							},
						}}, filepath.Join(dir, "fixtures"))
						if err != nil {
							t.Fatal(err)
						}
						overrides, err := config.ValidationOverrides([]string{
							"workflow=GitFlow/v1", "output.custom-version-format=global-{SemVer}",
							"output.branches.feature.custom-version-format=feature-{SemVer}",
						}, "v7")
						if err != nil {
							t.Fatal(err)
						}
						values, _, err := calculateCLI(cps[0].Path, "", "", "", overrides, io.Discard, gitrepo.OpenOptions{})
						value, _ := values["CustomVersion"].(string)
						if err != nil || !strings.HasPrefix(value, tc.Expected[0]) {
							t.Fatalf("custom version = %q, error %v", value, err)
						}
					default:
						t.Fatal("missing original assertion", method)
					}
				})
			}
		})
	}
}

func supportWorktreeFixture(t *testing.T, dir string) (string, string) {
	t.Helper()
	cps, err := fixtures.Generate([]fixtures.Recipe{{
		ID: "worktree-executor", Steps: []fixtures.Step{
			{Op: "commit", ID: "first"}, {Op: "tag", Name: "v1.0.0"}, {Op: "checkpoint", ID: "first"},
			{Op: "commit", ID: "latest"}, {Op: "tag", Name: "v2.0.0"}, {Op: "checkpoint", ID: "main"},
		},
	}}, filepath.Join(dir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "worktree")
	const branch = "support/1.0"
	if err := fixtures.LinkWorktree(cps[1].Path, path, branch, false); err != nil {
		t.Fatal(err)
	}
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{EnableDotGitCommonDir: true})
	if err != nil {
		t.Fatal(err)
	}
	ref := plumbing.NewBranchReferenceName(branch)
	if err := repo.Storer.SetReference(plumbing.NewHashReference(ref, plumbing.NewHash(cps[0].Head))); err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := worktree.Checkout(&git.CheckoutOptions{Branch: ref, Force: true}); err != nil {
		t.Fatal(err)
	}
	return path, cps[0].Head
}

func teamCityRepository(t *testing.T, dir string) string {
	t.Helper()
	steps := []fixtures.Step{}
	for i := range 5 {
		steps = append(steps, fixtures.Step{Op: "commit", ID: "seed-" + string(rune('a'+i))})
	}
	steps = append(steps, fixtures.Step{Op: "checkpoint", ID: "cloned"},
		fixtures.Step{Op: "commit", ID: "tagged"}, fixtures.Step{Op: "tag", Name: "1.2.3"},
		fixtures.Step{Op: "commit", ID: "remote-latest"}, fixtures.Step{Op: "checkpoint", ID: "remote"})
	cps, err := fixtures.Generate([]fixtures.Recipe{{ID: "teamcity-json", Steps: steps}}, filepath.Join(dir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(dir, "clone")
	if err := fixtures.Copy(cps[0].Path, local); err != nil {
		t.Fatal(err)
	}
	repo, err := git.PlainOpen(local)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{cps[1].Path}}); err != nil {
		t.Fatal(err)
	}
	ref := plumbing.NewRemoteReferenceName("origin", "main")
	if err := repo.Storer.SetReference(plumbing.NewHashReference(ref, plumbing.NewHash(cps[0].Head))); err != nil {
		t.Fatal(err)
	}
	return local
}

func TestCSharpBuildServerJSON(t *testing.T) {
	var corpus struct {
		Suites struct {
			JsonOutputOnBuildServerTest struct {
				Methods map[string]struct{ Cases []repositoryExecutionCase }
			}
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-repository-execution.json", &corpus); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEAMCITY_VERSION", "8.0.0")
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
	for method, item := range corpus.Suites.JsonOutputOnBuildServerTest.Methods {
		t.Run(method, func(t *testing.T) {
			for _, tc := range item.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					local := teamCityRepository(t, t.TempDir())
					args := []string{local, "--output", "json"}
					buildServer := method != "BeingOnBuildServerDoesntOverrideOutputJson"
					if buildServer {
						args = append(args, "--output", "buildserver")
					}
					if len(tc.Arguments) != 0 {
						args = append(args, "--output", "file")
						if tc.Arguments[0] != "" {
							args = append(args, "--output-file", tc.Arguments[0])
						}
					}
					var stdout, stderr bytes.Buffer
					if err := execute(args, &stdout, &stderr); err != nil || stderr.Len() != 0 {
						t.Fatalf("TeamCity output: %v, %q", err, &stderr)
					}
					data := stdout.Bytes()
					if buildServer {
						message := "##teamcity[buildNumber '" + tc.Expected + "']"
						at := bytes.Index(data, []byte("{\n"))
						if at < 0 || !bytes.Contains(data[:at], []byte(message)) {
							t.Fatalf("expected %s and JSON, got %s", message, data)
						}
						data = data[at:]
					}
					checkVersionJSON(t, data, tc.Expected)
					if len(tc.Arguments) != 0 {
						data, err := os.ReadFile(filepath.Join(local, tc.Arguments[1]))
						if err != nil {
							t.Fatal(err)
						}
						checkVersionJSON(t, data, tc.Expected)
					}
				})
			}
		})
	}
}

func checkVersionJSON(t *testing.T, data []byte, expected string) {
	t.Helper()
	var variables map[string]any
	if err := json.Unmarshal(data, &variables); err != nil || variables["FullSemVer"] != expected {
		t.Fatalf("FullSemVer = %v, want %s, error %v", variables["FullSemVer"], expected, err)
	}
}

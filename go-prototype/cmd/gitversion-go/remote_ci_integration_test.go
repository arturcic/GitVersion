//go:build platform

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"maps"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

const fixtureUser, fixturePassword = "parity-user", "fixture-only secret"

type remoteCIHarness struct {
	baseline                         parity.Baseline
	exe, candidate, dll, report, git string
}

func newRemoteCIHarness(t *testing.T) remoteCIHarness {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var h remoteCIHarness
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &h.baseline); err != nil {
		t.Fatal(err)
	}
	baselineDir := filepath.Join(root, ".local/baseline")
	files, err := parity.FileSet(baselineDir)
	if err != nil || !reflect.DeepEqual(files, h.baseline.Files) {
		t.Fatal("baseline file set mismatch", err)
	}
	if spec, err := parity.FileHash(filepath.Join(root, "baseline.json")); err != nil || spec != h.baseline.SpecSHA256 {
		t.Fatal("baseline specification mismatch", err)
	}
	h.dll = filepath.Join(baselineDir, "gitversion.dll")
	h.exe, h.candidate = os.Getenv("PARITY_DOTNET"), os.Getenv("PARITY_CANDIDATE")
	h.git, err = exec.LookPath("git")
	if err != nil {
		t.Fatal("Git is required for the managed oracle and fixture server", err)
	}
	h.report, err = os.MkdirTemp(os.Getenv("PARITY_EVIDENCE_DIR"), "remote-ci-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", h.report)
	return h
}

func (h remoteCIHarness) run(t *testing.T, implementation, checkout string, env map[string]string, args []string, failure string) []byte {
	t.Helper()
	area := filepath.Join(h.report, filepath.Base(t.Name()), implementation)
	if err := os.MkdirAll(area, 0755); err != nil {
		t.Fatal(err)
	}
	exe := h.candidate
	vars := maps.Clone(h.baseline.Spec.Environment)
	maps.Copy(vars, env)
	verbosity := "quiet"
	if failure != "" {
		verbosity = "minimal"
	}
	args = append([]string{"--no-cache", "--verbosity", verbosity}, args...)
	if implementation == "csharp" {
		exe = h.exe
		args = append([]string{"--fx-version", h.baseline.Spec.Runtime, h.dll}, args...)
		if failure != "" {
			args = append(args, "--log-file", filepath.Join(area, "gitversion.log"))
		}
	} else {
		// The fixture server and C# may use Git. The Go process cannot.
		vars["PATH"] = area
	}
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir, cmd.Env = checkout, parity.Environment(area, vars)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	diagnostics := stdout.String() + stderr.String()
	if failure != "" && implementation == "csharp" {
		log, err := os.ReadFile(filepath.Join(area, "gitversion.log"))
		if err != nil {
			t.Fatal(err)
		}
		diagnostics += string(log)
	}
	for _, secret := range []string{fixturePassword, "wrong-secret", base64.StdEncoding.EncodeToString([]byte(fixtureUser + ":" + fixturePassword)), base64.StdEncoding.EncodeToString([]byte(fixtureUser + ":wrong-secret"))} {
		if strings.Contains(diagnostics, secret) {
			t.Fatal(implementation, "exposed fixture credentials in process output")
		}
	}
	if failure == "" && err != nil {
		t.Fatalf("%s: %v\n%s", implementation, err, diagnostics)
	}
	if failure != "" {
		var exit *exec.ExitError
		matched := strings.Contains(diagnostics, failure)
		if implementation == "csharp" && strings.HasPrefix(failure, "Unauthorized:") {
			// Some Git clients report their suppressed credential prompt for HTTP 401.
			matched = matched || strings.Contains(diagnostics, "terminal prompts disabled")
		}
		if !errors.As(err, &exit) || exit.ExitCode() != 1 || !matched {
			t.Fatalf("%s: expected exit 1 and %q, got %v\n%s", implementation, failure, err, diagnostics)
		}
	}
	if err := os.WriteFile(filepath.Join(area, "stdout"), stdout.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(area, "stderr"), stderr.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return stdout.Bytes()
}

func (h remoteCIHarness) fixtureGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), h.git, args...)
	cmd.Dir = dir
	cmd.Env = parity.Environment(dir, nil)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture git %s: %v: %s", args[0], err, out)
	}
}

func remoteCIHead(t *testing.T, repo *git.Repository) plumbing.Hash {
	t.Helper()
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	return head.Hash()
}

func remoteCIRepo(t *testing.T, path string) *git.Repository {
	t.Helper()
	repo, err := git.PlainOpen(path)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func remoteCICommit(t *testing.T, repo *git.Repository, seq int64) plumbing.Hash {
	t.Helper()
	tree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	sig := &object.Signature{Name: "Fixture", Email: "fixture@example.test", When: time.Unix(1700000000+seq, 0).UTC()}
	sha, err := tree.Commit("advance fixture", &git.CommitOptions{AllowEmptyCommits: true, Author: sig, Committer: sig})
	if err != nil {
		t.Fatal(err)
	}
	return sha
}

func setRemoteCIRef(t *testing.T, repo *git.Repository, ref string, sha plumbing.Hash) {
	t.Helper()
	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(ref), sha)); err != nil {
		t.Fatal(err)
	}
}

func verifyRemoteCIResult(t *testing.T, ref, got []byte, head plumbing.Hash, branch string) {
	t.Helper()
	if diff, err := parity.Compare(ref, got); err != nil || len(diff) != 0 {
		t.Fatalf("JSON differs: %v (%v)", diff, err)
	}
	values, err := parity.Object(got)
	if err != nil || len(values) != 33 || values["Sha"] != head.String() || values["BranchName"] != branch {
		t.Fatalf("expected %s at %s and all 33 fields, got %s (%v)", branch, head, got, err)
	}
}

func TestPlatformRemoteCIParity(t *testing.T) {
	h := newRemoteCIHarness(t)
	cases := []struct {
		name, mode, failure string
		status              int
	}{
		{name: "authenticated-clone", mode: "clone"},
		{name: "incorrect-credentials", mode: "clone", status: 401, failure: "Unauthorized: Incorrect username/password"},
		{name: "forbidden-remote", mode: "clone", status: 403, failure: "Forbidden: Possibly Incorrect username/password"},
		{name: "missing-remote", mode: "clone", status: 404, failure: "Not found: The repository was not found"},
		{name: "authenticated-fetch", mode: "fetch"},
		{name: "unauthorized-fetch", mode: "fetch", status: 401, failure: "Unauthorized: Incorrect username/password"},
		{name: "shallow-fetch", mode: "shallow"},
		{name: "shallow-fetch-rejected", mode: "shallow", failure: "Repository is a shallow clone. Git repositories must contain the full history."},
		{name: "shallow-no-fetch", mode: "shallow-no-fetch"},
		{name: "jenkins-duplicate-origin", mode: "jenkins"},
		{name: "multiple-remotes-rejected", mode: "multi", failure: "2 remote(s) have been detected."},
		{name: "missing-ci-remote", mode: "no-remote", failure: "0 remote(s) have been detected."},
		{name: "github-detached-provider-pr", mode: "provider"},
		{name: "detached-advertised-pr", mode: "advertised"},
		{name: "detached-advertised-annotated-tag", mode: "tag"},
		{name: "detached-local-annotated-tag", mode: "local-tag"},
		{name: "detached-stale-local-branch", mode: "stale"},
		{name: "detached-ignored-tracking-branch", mode: "ignored"},
		{name: "detached-ordinary-remote-tip", mode: "ordinary", failure: "doesn't look like a valid pull request"},
		{name: "detached-no-remote-tip", mode: "no-tip", failure: "Couldn't find any remote tips"},
		{name: "detached-ambiguous-advertisements", mode: "ambiguous", failure: "Found more than one remote tip"},
	}
	comparisons, jsonComparisons := 0, 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scratch := t.TempDir()
			source := filepath.Join(scratch, "upstream.git")
			prepareExecutorCacheRepo(t, source)
			remote := remoteCIRepo(t, source)
			initial := remoteCIHead(t, remote)
			before := remoteCICommit(t, remote, 1)
			var requests, authenticated atomic.Int32
			handler := &cgi.Handler{Path: h.git, Args: []string{"http-backend"}, Env: []string{
				"GIT_PROJECT_ROOT=" + scratch, "GIT_HTTP_EXPORT_ALL=1", "PATH=" + os.Getenv("PATH"),
			}}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				user, password, ok := r.BasicAuth()
				if !ok || user != fixtureUser || password != fixturePassword {
					w.Header().Set("WWW-Authenticate", "Basic realm=\"Git fixture\"")
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				authenticated.Add(1)
				if tc.status != 0 {
					w.WriteHeader(tc.status)
					return
				}
				handler.ServeHTTP(w, r)
			}))
			defer server.Close()
			url := server.URL + "/upstream.git"
			outputs := map[string][]byte{}
			wantHead, wantBranch := before, "main"
			seed := filepath.Join(scratch, "seed")
			shallow := strings.HasPrefix(tc.mode, "shallow")
			if shallow {
				h.fixtureGit(t, scratch, "clone", "--quiet", "--depth=1", "--branch", "main", "file://"+source, seed)
				if _, err := remoteCIRepo(t, seed).CommitObject(initial); err == nil {
					t.Fatal("shallow seed contains the ancestor")
				}
			} else if tc.mode != "clone" {
				if err := fixtures.Copy(source, seed); err != nil {
					t.Fatal(err)
				}
			}
			advanced := remoteCICommit(t, remote, 2)
			if shallow {
				setRemoteCIRef(t, remote, "refs/heads/feature/fetched", advanced)
			}
			branch := "refs/pull/42/merge"
			switch tc.mode {
			case "provider", "advertised", "ambiguous", "tag", "local-tag", "ordinary", "no-tip", "stale", "ignored":
				setRemoteCIRef(t, remote, branch, advanced)
				setRemoteCIRef(t, remote, "refs/heads/main", before)
				if tc.mode == "ambiguous" {
					setRemoteCIRef(t, remote, "refs/pull/43/merge", advanced)
				}
				// Seed the detached commit without giving the checkout a local branch at it.
				h.fixtureGit(t, seed, "fetch", "--quiet", source, branch)
				h.fixtureGit(t, seed, "checkout", "--quiet", "--detach", advanced.String())
				wantHead, wantBranch = advanced, "pull/42/merge"
				switch tc.mode {
				case "tag", "local-tag", "ordinary", "no-tip":
					if err := remote.Storer.RemoveReference(plumbing.ReferenceName(branch)); err != nil {
						t.Fatal(err)
					}
				}
				if tc.mode == "tag" || tc.mode == "local-tag" {
					sig := &object.Signature{Name: "Fixture", Email: "fixture@example.test", When: time.Unix(1700000003, 0).UTC()}
					tag, err := remote.CreateTag("2.0.0", advanced, &git.CreateTagOptions{Tagger: sig, Message: "release"})
					if err != nil {
						t.Fatal(err)
					}
					h.fixtureGit(t, seed, "fetch", "--quiet", source, tag.Name().String()+":"+tag.Name().String())
					wantBranch = "(no branch)"
				} else if tc.mode == "stale" {
					wantBranch = "feature/ci"
				} else if tc.mode == "ordinary" {
					setRemoteCIRef(t, remote, "refs/heads/untracked", advanced)
				}
			case "clone":
				wantHead = advanced
			}
			for _, implementation := range []string{"csharp", "go"} {
				checkout := filepath.Join(scratch, implementation)
				env := map[string]string{"ContinuaCI.Version": "true"}
				password := fixturePassword
				if tc.status == http.StatusUnauthorized {
					password = "wrong-secret"
				}
				args := []string{"--username", fixtureUser, "--password", password}
				if tc.mode == "clone" {
					if err := os.Mkdir(checkout, 0755); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--url", url, "--dynamic-repo-location", filepath.Join(checkout, "clones"), "--branch", "main", "--no-fetch")
				} else {
					if err := fixtures.Copy(seed, checkout); err != nil {
						t.Fatal(err)
					}
					repo := remoteCIRepo(t, checkout)
					if shallow {
						h.fixtureGit(t, checkout, "remote", "set-url", "origin", url)
					} else if tc.mode != "no-remote" {
						remoteURL := url
						if tc.mode == "local-tag" {
							remoteURL = "../upstream.git"
						}
						if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{remoteURL}}); err != nil {
							t.Fatal(err)
						}
					}
					switch tc.mode {
					case "jenkins", "multi":
						if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin1", URLs: []string{url}}); err != nil {
							t.Fatal(err)
						}
						args = append(args, "--no-fetch")
						if tc.mode == "jenkins" {
							env = map[string]string{"JENKINS_URL": "fixture", "BRANCH_NAME": "feature/ci"}
							setRemoteCIRef(t, repo, "refs/remotes/origin/feature/ci", before)
							setRemoteCIRef(t, repo, "refs/remotes/origin1/feature/ci", initial)
							wantBranch = "feature/ci"
						}
					case "provider":
						env = map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF": branch}
					case "stale", "ignored":
						setRemoteCIRef(t, repo, "refs/heads/feature/ci", before)
						setRemoteCIRef(t, repo, "refs/remotes/origin/feature/ci", advanced)
						args = append(args, "--no-fetch")
						if tc.mode == "ignored" {
							data := []byte("calculation:\n  ignore:\n    branches: ['^feature/ci$']\n")
							if err := os.WriteFile(filepath.Join(checkout, "GitVersion.yml"), data, 0600); err != nil {
								t.Fatal(err)
							}
						}
					case "advertised", "ambiguous", "tag", "local-tag", "ordinary", "no-tip", "shallow-no-fetch":
						args = append(args, "--no-fetch")
					}
					if shallow || tc.mode == "fetch" {
						env["GIT_BRANCH"] = "main"
					}
					if shallow && tc.failure == "" {
						args = append(args, "--allow-shallow")
					}
				}
				n := requests.Load()
				auth := authenticated.Load()
				invokingDirectory := checkout
				if tc.mode == "local-tag" {
					invokingDirectory = scratch
					args = append(args, "--target-path", checkout)
				}
				outputs[implementation] = h.run(t, implementation, invokingDirectory, env, args, tc.failure)
				if tc.mode == "fetch" && tc.failure != "" {
					repo := remoteCIRepo(t, checkout)
					_, err := repo.Reference("refs/remotes/origin/main", true)
					if !errors.Is(err, plumbing.ErrReferenceNotFound) || remoteCIHead(t, repo) != before {
						t.Fatalf("%s: rejected fetch changed the checkout or published a tracking ref (%v)", implementation, err)
					}
				}
				var wantRequest bool
				switch tc.mode {
				case "clone", "fetch", "shallow", "advertised", "ambiguous", "tag", "ordinary", "no-tip", "ignored":
					wantRequest = true
				}
				if (requests.Load() > n) != wantRequest || wantRequest && (authenticated.Load() > auth) != (tc.status != http.StatusUnauthorized) {
					t.Fatalf("%s: unexpected HTTP requests or missing authentication", implementation)
				}
				if shallow || tc.mode == "fetch" && tc.failure == "" {
					repo := remoteCIRepo(t, checkout)
					ref, err := repo.Reference("refs/remotes/origin/main", true)
					want := advanced
					if tc.mode == "shallow-no-fetch" {
						want = before
					}
					if err != nil || ref.Hash() != want || remoteCIHead(t, repo) != before {
						t.Fatalf("tracking ref %v, want %s and checkout stays at %s (%v)", ref, want, before, err)
					}
					if shallow {
						extra, extraErr := repo.Reference("refs/remotes/origin/feature/fetched", true)
						if tc.mode == "shallow-no-fetch" {
							if !errors.Is(extraErr, plumbing.ErrReferenceNotFound) {
								t.Fatalf("unexpected tracking ref: %v (%v)", extra, extraErr)
							}
						} else if extraErr != nil || extra.Hash() != advanced {
							t.Fatalf("single-branch shallow clone did not fetch new branch: %v (%v)", extra, extraErr)
						}
						if _, err := repo.CommitObject(initial); !errors.Is(err, plumbing.ErrObjectNotFound) {
							t.Fatalf("%s: ordinary fetch retrieved the parent behind the shallow boundary (%v)", implementation, err)
						}
						data, err := os.ReadFile(filepath.Join(checkout, ".git", "shallow"))
						if err != nil || string(data) != before.String()+"\n" {
							t.Fatalf("shallow boundary changed: %q (%v)", data, err)
						}
					}
				}
				if tc.mode == "jenkins" {
					_, err := remoteCIRepo(t, checkout).Reference("refs/remotes/origin1/feature/ci", true)
					if implementation == "go" && err != nil || implementation == "csharp" && !errors.Is(err, plumbing.ErrReferenceNotFound) {
						t.Fatalf("%s: unexpected duplicate remote ref state: %v", implementation, err)
					}
				}
				if tc.mode == "stale" {
					repo := remoteCIRepo(t, checkout)
					local, err := repo.Reference("refs/heads/feature/ci", true)
					wantLocal := before
					if implementation == "csharp" {
						wantLocal = advanced
					}
					if err != nil || local.Hash() != wantLocal || remoteCIHead(t, repo) != advanced {
						t.Fatalf("%s: local ref %v, want %s and checkout commit %s (%v)", implementation, local, wantLocal, advanced, err)
					}
				}
				if tc.failure == "" {
					configPath := filepath.Join(checkout, ".git", "config")
					if tc.mode == "clone" {
						configPath = filepath.Join(checkout, "clones", "upstream", ".git", ".git", "config")
					}
					config, err := os.ReadFile(configPath)
					if err != nil || bytes.Contains(config, []byte(fixtureUser)) || bytes.Contains(config, []byte(fixturePassword)) {
						t.Fatalf("%s: clone/fetch persisted fixture credentials or config unavailable: %v", implementation, err)
					}
				}
			}
			if tc.failure == "" {
				verifyRemoteCIResult(t, outputs["csharp"], outputs["go"], wantHead, wantBranch)
				jsonComparisons++
			}
			comparisons++
		})
	}
	if !t.Failed() && comparisons == len(cases) {
		hash, err := parity.FileHash(h.candidate)
		if err != nil {
			t.Fatal(err)
		}
		if err := parity.WriteJSON(filepath.Join(h.report, "report.json"), map[string]any{
			"comparisons": comparisons, "json_comparisons": jsonComparisons, "rejection_comparisons": comparisons - jsonComparisons, "baseline_revision": h.baseline.Spec.Revision, "baseline_files": h.baseline.Files,
			"candidate_sha256": hash, "spec_sha256": h.baseline.SpecSHA256, "csharp_backend": "managed", "go_git_on_path": false,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

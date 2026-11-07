//go:build repositoryoracle

package gitrepo

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

// This gate runs the same LibGit2Sharp expressions as the original C# tests.
// Capture additionally freezes the three Git CLI expectations for offline replay.
func TestPlatformRepositoryContractOracle(t *testing.T) {
	corpus := readContractCorpus(t)
	output := os.Getenv("CONTRACT_REFERENCE_OUT")
	var reference contractReference
	if output == "" {
		reference = readContractReference(t, corpus)
	} else {
		reference = contractReference{Revision: corpus.Revision, Cases: map[string]json.RawMessage{}}
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	oracle := filepath.Join(root, ".local/repository-contract-oracle")
	hash, err := parity.FileHash(filepath.Join(oracle, "LibGit2Sharp.dll"))
	if err != nil || hash != baseline.Files["LibGit2Sharp.dll"] {
		t.Fatal("oracle library differs from the pinned baseline", err)
	}
	native := filepath.Join(oracle, filepath.Base(os.Getenv("CONTRACT_NATIVE_LIBRARY")))
	nativeHash, err := parity.FileHash(native)
	if err != nil {
		t.Fatal(err)
	}
	assembly := filepath.Join(oracle, "RepositoryContracts.dll")
	assemblyHash, err := parity.FileHash(assembly)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := os.MkdirTemp(filepath.Join(root, ".local"), "repository-contract-oracle-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", workspace)
	run := func(t *testing.T, path, exe string, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, exe, args...)
		cmd.Dir = path
		cmd.Env = parity.Environment(os.Getenv("HOME"), nil)
		data, err := cmd.Output()
		if err != nil {
			if failure, ok := err.(*exec.ExitError); ok {
				t.Fatalf("%v: %s", err, failure.Stderr)
			}
			t.Fatal(err)
		}
		return data
	}
	results := map[string]json.RawMessage{}
	for _, suite := range []string{"GitStatusCalculatorTests", "GitTreeDiffTests", "GitReferenceStoreTests"} {
		names := []string{}
		for name := range corpus.Suites[suite].Methods {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			var fixture *contractFixture
			var mode string
			switch suite {
			case "GitStatusCalculatorTests":
				fixture = contractStatusFixture(t, name)
				mode = "status"
			case "GitTreeDiffTests":
				if name == "IdenticalTreesProduceNoChangedPaths" || (name == "FlattenTreeListsAllBlobPaths" && output == "") {
					continue
				}
				fixture = contractTreeFixture(t, name)
				mode = "diff"
			case "GitReferenceStoreTests":
				if output == "" || (name != "EnumerationMatchesGitForEachRef" && name != "ReadsPackedRefsIncludingPeeledTargets") {
					continue
				}
				fixture, _, _ = contractReferenceFixture(t, name)
			}
			t.Run(name, func(t *testing.T) {
				var raw []byte
				switch name {
				case "EnumerationMatchesGitForEachRef":
					raw, err = json.Marshal(strings.Split(strings.TrimSuffix(string(run(t, fixture.path, "git", "for-each-ref", "--format=%(refname) %(objectname)")), "\n"), "\n"))
				case "ReadsPackedRefsIncludingPeeledTargets":
					raw, err = json.Marshal(strings.Fields(string(run(t, fixture.path, "git", "rev-parse", "v1.0.0", "v1.0.0^{}"))))
				case "FlattenTreeListsAllBlobPaths":
					raw, err = json.Marshal(strings.TrimSpace(string(run(t, fixture.path, "git", "rev-parse", "HEAD:a.txt"))))
				default:
					raw = run(t, fixture.path, os.Getenv("PARITY_DOTNET"), "--fx-version", baseline.Spec.Runtime, assembly, mode, fixture.path)
				}
				if err != nil || !json.Valid(raw) {
					t.Fatal("oracle output is invalid", err, string(raw))
				}
				results[name] = json.RawMessage(raw)
				if output != "" {
					reference.Cases[name] = json.RawMessage(raw)
				} else {
					var expected, actual any
					if err := json.Unmarshal(reference.Cases[name], &expected); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(raw, &actual); err != nil || !reflect.DeepEqual(actual, expected) {
						t.Fatalf("fresh oracle differs: expected %v, actual %v, error %v", expected, actual, err)
					}
				}
			})
		}
	}
	if t.Failed() {
		return
	}
	expectedCount := 16
	if output != "" {
		expectedCount = 19
	}
	if len(results) != expectedCount {
		t.Fatal("oracle case count differs", len(results))
	}
	if output != "" {
		if err := parity.WriteJSON(output, reference); err != nil {
			t.Fatal(err)
		}
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "report.json"), map[string]any{"mode": "repository-contract-oracle", "passed": len(results), "failed": 0, "cases": results}); err != nil {
		t.Fatal(err)
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "provenance.json"), map[string]string{"platform": parity.Platform(), "baseline_revision": baseline.Spec.Revision, "libgit2sharp_sha256": hash, "native_sha256": nativeHash, "oracle_sha256": assemblyHash}); err != nil {
		t.Fatal(err)
	}
}

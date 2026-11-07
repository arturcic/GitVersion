//go:build repositoryoracle

package gitrepo

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformRevisionWalkerOracle(t *testing.T) {
	corpus := readRevisionCorpus(t)
	output := os.Getenv("REVISION_REFERENCE_OUT")
	reference := contractReference{Revision: corpus.Revision, Cases: map[string]json.RawMessage{}}
	if output == "" {
		reference = readRevisionReference(t, corpus)
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
	library, err := parity.FileHash(filepath.Join(oracle, "LibGit2Sharp.dll"))
	if err != nil || library != baseline.Files["LibGit2Sharp.dll"] {
		t.Fatal("oracle library differs from pinned baseline", err)
	}
	native, err := parity.FileHash(filepath.Join(oracle, filepath.Base(os.Getenv("CONTRACT_NATIVE_LIBRARY"))))
	if err != nil {
		t.Fatal(err)
	}
	assembly := filepath.Join(oracle, "RepositoryContracts.dll")
	assemblyHash, err := parity.FileHash(assembly)
	if err != nil {
		t.Fatal(err)
	}
	results := map[string][][]string{}
	for method, spec := range corpus.Methods {
		if revisionHistory(method) == "" {
			continue
		}
		for _, tc := range spec.Cases {
			key := method + "/" + tc.ID
			t.Run(key, func(t *testing.T) {
				fixture, queries := revisionQueries(t, method, tc.Argument)
				result := make([][]string, len(queries))
				for i, q := range queries {
					func() {
						ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
						defer cancel()
						args := q.arguments()
						runArgs := append([]string{"--fx-version", baseline.Spec.Runtime, assembly, args[0], fixture.path}, args[1:]...)
						cmd := exec.CommandContext(ctx, os.Getenv("PARITY_DOTNET"), runArgs...)
						cmd.Dir = fixture.path
						cmd.Env = parity.Environment(os.Getenv("HOME"), nil)
						data, err := cmd.CombinedOutput()
						if err != nil {
							t.Fatalf("C# oracle: %v: %s", err, data)
						}
						if q.base {
							var base string
							if err := json.Unmarshal(data, &base); err != nil || base == "" {
								t.Fatalf("invalid C# merge base %s, error %v", data, err)
							}
							result[i] = []string{base}
						} else if err := json.Unmarshal(data, &result[i]); err != nil || len(result[i]) == 0 {
							t.Fatalf("invalid C# walk %s, error %v", data, err)
						}
					}()
				}
				results[key] = result
				if output != "" {
					data, err := json.Marshal(result)
					if err != nil {
						t.Fatal(err)
					}
					reference.Cases[key] = data
				} else {
					var expected [][]string
					if err := json.Unmarshal(reference.Cases[key], &expected); err != nil || !reflect.DeepEqual(result, expected) {
						t.Fatalf("fresh C# oracle differs: expected %v, got %v, error %v", expected, result, err)
					}
				}
			})
		}
	}
	if t.Failed() {
		return
	}
	if len(results) != 47 {
		t.Fatal("unexpected number of oracle cases", len(results))
	}
	if output != "" {
		if err := parity.WriteJSON(output, reference); err != nil {
			t.Fatal(err)
		}
	}
	workspace, err := os.MkdirTemp(filepath.Join(root, ".local"), "revision-walker-oracle-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", workspace)
	if err := parity.WriteJSON(filepath.Join(workspace, "report.json"), map[string]any{"mode": "revision-walker-oracle", "passed": len(results), "failed": 0, "cases": results}); err != nil {
		t.Fatal(err)
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "provenance.json"), map[string]string{"revision": corpus.Revision, "native_sha256": native, "oracle_sha256": assemblyHash, "libgit2sharp_sha256": library}); err != nil {
		t.Fatal(err)
	}
}

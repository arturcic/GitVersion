//go:build repositoryoracle

package gitrepo

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformDualBackendOracle(t *testing.T) {
	corpus := readDualBackendCorpus(t)
	output := os.Getenv("DUAL_BACKEND_REFERENCE_OUT")
	reference := dualBackendReference{Revision: corpus.Revision, Cases: map[string]json.RawMessage{}}
	if output == "" {
		reference = readDualBackendReference(t, corpus)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	published := filepath.Join(root, ".local/baseline")
	files, err := parity.FileSet(published)
	if err != nil || !reflect.DeepEqual(files, baseline.Files) {
		t.Fatal("pinned C# assembly set changed", err)
	}
	oracle := filepath.Join(root, ".local/dual-backend-oracle")
	copies, err := parity.FileSet(oracle)
	if err != nil {
		t.Fatal(err)
	}
	for name, hash := range copies {
		if expected, ok := baseline.Files[name]; ok && hash != expected {
			t.Fatal("oracle dependency differs from the pinned baseline", name)
		}
	}
	var native, expectedNative string
	if runtime.GOOS == "windows" {
		native = "git2-5853918.dll"
		arch := runtime.GOARCH
		switch arch {
		case "amd64":
			arch = "x64"
		case "386":
			arch = "x86"
		}
		expectedNative = baseline.Files["runtimes/win-"+arch+"/native/"+native]
	} else {
		native = "libgit2-5853918.so"
		if runtime.GOOS == "darwin" {
			native = "libgit2-5853918.dylib"
		}
		expectedNative, err = parity.FileHash(filepath.Join(root, ".local/repository-contract-oracle", native))
		if err != nil {
			t.Fatal(err)
		}
	}
	if expectedNative == "" || copies[native] != expectedNative {
		t.Fatalf("native oracle library %s differs: got %q, want %q", native, copies[native], expectedNative)
	}
	assembly := filepath.Join(oracle, "DualBackend.dll")
	home := t.TempDir()
	results := map[string]json.RawMessage{}
	for method := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			fixture := newDualBackendFixture(t, method)
			before, err := fixtures.Digest(fixture.root)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Getenv("PARITY_DOTNET"), "--fx-version", baseline.Spec.Runtime,
				assembly, method, fixture.path, fixture.root, published)
			cmd.Dir = fixture.path
			cmd.Env = parity.Environment(home, baseline.Spec.Environment)
			raw, err := cmd.Output()
			if err != nil {
				if failed, ok := err.(*exec.ExitError); ok {
					t.Fatalf("pinned adapters failed: %v: %s", err, failed.Stderr)
				}
				t.Fatal(err)
			}
			if !json.Valid(raw) {
				t.Fatalf("invalid C# oracle output: %s", raw)
			}
			after, err := fixtures.Digest(fixture.root)
			if err != nil || after != before {
				t.Fatal("C# comparison changed the fixture", err)
			}
			results[method] = json.RawMessage(raw)
			if output != "" {
				reference.Cases[method] = json.RawMessage(raw)
			} else if diff, err := compareDualBackend(reference.Cases[method], raw); err != nil || len(diff) != 0 {
				t.Fatalf("fresh C# result differs: %v %+v", err, diff)
			}
		})
	}
	if t.Failed() {
		return
	}
	if len(results) != 19 {
		t.Fatal("unexpected number of C# results", len(results))
	}
	if output != "" {
		if err := parity.WriteJSON(output, reference); err != nil {
			t.Fatal(err)
		}
	}
	workspace, err := os.MkdirTemp(filepath.Join(root, ".local"), "dual-backend-oracle-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", workspace)
	if err := parity.WriteJSON(filepath.Join(workspace, "report.json"),
		map[string]any{"mode": "dual-backend-oracle", "passed": len(results), "failed": 0, "cases": results}); err != nil {
		t.Fatal(err)
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "provenance.json"),
		map[string]any{"source_revision": corpus.Revision, "published_files": files, "oracle_files": copies}); err != nil {
		t.Fatal(err)
	}
}

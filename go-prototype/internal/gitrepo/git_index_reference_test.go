//go:build repositoryoracle

package gitrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestGitIndexReference(t *testing.T) {
	corpus := readIndexStreamCorpus(t)
	got := indexReference{Revision: corpus.Revision, Versions: map[string][]string{}}
	home := t.TempDir()
	for _, version := range []uint32{2, 3, 4} {
		name := strconv.FormatUint(uint64(version), 10)
		f := indexVersionFixture(t, version)
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		command := exec.CommandContext(ctx, "git", "ls-files", "--stage")
		command.Dir = f.path
		command.Env = parity.Environment(home, nil)
		data, err := command.Output()
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		got.Versions[name] = strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		if len(got.Versions[name]) != 3 {
			t.Fatal("Git index entry count changed", name, got.Versions[name])
		}
	}
	if output := os.Getenv("GIT_INDEX_REFERENCE_OUT"); output != "" {
		if err := parity.WriteJSON(output, got); err != nil {
			t.Fatal(err)
		}
	} else if expected := readIndexReference(t, corpus); !reflect.DeepEqual(got, expected) {
		t.Fatal("fresh Git index expectations differ from the portable reference")
	}
}

func TestPlatformIndexAssumeValidParity(t *testing.T) {
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
		t.Fatal("published C# artifacts changed", err)
	}
	exe := os.Getenv("PARITY_CANDIDATE")
	hash, err := parity.FileHash(exe)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := os.MkdirTemp(filepath.Join(root, ".local"), "index-assume-valid-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", workspace)
	commands := []parity.Command{
		{Executable: os.Getenv("PARITY_DOTNET"), Arguments: append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(published, "gitversion.dll")}, baseline.Spec.Arguments...)},
		{Executable: exe, Arguments: []string{"calculate", "--repository", "{repo}"}},
	}
	var passed []string
	for _, version := range []uint32{2, 3, 4} {
		name := fmt.Sprintf("index-v%d", version)
		if t.Run(name, func(t *testing.T) {
			f := assumeValidFixture(t, version, false)
			var outputs [][]byte
			for i, command := range commands {
				path := filepath.Join(workspace, fmt.Sprintf("%s-%d", name, i))
				result, err := parity.RunInWorkingDirectory(t.Context(), command, f.path, path, baseline.Spec.Environment, 30*time.Second)
				if err != nil {
					t.Fatalf("process %d: %v (%s)", i, err, result.Stderr)
				}
				outputs = append(outputs, result.Stdout)
			}
			diff, err := parity.Compare(outputs[0], outputs[1])
			if err != nil || len(diff) != 0 {
				t.Fatalf("C# and Go index outcomes differ: %v %+v", err, diff)
			}
			var values map[string]any
			if err := json.Unmarshal(outputs[0], &values); err != nil || len(values) != 33 ||
				values["UncommittedChanges"] != float64(0) {
				t.Fatalf("assume-valid result should report zero changes, got %v, error %v", values["UncommittedChanges"], err)
			}
		}) {
			passed = append(passed, name)
		}
	}
	report := map[string]any{"passed": len(passed), "failed": 3 - len(passed), "cases": passed, "candidate_sha256": hash}
	if err := parity.WriteJSON(filepath.Join(workspace, "report.json"), report); err != nil {
		t.Fatal(err)
	}
}

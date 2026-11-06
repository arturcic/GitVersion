//go:build expansion

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPublicHistoryReplayAndRepeatability(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	if err := json.Unmarshal([]byte(os.Getenv("PARITY_PUBLIC_SOURCES")), &sources); err != nil {
		t.Fatal("PARITY_PUBLIC_SOURCES must map manifest repository names to local paths:", err)
	}
	var cases []fixtures.PublicCase
	manifest := filepath.Join(root, "testdata/public-history.json")
	if err := parity.ReadJSON(manifest, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 6 {
		t.Fatal("public manifest inventory changed")
	}
	var baseline parity.Baseline
	lock := filepath.Join(root, ".local/baseline.lock.json")
	if err := parity.ReadJSON(lock, &baseline); err != nil {
		t.Fatal(err)
	}
	dotnet := os.Getenv("PARITY_DOTNET")
	baseDir := filepath.Join(root, ".local/baseline")
	if err := parity.VerifyBaseline(baseline, filepath.Join(root, "baseline.json"), dotnet, baseDir); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, ".local/gitversion-go")
	candidateHash, err := parity.FileHash(exe)
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(root, ".local/public-evidence")
	if err := os.MkdirAll(parent, 0755); err != nil {
		t.Fatal(err)
	}
	workspace, err := os.MkdirTemp(parent, "run-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", workspace)
	commands := []parity.Command{
		{Executable: dotnet, Arguments: append([]string{"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll")}, baseline.Spec.Arguments...)},
		{Executable: exe, Arguments: []string{"calculate", "--repository", "{repo}"}},
	}
	checkpoints := []fixtures.Checkpoint{}
	benchmarks := map[string][][]parity.Result{}
	for _, spec := range cases {
		t.Run(spec.ID, func(t *testing.T) {
			source := sources[spec.Repository]
			if source == "" {
				t.Fatal("missing local source", spec.Repository)
			}
			cp, err := fixtures.PublicSnapshot(source, filepath.Join(workspace, spec.ID), spec)
			if err != nil {
				t.Fatal(err)
			}
			checkpoints = append(checkpoints, cp)
			var expected []byte
			// Initial parity plus relocated, changed-locale repeats for both tools.
			for repeat := 0; repeat < 3; repeat++ {
				for j, command := range commands {
					env := map[string]string{}
					for k, v := range baseline.Spec.Environment {
						env[k] = v
					}
					if repeat > 0 {
						env["TZ"] = "Pacific/Honolulu"
						env["LANG"] = "tr_TR.UTF-8"
						env["LC_ALL"] = "tr_TR.UTF-8"
						env["PATH"] = ""
					}
					path := filepath.Join(workspace, fmt.Sprintf("%s-repeat-%d-%d", spec.ID, repeat, j))
					result, err := parity.Run(t.Context(), command, cp.Path, path, env, 30*time.Second)
					if err != nil {
						t.Fatal(err)
					}
					if err := saveResult(path, result); err != nil {
						t.Fatal(err)
					}
					if err := result.Validate(); err != nil {
						t.Fatalf("calculator %d: %v %s", j, err, result.Stderr)
					}
					if expected == nil {
						expected = result.Stdout
					}
					diff, err := parity.Compare(expected, result.Stdout)
					if err != nil || len(diff) > 0 {
						t.Fatalf("repeat %d tool %d: %+v %v", repeat, j, diff, err)
					}
				}
			}
			// Only cases that already passed parity are timed. Alternate launch
			// order, disable caches, exclude copying from startup-inclusive time.
			if spec.ID == "actions-root" || spec.ID == "actions-intermediate" {
				samples := make([][]parity.Result, 2)
				for n := 0; n < 20; n++ {
					for slot := 0; slot < 2; slot++ {
						j := (slot + n) % 2
						path := filepath.Join(workspace, fmt.Sprintf("%s-benchmark-%02d-%d", spec.ID, n, j))
						result, err := parity.Run(t.Context(), commands[j], cp.Path, path, baseline.Spec.Environment, 30*time.Second)
						if err != nil {
							t.Fatal(err)
						}
						if err := saveResult(path, result); err != nil {
							t.Fatal(err)
						}
						if err := result.Validate(); err != nil {
							t.Fatal(err)
						}
						diff, err := parity.Compare(expected, result.Stdout)
						if err != nil || len(diff) > 0 {
							t.Fatalf("benchmark output changed: %+v %v", diff, err)
						}
						samples[j] = append(samples[j], result)
					}
				}
				benchmarks[spec.ID] = samples
			}
		})
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "checkpoints.json"), checkpoints); err != nil {
		t.Fatal(err)
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "benchmark-samples.json"), benchmarks); err != nil {
		t.Fatal(err)
	}
	after, err := parity.FileHash(exe)
	if err != nil || after != candidateHash {
		t.Fatal("candidate changed")
	}
	if err := parity.VerifyBaseline(baseline, filepath.Join(root, "baseline.json"), dotnet, baseDir); err != nil {
		t.Fatal(err)
	}
	manifestHash, err := parity.FileHash(manifest)
	if err != nil {
		t.Fatal(err)
	}
	lockHash, err := parity.FileHash(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := parity.WriteJSON(filepath.Join(workspace, "provenance.json"), map[string]any{"complete": !t.Failed(), "cases": len(cases), "candidate_sha256": candidateHash, "manifest_sha256": manifestHash, "baseline_lock_sha256": lockHash, "platform": parity.Platform(), "benchmark_condition": "warm filesystem cache; startup inclusive; 20 interleaved runs per tool/case; no result cache; no trace", "config_policy": "synthetic frozen GitHubFlow overlay; projected branch refs and explicitly selected real tags"}); err != nil {
		t.Fatal(err)
	}
}

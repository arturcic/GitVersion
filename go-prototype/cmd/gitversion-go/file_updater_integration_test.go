//go:build platform

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

type fileUpdaterCase struct {
	name, path, original   string
	explicit, ensure, skip bool
}

func testFileUpdaterParity(t *testing.T, kind, flag string, cases []fileUpdaterCase) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	baseDir := filepath.Join(root, ".local/baseline")
	var baseline parity.Baseline
	if err := parity.ReadJSON(filepath.Join(root, ".local/baseline.lock.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	files, err := parity.FileSet(baseDir)
	if err != nil || !reflect.DeepEqual(files, baseline.Files) {
		t.Fatal("baseline file set mismatch", err)
	}
	spec, err := parity.FileHash(filepath.Join(root, "baseline.json"))
	if err != nil || spec != baseline.SpecSHA256 {
		t.Fatal("baseline specification mismatch", err)
	}
	candidate := os.Getenv("PARITY_CANDIDATE")
	hash, err := parity.FileHash(candidate)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(os.Getenv("PARITY_EVIDENCE_DIR"), kind+"-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caseDir := filepath.Join(dir, tc.name)
			if err := os.Mkdir(caseDir, 0755); err != nil {
				t.Fatal(err)
			}
			repositories, err := fixtures.Generate([]fixtures.Recipe{{
				ID: kind, Steps: []fixtures.Step{
					{Op: "commit", ID: "root"}, {Op: "tag", Name: "1.2.3"}, {Op: "checkpoint", ID: "head"},
				},
			}}, filepath.Join(caseDir, "fixtures"))
			if err != nil {
				t.Fatal(err)
			}
			repo := repositories[0].Path
			ignore := filepath.Join(repo, ".git", "info", "exclude")
			if err := os.MkdirAll(filepath.Dir(ignore), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ignore, []byte("/"+tc.path+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if tc.original != "" {
				if err := os.WriteFile(filepath.Join(repo, tc.path), []byte(tc.original), 0644); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"{repo}", "--output", "json", "--no-cache", "--no-fetch", "--no-normalize", flag}
			if tc.explicit {
				args = append(args, tc.path)
			}
			if tc.ensure {
				args = append(args, "--ensure-assembly-info")
			}
			run := func(name, executable string, arguments []string) (parity.Result, []byte) {
				t.Helper()
				path := filepath.Join(caseDir, name)
				res, err := parity.Run(t.Context(), parity.Command{Executable: executable, Arguments: arguments},
					repo, path, baseline.Spec.Environment, 30*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				if err := parity.WriteJSON(filepath.Join(path, "process.json"), res); err != nil {
					t.Fatal(err)
				}
				for file, data := range map[string][]byte{"stdout": res.Stdout, "stderr": res.Stderr} {
					if err := os.WriteFile(filepath.Join(path, file), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				wantError := "repository mutated"
				if tc.skip {
					wantError = ""
				}
				if res.Error != wantError || res.ExitCode != 0 || (!tc.skip && len(res.Stderr) != 0) {
					t.Fatalf("%s exit %d: %s, stderr %q", name, res.ExitCode, res.Error, res.Stderr)
				}
				updated, err := os.ReadFile(filepath.Join(path, "repo", tc.path))
				if err != nil {
					t.Fatal(err)
				}
				if unchanged := bytes.Equal(updated, []byte(tc.original)); unchanged != tc.skip {
					t.Fatalf("%s unchanged %s = %v, want %v", name, tc.path, unchanged, tc.skip)
				}
				return res, updated
			}
			ref, refFile := run("csharp", os.Getenv("PARITY_DOTNET"), append([]string{
				"--fx-version", baseline.Spec.Runtime, filepath.Join(baseDir, "gitversion.dll"),
			}, args...))
			got, gotFile := run("go", candidate, args)
			diff, err := parity.Compare(ref.Stdout, got.Stdout)
			if err != nil || len(diff) != 0 {
				t.Fatalf("JSON mismatch: %v, %v", diff, err)
			}
			if tc.skip {
				logPrefix := regexp.MustCompile(`(?m)^WARN \[[^\]\n]+\] `)
				normalize := func(data []byte, implementation string) string {
					repoDir := filepath.Join(caseDir, implementation, "repo")
					return strings.ReplaceAll(logPrefix.ReplaceAllString(string(data), ""), repoDir, "{repo}")
				}
				expected := normalize(ref.Stderr, "csharp")
				if expected == "" || normalize(got.Stderr, "go") != expected {
					t.Fatalf("warning mismatch\nC# %q\nGo %q", ref.Stderr, got.Stderr)
				}
			}
			if !bytes.Equal(refFile, gotFile) || ref.Before != got.Before || ref.After != got.After {
				t.Fatalf("file updater changed different bytes or files\nC# %q\nGo %q", refFile, gotFile)
			}
		})
	}
	if !t.Failed() {
		if err := parity.WriteJSON(filepath.Join(dir, "report.json"), map[string]any{
			"comparisons": len(cases), "baseline_revision": baseline.Spec.Revision, "candidate_sha256": hash,
			"baseline_files": files, "spec_sha256": spec,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

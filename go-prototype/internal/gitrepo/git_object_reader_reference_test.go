//go:build repositoryoracle

package gitrepo

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/go-git/go-git/v5/plumbing"
)

func TestGitObjectReaderReference(t *testing.T) {
	corpus := readObjectReaderCorpus(t)
	reference := objectReaderReference{Revision: corpus.Revision, Cases: map[string][]string{}}
	home := t.TempDir()
	run := func(f *managedObjectFixture, args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = f.path
		cmd.Env = parity.Environment(home, nil)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return strings.TrimSuffix(string(out), "\n")
	}
	for method, args := range map[string][]string{
		"ReadsACommitFromLooseObjects": {"rev-parse", "HEAD^{tree}"},
		"ReadsCommitsFromAPackFile":    {"rev-parse", "HEAD^{tree}"},
		"CommitMatchesGitLogOutput":    {"log", "-1", "--format=%an%n%ae%n%at%n%cn%n%ce%n%ct"},
		"ReadsABlobFromLooseObjects":   {"rev-parse", "HEAD:file.txt"},
	} {
		f, _ := managedLooseFixture(t, method)
		reference.Cases[method] = strings.Split(run(f, args...), "\n")
	}
	for _, method := range []string{"CommitMessageMatchesGitCatFileOutput", "ReadsACommitMessageHonoringTheEncodingHeader"} {
		f, _ := managedLooseFixture(t, method)
		raw := run(f, "cat-file", "commit", f.tip.String())
		headers, message, ok := strings.Cut(raw, "\n\n")
		if !ok {
			t.Fatal("Git did not return a raw commit", method)
		}
		if method == "CommitMessageMatchesGitCatFileOutput" {
			reference.Cases[method] = []string{message}
			continue
		}
		if !strings.Contains(headers, "\nencoding ISO-8859-1") {
			t.Fatal("Git did not read the expected commit encoding")
		}
		reference.Cases[method] = []string{"encoding ISO-8859-1", run(f, "log", "-1", "--encoding=UTF-8", "--format=%s")}
	}
	for method, args := range map[string][]string{
		"ReadsTheRootTreeEntriesMatchingGitLsTree": {"HEAD^{tree}"},
		"ReadsANestedTree":                         {"HEAD^{tree}", "HEAD:sub", "HEAD:sub/c.txt"},
		"FindsANodeUsingTheStreamingReader":        {"HEAD^{tree}", "HEAD:sub"},
	} {
		f := managedTreeFixture(t, method)
		results := strings.Split(run(f, append([]string{"rev-parse"}, args...)...), "\n")
		if method == "ReadsTheRootTreeEntriesMatchingGitLsTree" {
			results = append(results, strings.Split(run(f, "ls-tree", results[0]), "\n")...)
		}
		reference.Cases[method] = results
	}
	for key, count := range map[string]int{"PackedDeltaHistoryBlobs": 30, "PackedTwentyCommitTree": 20} {
		f := newManagedObjectFixture(t)
		ids, _ := f.growingHistory(count)
		args := []string{"rev-parse"}
		if count == 20 {
			args = append(args, "HEAD^{tree}", "HEAD:file.txt")
		} else {
			for _, id := range ids {
				args = append(args, id.String()+":file.txt")
			}
		}
		reference.Cases[key] = strings.Split(run(f, args...), "\n")
	}
	for method, name := range map[string]string{
		"ReadsAnAnnotatedTag": "v1.2.3", "ReadsAnAnnotatedTagWithAMultiLineMessage": "v2.0.0",
		"ReadsANestedAnnotatedTag": "outer", "ReadsAnAnnotatedTagFromAPackFile": "v3.0.0",
	} {
		f, _, _ := managedTagFixture(t, method)
		results := strings.Split(run(f, "rev-parse", name, name+"^{}"), "\n")
		if method == "ReadsANestedAnnotatedTag" {
			results = append(results, run(f, "rev-parse", "inner"))
		}
		for _, id := range results {
			if !plumbing.IsHash(id) {
				t.Fatalf("Git did not resolve %s: %q", name, id)
			}
		}
		reference.Cases[method] = results
	}
	if len(reference.Cases) != 15 {
		t.Fatal("Git reference case count changed", len(reference.Cases))
	}
	if output := os.Getenv("GIT_OBJECT_READER_REFERENCE_OUT"); output != "" {
		if err := parity.WriteJSON(output, reference); err != nil {
			t.Fatal(err)
		}
	} else if expected := readObjectReaderReference(t, corpus); !reflect.DeepEqual(reference, expected) {
		t.Fatal("fresh Git object expectations differ from the captured reference")
	}
}

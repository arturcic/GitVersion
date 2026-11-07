package gitrepo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagedIgnoreEntryMatching(t *testing.T) {
	for _, tc := range []struct {
		name, rules, path                    string
		directory, ignoreCase, want, decided bool
	}{
		{"parent-reincluded", "[Oo]bj\nartifacts\n!build/artifacts\n", "build/artifacts", true, false, false, true},
		{"child-still-ignored", "[Oo]bj\nartifacts\n!build/artifacts\n", "build/artifacts/obj", true, false, true, true},
		{"directory-only-file", "cache/\n", "cache", false, false, false, false},
		{"directory-only-directory", "cache/\n", "nested/cache", true, false, true, true},
		{"anchored-no-nested", "/file.txt\n", "nested/file.txt", false, false, false, false},
		{"double-star", "a/**/b\n", "a/one/two/b", false, false, true, true},
		{"double-star-zero", "a/**/b\n", "a/b", false, false, true, true},
		{"mid-segment-star", "out**txt\n", "output.txt", false, false, true, true},
		{"escaped-leading-hash", "\\#name\n", "#name", false, false, true, true},
		{"escaped-trailing-space", "file\\ \n", "file ", false, false, true, true},
		{"negated-class", "[!a].txt\n", "b.txt", false, false, true, true},
		{"unterminated-class", "[x\n", "[x", false, false, true, true},
		{"case-insensitive", "BIN/\n", "bin", true, true, true, true},
		{"escaped-unicode", "\\é.txt\n", "é.txt", false, false, true, true},
		{"case-sensitive", "BIN/\n", "bin", true, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rules, err := parseIgnoreRules(tc.rules, tc.ignoreCase)
			if err != nil {
				t.Fatal(err)
			}
			got, decided, err := (ignoreSource{rules: rules}).decision(tc.path, tc.directory)
			if err != nil || got != tc.want || decided != tc.decided {
				t.Fatalf("want %t/%t got %t/%t (%v)", tc.want, tc.decided, got, decided, err)
			}
		})
	}
}
func TestIgnoredChildUnderReincludedParentIsClean(t *testing.T) {
	f := makeGraph(t, []graphCommit{{name: "root", files: map[string]string{".gitignore": "[Oo]bj\nartifacts\n!build/artifacts\n", "project.txt": "tracked\n"}}}, map[string]string{"main": "root"}, "main")
	p := filepath.Join(f.path, "build/artifacts/obj/output.bin")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("ignored\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(f.path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.path, "project.txt"), []byte("changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(f.path); err == nil {
		t.Fatal("tracked change hidden by ignore rules")
	}
}
func TestIgnoredParentCannotBeReincludedFromBelow(t *testing.T) {
	f := makeGraph(t, []graphCommit{{name: "root", files: map[string]string{".gitignore": "cache/\n!cache/visible.txt\n"}}}, map[string]string{"main": "root"}, "main")
	if err := os.MkdirAll(filepath.Join(f.path, "cache"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.path, "cache/visible.txt"), []byte("ignored\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(f.path); err != nil {
		t.Fatal(err)
	}
}

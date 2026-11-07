package gitrepo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWorktreeConfigMarkerUsesPinnedCommonConfiguration(t *testing.T) {
	for _, tc := range []struct{ name, extension, overlay string }{
		{"enabled-missing", "worktreeconfig = true", ""},
		{"mixed-case-enabled", "WorkTreeConfig = true", "[core]\n bare = true\n"},
		{"disabled", "worktreeConfig = false", "invalid syntax deliberately ignored\n"},
		{"enabled-overlay", "worktreeConfig = true", "[core]\n bare = true\n[extensions]\n unknown = true\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := diamond(t, false)
			configPath := filepath.Join(f.path, ".git", "config")
			original, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			raw := append(append([]byte{}, original...), []byte("\n[extensions]\n "+tc.extension+"\n")...)
			if err := os.WriteFile(configPath, raw, 0644); err != nil {
				t.Fatal(err)
			}
			overlayPath := filepath.Join(f.path, ".git", "config.worktree")
			if tc.overlay != "" {
				if err := os.WriteFile(overlayPath, []byte(tc.overlay), 0644); err != nil {
					t.Fatal(err)
				}
			}
			r, err := Open(f.path)
			if err != nil {
				t.Fatal(err)
			}
			_, head := r.Head()
			if head != f.ids["merge"] {
				t.Fatal("HEAD changed", head)
			}
			if _, err := r.Observe(); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(configPath)
			if err != nil || !bytes.Equal(after, raw) {
				t.Fatal("common config modified", err)
			}
			after, err = os.ReadFile(overlayPath)
			if tc.overlay == "" {
				if !os.IsNotExist(err) {
					t.Fatal("worktree config unexpectedly created", err)
				}
			} else if err != nil || string(after) != tc.overlay {
				t.Fatal("worktree config modified", err)
			}
		})
	}
}

func TestWorktreeConfigAdapterPreservesUnknownExtensionRejection(t *testing.T) {
	f := diamond(t, false)
	p := filepath.Join(f.path, ".git", "config")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, []byte("\n[extensions]\n worktreeconfig = true\n futureUnsupportedExtension = true\n")...)
	if err := os.WriteFile(p, raw, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(f.path); err == nil {
		t.Fatal("unknown extension silently discarded")
	}
}

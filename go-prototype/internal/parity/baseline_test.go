package parity

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFileSetDetectsAddedRemovedAndChangedFiles(t *testing.T) {
	for _, kind := range []string{"added", "removed", "changed"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "gitversion.dll")
			if err := os.WriteFile(path, []byte("reference"), 0644); err != nil {
				t.Fatal(err)
			}
			before, err := FileSet(dir)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "added":
				err = os.WriteFile(filepath.Join(dir, "extra.dll"), []byte("extra"), 0644)
			case "removed":
				err = os.Remove(path)
			case "changed":
				err = os.WriteFile(path, []byte("candidate"), 0644)
			}
			if err != nil {
				t.Fatal(err)
			}
			after, err := FileSet(dir)
			if kind == "removed" {
				if err == nil {
					t.Fatal("accepted empty baseline")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if reflect.DeepEqual(before, after) {
				t.Fatalf("missed %s file", kind)
			}
		})
	}
}

func TestBaselineSpecRejectsUnpinnedIdentity(t *testing.T) {
	s, err := ReadSpec("../../baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"revision", "backend", "parser", "configuration", "schema"} {
		t.Run(kind, func(t *testing.T) {
			var copy Spec
			if err := ReadJSON("../../baseline.json", &copy); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "revision":
				copy.Revision = "main"
			case "backend":
				copy.Environment["GITVERSION_GIT_BACKEND"] = "libgit2"
			case "parser":
				copy.Environment["GITVERSION_ARGUMENT_PARSER_VERSION"] = "v6"
			case "configuration":
				copy.Environment["GITVERSION_CONFIGURATION_VERSION"] = "v6"
			case "schema":
				copy.Schema = 2
			}
			p := filepath.Join(t.TempDir(), "spec.json")
			if err := WriteJSON(p, copy); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadSpec(p); err == nil {
				t.Fatalf("accepted changed %s", kind)
			}
		})
	}
	if s.Revision != "14d222235b7dabb7033473528a76fd18478a9217" {
		t.Fatal("reference revision changed; review baseline migration")
	}
}

func TestRuntimeDirectorySelectsExactVersion(t *testing.T) {
	p, err := RuntimeDirectory("Microsoft.NETCore.App 10.0.13 [/dotnet/shared/Microsoft.NETCore.App]\nMicrosoft.NETCore.App 10.0.14 [/dotnet/shared/Microsoft.NETCore.App]\n", "10.0.14")
	if err != nil || p != filepath.Join("/dotnet/shared/Microsoft.NETCore.App", "10.0.14") {
		t.Fatalf("%s %v", p, err)
	}
	if _, err = RuntimeDirectory("Microsoft.NETCore.App 10.0.13 [/dotnet/shared/Microsoft.NETCore.App]", "10.0.14"); err == nil {
		t.Fatal("silently rolled runtime")
	}
}

func TestVerifyBaselineRejectsTamperingBeforeExecution(t *testing.T) {
	s, err := ReadSpec("../../baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "gitversion.dll"), []byte("reference"), 0644); err != nil {
		t.Fatal(err)
	}
	files, err := FileSet(dir)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := FileHash("../../baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	b := Baseline{Schema: 1, Spec: s, SpecSHA256: hash, Files: files, Platform: Platform()}
	if err = os.WriteFile(filepath.Join(dir, "gitversion.dll"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}
	if err = VerifyBaseline(b, "../../baseline.json", "/should-not-execute", dir); err == nil || err.Error() != "baseline files changed" {
		t.Fatalf("did not reject tampered assembly first: %v", err)
	}
}

func TestVerifyBaselineRejectsSpecAndHostDrift(t *testing.T) {
	for _, kind := range []string{"spec", "host"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			published := filepath.Join(dir, "published")
			if err := os.Mkdir(published, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(published, "gitversion.dll"), []byte("reference"), 0644); err != nil {
				t.Fatal(err)
			}
			specPath := filepath.Join(dir, "baseline.json")
			data, err := os.ReadFile("../../baseline.json")
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(specPath, data, 0644); err != nil {
				t.Fatal(err)
			}
			spec, err := ReadSpec(specPath)
			if err != nil {
				t.Fatal(err)
			}
			files, err := FileSet(published)
			if err != nil {
				t.Fatal(err)
			}
			host := filepath.Join(dir, "dotnet")
			if err = os.WriteFile(host, []byte("never execute this"), 0644); err != nil {
				t.Fatal(err)
			}
			hostHash, err := FileHash(host)
			if err != nil {
				t.Fatal(err)
			}
			b := Baseline{Schema: 1, Spec: spec, SpecSHA256: Hash(data), Files: files, HostSHA256: hostHash, Platform: Platform()}
			want := "dotnet host changed"
			if kind == "spec" {
				want = "baseline spec changed"
				err = os.WriteFile(specPath, append(data, '\n'), 0644)
			} else {
				err = os.WriteFile(host, []byte("modified host"), 0644)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = VerifyBaseline(b, specPath, host, published); err == nil || err.Error() != want {
				t.Fatalf("want %s before executing host, got %v", want, err)
			}
		})
	}
}

func TestRuntimeDirectoryPreservesSpaces(t *testing.T) {
	got, err := RuntimeDirectory("Microsoft.NETCore.App 10.0.14 [/Applications/Shared Tools/dotnet]\r\n", "10.0.14")
	if err != nil || got != "/Applications/Shared Tools/dotnet/10.0.14" {
		t.Fatalf("runtime path %q: %v", got, err)
	}
}

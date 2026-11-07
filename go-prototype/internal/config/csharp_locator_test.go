package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type locatorFixtureFS struct {
	files map[string][]byte
	dirs  map[string]bool
	order []string
}

func (f *locatorFixtureFS) DirectoryExists(p string) bool     { return f.dirs[p] }
func (f *locatorFixtureFS) FileExists(p string) bool          { _, ok := f.files[filepath.Clean(p)]; return ok }
func (f *locatorFixtureFS) ReadFile(p string) ([]byte, error) { return f.files[filepath.Clean(p)], nil }
func (f *locatorFixtureFS) Files(p string) ([]string, error) {
	out := []string{}
	if f.order != nil {
		return append(out, f.order...), nil
	}
	for name := range f.files {
		if filepath.Dir(name) == p {
			out = append(out, name)
		}
	}
	return out, nil
}
func TestCSharpConfigurationFileLocator(t *testing.T) {
	var corpus struct {
		Methods map[string][]struct {
			ID, Operation, Working, Repo string
			Custom                       *string
			Files                        []string
			ExpectedPath                 *string `json:"expected_path"`
			ExpectedMessage              string  `json:"expected_message"`
			ExpectedLog                  string  `json:"expected_log"`
			Reverse, Error               bool
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-file-locator.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Methods) != 14 {
		t.Fatal("source declarations changed")
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, c := range cases {
				t.Run(c.ID, func(t *testing.T) {
					base := t.TempDir()
					repo := filepath.Join(base, "MyGitRepo")
					working := filepath.Join(repo, "Working")
					lower := strings.ToLower(repo)
					resolve := func(s string) string {
						for _, pair := range [][2]string{{"@lowerrepo", lower}, {"@working", working}, {"@repo", repo}, {"@unrelated", filepath.Join(base, "unrelated")}, {"@process", filepath.Join(base, "process")}} {
							s = strings.ReplaceAll(s, pair[0], pair[1])
						}
						return s
					}
					fs := &locatorFixtureFS{files: map[string][]byte{}, dirs: map[string]bool{repo: true, working: true, lower: true}}
					for _, file := range c.Files {
						path := resolve(file)
						fs.files[path] = []byte{}
						fs.dirs[filepath.Dir(path)] = true
					}
					if c.Operation == "order" {
						fs.order = []string{filepath.Join(repo, "GitVersion.yml"), filepath.Join(repo, "gitversion.yml"), filepath.Join(repo, "GitVersion.yaml")}
						if c.Reverse {
							fs.order[0], fs.order[2] = fs.order[2], fs.order[0]
						}
					}
					var custom *string
					if c.Custom != nil {
						x := resolve(*c.Custom)
						custom = &x
					}
					log := ""
					locator := FileLocator{FS: fs, Custom: custom, Log: func(s string) { log = s }}
					work := resolve(c.Working)
					var got *string
					var err error
					switch c.Operation {
					case "order":
						got, err = locator.Get(&repo)
					case "get":
						got, err = locator.Get(&work)
					case "provide":
						_, err = locator.ProvideForDirectory(&work, "v7")
					case "verify":
						err = locator.Verify(work, repo)
					default:
						t.Fatal(c.Operation)
					}
					if (err != nil) != c.Error {
						t.Fatal(err)
					}
					if c.ExpectedMessage != "" {
						workingFile, repoFile := "", ""
						if len(c.Files) == 2 {
							repoFile = resolve(c.Files[0])
							workingFile = resolve(c.Files[1])
						} else {
							workingFile = configPath(work, pathValue(custom))
							repoFile = configPath(repo, pathValue(custom))
						}
						expected := c.ExpectedMessage
						for _, p := range [][2]string{{"{workingDirectoryConfigFilePath.Value}", workingFile}, {"{workDirConfigFilePath.Value}", workingFile}, {"{repositoryConfigFilePath.Value}", repoFile}, {"{workingPathFileConfig}", workingFile}, {"{repoPathFileConfig}", repoFile}} {
							expected = strings.ReplaceAll(expected, p[0], p[1])
						}
						if err == nil || err.Error() != expected {
							t.Fatalf("diagnostic: got %v want %s", err, expected)
						}
					}
					if c.Operation == "get" || c.Operation == "order" {
						if c.ExpectedPath == nil {
							if got != nil {
								t.Fatal("expected missing path", *got)
							}
						} else if got == nil || *got != resolve(*c.ExpectedPath) {
							t.Fatal("path mismatch", got, resolve(*c.ExpectedPath))
						}
					}
					if c.ExpectedLog != "" && !regexp.MustCompile(c.ExpectedLog).MatchString(log) {
						t.Fatal("log", log, c.ExpectedLog)
					}
				})
			}
		})
	}
}

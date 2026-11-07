package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/output"
	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

type msbuildOutputCase struct {
	ID, Content, Approved, Namespace string
	FileName                         string `json:"file_name"`
	ExpectedError                    string `json:"expected_error"`
	Arguments                        []*string
	Variables                        map[string]string
	BuildServer                      bool `json:"build_server"`
}

func msbuildOutputCases(t *testing.T) map[string]struct {
	Methods map[string]struct{ Cases []msbuildOutputCase }
} {
	t.Helper()
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct{ Cases []msbuildOutputCase }
		}
	}
	if err := parity.ReadJSON("../../testdata/csharp-msbuild-output.json", &corpus); err != nil {
		t.Fatal(err)
	}
	return corpus.Suites
}

func TestCSharpMSBuildOutput(t *testing.T) {
	corpus := msbuildOutputCases(t)
	for _, suite := range []string{"InvalidFileCheckerTests", "AssemblyInfoFileHelperTests", "GitVersionInfoGeneratorTests", "GenerateGitVersionInformationTest"} {
		for method, entry := range corpus[suite].Methods {
			if strings.Contains(method, "WhenRunWithMsBuild") {
				continue
			}
			t.Run(method, func(t *testing.T) {
				for _, tc := range entry.Cases {
					t.Run(tc.ID, func(t *testing.T) {
						switch suite {
						case "InvalidFileCheckerTests":
							dir := t.TempDir()
							project := filepath.Join(dir, "Fake.csproj")
							writeMSBuildFixture(t, project, "")
							writeMSBuildFixture(t, filepath.Join(dir, tc.FileName), tc.Content)
							var out, diagnostics bytes.Buffer
							err := execute([]string{"msbuild", "check-assembly-info", "--project-file", project, tc.FileName}, &out, &diagnostics)
							if tc.ExpectedError == "" && err != nil || tc.ExpectedError != "" && (err == nil || err.Error() != tc.ExpectedError) {
								t.Fatalf("checker error = %v, want %q", err, tc.ExpectedError)
							}
						case "AssemblyInfoFileHelperTests":
							checkMSBuildFileInfo(t, method, tc)
						case "GitVersionInfoGeneratorTests":
							checkMSBuildApproval(t, method, tc)
						case "GenerateGitVersionInformationTest":
							checkMSBuildCommand(t, method, tc)
						}
					})
				}
			})
		}
	}
}

func checkMSBuildFileInfo(t *testing.T, method string, tc msbuildOutputCase) {
	t.Helper()
	if method == "GetFileExtensionShouldReturnCorrectExtension" {
		language := *tc.Arguments[0]
		extension, err := languageExtension(language)
		if tc.Arguments[1] == nil {
			want := "Unknown language detected: '" + language + "'"
			if err == nil || err.Error() != want {
				t.Fatalf("language error = %v, want %q", err, want)
			}
		} else if err != nil || extension != *tc.Arguments[1] {
			t.Fatalf("extension = %q, want %q, error %v", extension, *tc.Arguments[1], err)
		}
		return
	}
	intermediate := "MyIntermediateOutputPath"
	if method == "GetFileWriteInfoShouldCreateRandomNamedFileWhenNoIntermediateOutputPath" {
		intermediate = ""
	}
	path, extension, err := generatedVersionInfoPath("MyProject.csproj", intermediate, "C#", "GeneratedVersionInformation")
	if err != nil || extension != "cs" {
		t.Fatalf("generated path = %q, extension %q, error %v", path, extension, err)
	}
	if intermediate == "" {
		if filepath.Dir(path) != filepath.Join(os.TempDir(), "GitVersionTask") ||
			!strings.HasPrefix(filepath.Base(path), "GeneratedVersionInformation_MyProject_") || !strings.HasSuffix(path, ".g.cs") {
			t.Fatalf("temporary name %q", path)
		}
	} else if path != filepath.Join(intermediate, "GeneratedVersionInformation.g.cs") {
		t.Fatalf("intermediate output path %q", path)
	}
}

func checkMSBuildApproval(t *testing.T, method string, tc msbuildOutputCase) {
	t.Helper()
	v, err := semver.Parse("1.2.3-unstable4")
	if err != nil {
		t.Fatal(err)
	}
	source := semver.Version{Major: 1, Minor: 2, Patch: 2}
	count := int64(5)
	date := time.Date(2014, 3, 6, 23, 59, 59, 0, time.UTC)
	values, err := output.FromSemanticVersion(output.VariableInput{
		Version: v, LegacySource: &source, LegacyIncrement: semver.Patch, CountSHA: "versionSourceSha",
		Branch: "feature1", SHA: "commitSha", ShortSHA: "commitShortSha", Date: &date, Distance: count, Build: &count,
	}, config.OutputSettings{}, os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	namespace := ""
	if method == "ShouldProperlyOutputNamespaceDeclaration" {
		namespace = "My.Custom.Namespace"
	}
	ext := *tc.Arguments[0]
	path := filepath.Join(t.TempDir(), "GitVersionInformation.g."+ext)
	if err := generateVersionInfo(path, ext, namespace, values); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != tc.Approved {
		t.Fatalf("generated %s differs from pinned approval: %v\n%s", ext, err, got)
	}
}

func checkMSBuildCommand(t *testing.T, method string, tc msbuildOutputCase) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
	azure := tc.BuildServer
	if azure {
		t.Setenv("TF_BUILD", "true")
	}
	working := msbuildVersionRepository(t, dir, azure)
	project := filepath.Join(working, "App.Project."+msbuildProjectExtension(t, *tc.Arguments[0]))
	writeMSBuildFixture(t, project, "<Project/>")
	// The project represents a build input. Keep it outside Git's dirty count.
	writeMSBuildFixture(t, filepath.Join(working, ".git", "info", "exclude"), filepath.Base(project)+"\n")
	args := []string{"msbuild", "generate-version-info", "--project-file", project, "--language", *tc.Arguments[0]}
	if strings.Contains(method, "IntermediateOutputPath") {
		args = append(args, "--intermediate-output-path", filepath.Join(dir, "obj"))
	}
	if tc.Namespace != "" {
		args = append(args, "--use-project-namespace")
		if strings.Contains(method, "RootNamespaceIsSet") {
			args = append(args, "--root-namespace", tc.Namespace)
		}
	}
	var stdout, stderr bytes.Buffer
	if err := execute(args, &stdout, &stderr); err != nil || stderr.Len() != 0 {
		t.Fatalf("generate version info: %v, stderr %q", err, stderr.String())
	}
	path := strings.TrimSpace(stdout.String())
	ext, err := languageExtension(*tc.Arguments[0])
	if err != nil || !strings.HasPrefix(filepath.Base(path), "GitVersionInformation") || !strings.HasSuffix(path, ".g."+ext) {
		t.Fatalf("generated path %q", path)
	}
	assertMSBuildVersionInfo(t, path, tc)
}

func assertMSBuildVersionInfo(t *testing.T, path string, tc msbuildOutputCase) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range tc.Variables {
		pattern := regexp.MustCompile(regexp.QuoteMeta(name) + `[^\r\n]*=[^\r\n]*"` + regexp.QuoteMeta(value) + `"`)
		if !pattern.Match(content) {
			t.Errorf("expected %s = %q in %s", name, value, path)
		}
	}
	if tc.Namespace != "" && !strings.Contains(strings.ToLower(string(content)), strings.ToLower("namespace "+tc.Namespace)) {
		t.Fatalf("namespace %s missing from %s", tc.Namespace, path)
	}
}

func msbuildProjectExtension(t *testing.T, language string) string {
	t.Helper()
	extension, err := languageExtension(language)
	if err != nil {
		t.Fatal(err)
	}
	return extension + "proj"
}

func writeMSBuildFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func msbuildVersionRepository(t *testing.T, dir string, azure bool) string {
	t.Helper()
	steps := []fixtures.Step{{Op: "commit", ID: "first"}, {Op: "tag", Name: "1.2.3"}, {Op: "commit", ID: "head"}}
	if azure {
		steps = nil
		for i := range 6 {
			steps = append(steps, fixtures.Step{Op: "commit", ID: "seed-" + string(rune('a'+i))})
		}
		steps = append(steps, fixtures.Step{Op: "commit", ID: "tagged"}, fixtures.Step{Op: "tag", Name: "1.0.0"},
			fixtures.Step{Op: "commit", ID: "head"}, fixtures.Step{Op: "branch", Name: "develop"})
	}
	steps = append(steps, fixtures.Step{Op: "checkpoint", ID: "head"})
	cps, err := fixtures.Generate([]fixtures.Recipe{{ID: "msbuild-output", Steps: steps}}, filepath.Join(dir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if !azure {
		return cps[0].Path
	}
	local := localCICopy(t, dir, cps[0].Path)
	repo, err := git.PlainOpen(local)
	if err != nil {
		t.Fatal(err)
	}
	tip := plumbing.NewHash(cps[0].Head)
	for _, branch := range []string{"main", "develop"} {
		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewRemoteReferenceName("origin", branch), tip)); err != nil {
			t.Fatal(err)
		}
	}
	tree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Checkout(&git.CheckoutOptions{Hash: tip, Force: true}); err != nil {
		t.Fatal(err)
	}
	for _, branch := range []string{"main", "develop"} {
		if err := repo.Storer.RemoveReference(plumbing.NewBranchReferenceName(branch)); err != nil {
			t.Fatal(err)
		}
	}
	return local
}

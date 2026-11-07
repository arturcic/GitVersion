//go:build platform

package main

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/parity"
)

func TestPlatformCSharpMSBuildOutput(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	sdk, candidate, dotnet := os.Getenv("PARITY_MSBUILD"), os.Getenv("PARITY_CANDIDATE"), os.Getenv("PARITY_DOTNET")
	hash, err := parity.FileHash(candidate)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(os.Getenv("PARITY_EVIDENCE_DIR"), "msbuild-output-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)
	comparisons := 0
	for method, entry := range msbuildOutputCases(t)["GenerateGitVersionInformationTest"].Methods {
		if !strings.Contains(method, "WhenRunWithMsBuild") {
			continue
		}
		t.Run(method, func(t *testing.T) {
			for _, tc := range entry.Cases {
				comparisons++
				t.Run(tc.ID, func(t *testing.T) {
					caseDir := t.TempDir()
					work := msbuildVersionRepository(t, caseDir, tc.BuildServer)
					language := *tc.Arguments[0]
					project := filepath.Join(work, "App."+msbuildProjectExtension(t, language))
					ext, err := languageExtension(language)
					if err != nil {
						t.Fatal(err)
					}
					intermediate := work
					if strings.Contains(method, "IntermediateOutputPath") || tc.Namespace != "" {
						intermediate = filepath.Join(work, "obj")
					}
					namespace := ""
					if strings.Contains(method, "RootNamespaceIsSet") {
						namespace = tc.Namespace
					}
					target := filepath.Join(root, "msbuild", "GitVersion.Go.targets")
					projectXML := fmt.Sprintf(`<Project>
  <PropertyGroup>
    <GitVersionGoLanguage>%s</GitVersionGoLanguage>
    <IntermediateOutputPath>%s</IntermediateOutputPath>
    <RootNamespace>%s</RootNamespace>
    <UseProjectNamespaceForGitVersionInformation>%t</UseProjectNamespaceForGitVersionInformation>
  </PropertyGroup>
  <Import Project="%s" />
</Project>`, html.EscapeString(language), html.EscapeString(intermediate),
						html.EscapeString(namespace), tc.Namespace != "", html.EscapeString(target))
					writeMSBuildFixture(t, project, projectXML)
					writeMSBuildFixture(t, filepath.Join(work, ".git", "info", "exclude"), filepath.Base(project)+"\n")
					env := map[string]string{"GITVERSION_ARGUMENT_PARSER_VERSION": "v7", "GITVERSION_CONFIGURATION_VERSION": "v7",
						"DOTNET_ReadyToRun": "0", "DOTNET_ROOT": filepath.Dir(dotnet), "MSBUILDNOINPROCNODE": "0"}
					if tc.BuildServer {
						env["TF_BUILD"] = "true"
					}
					ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
					defer cancel()
					cmd := exec.CommandContext(ctx, dotnet, sdk, "-nologo", "-v:minimal", "-nodeReuse:false", "-t:GenerateGitVersionGoInformation", "-p:GitVersionGoExecutable="+candidate, project)
					cmd.Dir, cmd.Env = work, parity.Environment(caseDir, env)
					var stdout, stderr bytes.Buffer
					cmd.Stdout, cmd.Stderr = &stdout, &stderr
					err = cmd.Run()
					expected := filepath.Join(intermediate, "GitVersionInformation.g."+ext)
					// macOS may expose the temporary directory through either /var or /private/var.
					reported := strings.ReplaceAll(stdout.String(), "/private/var/", "/var/")
					canonicalExpected := strings.ReplaceAll(expected, "/private/var/", "/var/")
					if err != nil || stderr.Len() != 0 || !strings.Contains(reported, "GitVersionInformationFilePath: "+canonicalExpected) {
						t.Fatalf("MSBuild failed to report %s: %v\nstdout %s\nstderr %s", expected, err, &stdout, &stderr)
					}
					assertMSBuildVersionInfo(t, expected, tc)
					path := filepath.Join(dir, method, tc.ID)
					for name, data := range map[string]string{"stdout": stdout.String(), "stderr": stderr.String(), "project.xml": projectXML} {
						writeMSBuildFixture(t, filepath.Join(path, name), data)
					}
					generated, err := os.ReadFile(expected)
					if err != nil {
						t.Fatal(err)
					}
					writeMSBuildFixture(t, filepath.Join(path, filepath.Base(expected)), string(generated))
				})
			}
		})
	}
	if !t.Failed() {
		sdkHash, err := parity.FileHash(sdk)
		if err != nil {
			t.Fatal(err)
		}
		if err := parity.WriteJSON(filepath.Join(dir, "report.json"), map[string]any{
			"comparisons": comparisons, "candidate_sha256": hash, "msbuild_sha256": sdkHash,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

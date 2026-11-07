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

func TestPlatformCSharpMSBuildTasks(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	sdk, candidate, dotnet := os.Getenv("PARITY_MSBUILD"), os.Getenv("PARITY_CANDIDATE"), os.Getenv("PARITY_DOTNET")
	hash, err := parity.FileHash(candidate)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(os.Getenv("PARITY_EVIDENCE_DIR"), "msbuild-tasks-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("evidence:", dir)
	t.Setenv("GITVERSION_ARGUMENT_PARSER_VERSION", "v7")
	t.Setenv("GITVERSION_CONFIGURATION_VERSION", "v7")
	comparisons := 0
	for suite, group := range readMSBuildTasks(t).Suites {
		for method, item := range group.Methods {
			if !strings.Contains(method, "WhenRunWithMsBuild") {
				continue
			}
			t.Run(method, func(t *testing.T) {
				for _, tc := range item.Cases {
					comparisons++
					t.Run(tc.ID, func(t *testing.T) {
						caseDir := t.TempDir()
						if tc.Azure {
							t.Setenv("TF_BUILD", "true")
						}
						work := msbuildVersionRepository(t, caseDir, tc.Azure, tc.Config)
						language := "C#"
						if suite == "UpdateAssemblyInfoTaskTest" {
							language = tc.Arguments[0]
						}
						project := filepath.Join(work, "App."+msbuildProjectExtension(t, language))
						intermediate := work
						if strings.Contains(method, "IntermediateOutputPath") {
							intermediate = filepath.Join(work, "obj")
						}
						targets := filepath.Join(root, "msbuild", "GitVersion.Go.targets")
						target := map[string]string{"GetVersionTaskTests": "GetGitVersionGoVersion", "WriteVersionInfoTest": "WriteGitVersionGoBuildLog", "UpdateAssemblyInfoTaskTest": "UpdateGitVersionGoAssemblyInfo"}[suite]
						message := ""
						if suite == "GetVersionTaskTests" {
							name := "GitVersion_" + tc.Arguments[0]
							message = fmt.Sprintf(`<Message Text="%s: $(%s)" Importance="High" />`, name, name)
						}
						versionFile := filepath.Join(caseDir, "version.json")
						projectXML := fmt.Sprintf(`<Project>
  <PropertyGroup>
    <GitVersionGoLanguage>%s</GitVersionGoLanguage>
    <IntermediateOutputPath>%s</IntermediateOutputPath>
    <GitVersionGoVersionFile>%s</GitVersionGoVersionFile>
  </PropertyGroup>
  <Import Project="%s" />
  <Target Name="CheckTaskOutput" DependsOnTargets="%s">%s</Target>
</Project>`, html.EscapeString(language), html.EscapeString(intermediate), html.EscapeString(versionFile),
							html.EscapeString(targets), target, message)
						writeMSBuildFixture(t, project, projectXML)
						writeMSBuildFixture(t, filepath.Join(work, ".git", "info", "exclude"), filepath.Base(project)+"\n")
						writeMSBuildTaskVersionFile(t, caseDir, work)
						env := map[string]string{"GITVERSION_ARGUMENT_PARSER_VERSION": "v7", "GITVERSION_CONFIGURATION_VERSION": "v7",
							"DOTNET_ReadyToRun": "0", "DOTNET_ROOT": filepath.Dir(dotnet)}
						if tc.Azure {
							env["TF_BUILD"] = "true"
						}
						ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
						defer cancel()
						command := exec.CommandContext(ctx, dotnet, sdk, "-nologo", "-v:minimal", "-nodeReuse:false", "-t:CheckTaskOutput", "-p:GitVersionGoExecutable="+candidate, project)
						command.Dir, command.Env = work, parity.Environment(caseDir, env)
						var stdout, stderr bytes.Buffer
						command.Stdout, command.Stderr = &stdout, &stderr
						err := command.Run()
						if err != nil || stderr.Len() != 0 || stdout.Len() == 0 {
							t.Fatalf("MSBuild %s: %v\nstdout %s\nstderr %s", target, err, &stdout, &stderr)
						}
						evidence := filepath.Join(dir, method, tc.ID)
						for name, data := range map[string]string{"stdout": stdout.String(), "stderr": stderr.String(), "project.xml": projectXML} {
							writeMSBuildFixture(t, filepath.Join(evidence, name), data)
						}
						switch suite {
						case "GetVersionTaskTests":
							want := "GitVersion_" + tc.Arguments[0] + ": " + tc.Arguments[1]
							if !strings.Contains(stdout.String(), want) {
								t.Fatalf("expected %q in %s", want, &stdout)
							}
						case "WriteVersionInfoTest":
							assertMSBuildTaskLog(t, stdout.String(), tc)
						case "UpdateAssemblyInfoTaskTest":
							ext, _ := languageExtension(language)
							file := filepath.Join(intermediate, "AssemblyInfo.g."+ext)
							reported := strings.ReplaceAll(stdout.String(), "/private/var/", "/var/")
							want := "AssemblyInfoTempFilePath: " + strings.ReplaceAll(file, "/private/var/", "/var/")
							if !strings.Contains(reported, want) {
								t.Fatalf("expected %q in %s", want, &stdout)
							}
							assertAssemblyInfoTaskFile(t, file, tc.Attribute)
							content, err := os.ReadFile(file)
							if err != nil {
								t.Fatal(err)
							}
							writeMSBuildFixture(t, filepath.Join(evidence, filepath.Base(file)), string(content))
						}
					})
				}
			})
		}
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

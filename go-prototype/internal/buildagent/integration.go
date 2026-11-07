package buildagent

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/output"
)

func (a *Agent) WriteIntegration(ctx context.Context, writer io.Writer, variables map[string]any, updateBuildNumber bool) error {
	if (a.Name == "GitLabCi" || a.Name == "Jenkins" || a.Name == "CodeBuild") && a.PropertyFile == nil {
		return nil
	}
	if a.Name == "BitBucketPipelines" && (a.PropertyFile == nil || a.PowerShellFile == nil) {
		return nil
	}
	write := func(message string) error {
		_, err := fmt.Fprintln(writer, message)
		return err
	}
	if updateBuildNumber {
		if err := write("Set Build Number for '" + a.Name + "'."); err != nil {
			return err
		}
		number, err := a.BuildNumber(ctx, variables)
		if err != nil {
			return err
		}
		if err := write(number); err != nil {
			return err
		}
	}
	if err := write("Set Output Variables for '" + a.Name + "'."); err != nil {
		return err
	}
	lines := []string{}
	for _, key := range output.VariableNames {
		messages, err := a.OutputVariables(ctx, key, ValueText(variables[key]))
		if err != nil {
			return err
		}
		for _, message := range messages {
			if err := write(message); err != nil {
				return err
			}
			lines = append(lines, message)
		}
	}
	switch a.Name {
	case "GitHubActions":
		file, exists := "", false
		if a.Environment != nil {
			file, exists = a.Environment("GITHUB_ENV")
		}
		if !exists {
			return write("Unable to write GitVersion variables to $GITHUB_ENV because the environment variable is not set.")
		}
		if err := write("Writing version variables to $GITHUB_ENV file for 'GitHubActions'."); err != nil {
			return err
		}
		var content strings.Builder
		for _, key := range output.VariableNames {
			if value := ValueText(variables[key]); value != "" {
				fmt.Fprintf(&content, "GitVersion_%s=%s\n", key, value)
			}
		}
		stream, err := os.OpenFile(a.path(file), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr := stream.WriteString(content.String())
		closeErr := stream.Close()
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	case "GitLabCi", "Jenkins", "CodeBuild":
		if err := write("Outputting variables to '" + *a.PropertyFile + "' ... "); err != nil {
			return err
		}
		return writeLines(a.path(*a.PropertyFile), lines)
	case "BitBucketPipelines":
		messages := []string{
			"Outputting variables to '" + *a.PropertyFile + "' for Bash,",
			"and to '" + *a.PowerShellFile + "' for Powershell ... ",
			"To import the file into your build environment, add the following line to your build step:",
			"Bash:", "  - source " + *a.PropertyFile, "Powershell:", "  - . .\\" + *a.PowerShellFile, "",
			"To reuse the file across build steps, add the file as a build artifact:",
			"Bash:", "  artifacts:", "    - " + *a.PropertyFile, "Powershell:", "  artifacts:", "    - " + *a.PowerShellFile,
		}
		for _, message := range messages {
			if err := write(message); err != nil {
				return err
			}
		}
		exports, ps := []string{}, []string{}
		for _, key := range output.VariableNames {
			value := ValueText(variables[key])
			name := "GITVERSION_" + strings.ToUpper(key)
			exports = append(exports, "export "+name+"="+value)
			ps = append(ps, "$"+name+" = \""+value+"\"")
		}
		if err := writeLines(a.path(*a.PropertyFile), exports); err != nil {
			return err
		}
		return writeLines(a.path(*a.PowerShellFile), ps)
	}
	return nil
}

func writeLines(path string, lines []string) error {
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

func WriteDotEnv(writer io.Writer, variables map[string]any) error {
	names := slices.Clone(output.VariableNames)
	sort.Strings(names)
	for _, key := range names {
		if _, err := fmt.Fprintf(writer, "GitVersion_%s='%s'\n", key, ValueText(variables[key])); err != nil {
			return err
		}
	}
	return nil
}

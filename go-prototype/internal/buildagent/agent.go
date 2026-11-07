package buildagent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/formatting"
	"github.com/GitTools/GitVersion/go-prototype/internal/output"
	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
)

type Agent struct {
	Name                         string
	Environment                  func(string) (string, bool)
	PropertyFile, PowerShellFile *string
	Directory                    string
	HTTPClient                   *http.Client
}

var registry = []struct{ name, variable string }{
	{"AppVeyor", "APPVEYOR"}, {"AzurePipelines", "TF_BUILD"}, {"BitBucketPipelines", "BITBUCKET_WORKSPACE"},
	{"BuildKite", "BUILDKITE"}, {"CodeBuild", "CODEBUILD_WEBHOOK_HEAD_REF"}, {"ContinuaCi", "ContinuaCI.Version"},
	{"Drone", "DRONE"}, {"EnvRun", "ENVRUN_DATABASE"}, {"GitHubActions", "GITHUB_ACTIONS"},
	{"GitLabCi", "GITLAB_CI"}, {"Jenkins", "JENKINS_URL"}, {"MyGet", "BuildRunner"},
	{"SpaceAutomation", "JB_SPACE_PROJECT_KEY"}, {"TeamCity", "TEAMCITY_VERSION"}, {"TravisCi", "TRAVIS"},
}

func New(name, directory string, environment func(string) (string, bool)) *Agent {
	agent := &Agent{Name: name, Directory: directory, Environment: environment}
	switch name {
	case "GitLabCi", "Jenkins", "CodeBuild", "BitBucketPipelines":
		file := "gitversion.properties"
		agent.PropertyFile = &file
	}
	if name == "BitBucketPipelines" {
		file := "gitversion.ps1"
		agent.PowerShellFile = &file
	}
	return agent
}

func Resolve(directory string, environment func(string) (string, bool)) *Agent {
	result := New("LocalBuild", directory, environment)
	for _, registered := range registry {
		agent := New(registered.name, directory, environment)
		if agent.CanApply() {
			result = agent
		}
	}
	return result
}

func (a *Agent) env(key string) string {
	if a.Environment == nil {
		return ""
	}
	value, _ := a.Environment(key)
	return value
}

func (a *Agent) CanApply() bool {
	switch a.Name {
	case "LocalBuild":
		return true
	case "BuildKite", "Drone":
		return strings.EqualFold(a.env(map[string]string{"BuildKite": "BUILDKITE", "Drone": "DRONE"}[a.Name]), "true")
	case "TravisCi":
		return a.env("TRAVIS") == "true" && a.env("CI") == "true"
	case "MyGet":
		return strings.EqualFold(a.env("BuildRunner"), "MyGet")
	case "CodeBuild":
		return a.env("CODEBUILD_WEBHOOK_HEAD_REF") != "" || a.env("CODEBUILD_SOURCE_VERSION") != ""
	case "EnvRun":
		path := a.env("ENVRUN_DATABASE")
		if path == "" {
			return false
		}
		info, err := os.Stat(path)
		return err == nil && !info.IsDir()
	}
	for _, registered := range registry {
		if registered.name == a.Name {
			return a.env(registered.variable) != ""
		}
	}
	return false
}

func (a *Agent) environmentValue(key string) *string {
	if a.Environment == nil {
		return nil
	}
	value, exists := a.Environment(key)
	if !exists {
		return nil
	}
	return &value
}

func (a *Agent) CurrentBranch() *string {
	switch a.Name {
	case "AzurePipelines":
		if branch := a.env("GIT_BRANCH"); strings.TrimSpace(branch) != "" {
			return &branch
		}
		value := a.environmentValue("BUILD_SOURCEBRANCH")
		if value != nil && strings.HasPrefix(strings.ToLower(*value), "refs/tags") {
			return nil
		}
		return value
	case "GitHubActions":
		if strings.EqualFold(a.env("GITHUB_REF_TYPE"), "tag") {
			return nil
		}
		return a.environmentValue("GITHUB_REF")
	case "GitLabCi":
		if a.env("CI_COMMIT_TAG") != "" {
			return nil
		}
		if value := a.env("CI_MERGE_REQUEST_REF_PATH"); value != "" {
			return &value
		}
		return a.environmentValue("CI_COMMIT_REF_NAME")
	case "BitBucketPipelines":
		value := a.env("BITBUCKET_BRANCH")
		if strings.HasPrefix(value, "refs/heads/") {
			return &value
		}
	case "BuildKite":
		if pr := a.env("BUILDKITE_PULL_REQUEST"); pr != "" && pr != "false" {
			value := "refs/pull/" + pr + "/head"
			return &value
		}
		return a.environmentValue("BUILDKITE_BRANCH")
	case "CodeBuild":
		if value := a.env("CODEBUILD_WEBHOOK_HEAD_REF"); value != "" {
			return &value
		}
		return a.environmentValue("CODEBUILD_SOURCE_VERSION")
	case "Drone":
		if strings.TrimSpace(a.env("DRONE_PULL_REQUEST")) != "" {
			if source := a.env("DRONE_SOURCE_BRANCH"); strings.TrimSpace(source) != "" {
				return &source
			}
			if source, _, found := strings.Cut(a.env("CI_COMMIT_REFSPEC"), ":"); found && source != "" {
				return &source
			}
		}
		return a.environmentValue("DRONE_BRANCH")
	case "Jenkins":
		if branch := a.env("BRANCH_NAME"); branch != "" {
			return &branch
		}
		if branch := a.environmentValue("GIT_LOCAL_BRANCH"); branch != nil {
			return branch
		}
		return a.environmentValue("GIT_BRANCH")
	case "SpaceAutomation":
		return a.environmentValue("JB_SPACE_GIT_BRANCH")
	case "TeamCity":
		if value := a.env("Git_Branch"); strings.TrimSpace(value) != "" {
			return &value
		}
	case "AppVeyor":
		if value := a.env("APPVEYOR_PULL_REQUEST_HEAD_REPO_BRANCH"); strings.TrimSpace(value) != "" {
			return &value
		}
		return a.environmentValue("APPVEYOR_REPO_BRANCH")
	}
	return nil
}

func (a *Agent) CurrentTag() *string {
	var tag string
	switch a.Name {
	case "GitHubActions":
		if !strings.EqualFold(a.env("GITHUB_REF_TYPE"), "tag") || !strings.HasPrefix(a.env("GITHUB_REF"), "refs/tags/") {
			return nil
		}
		tag = a.env("GITHUB_REF")
	case "GitLabCi":
		if a.env("CI_COMMIT_TAG") == "" {
			return nil
		}
		tag = "refs/tags/" + a.env("CI_COMMIT_TAG")
	default:
		return nil
	}
	return &tag
}

func (a *Agent) PreventFetch() bool {
	switch a.Name {
	case "AppVeyor", "ContinuaCi", "Drone", "MyGet":
		return false
	case "TeamCity":
		return strings.TrimSpace(a.env("GIT_BRANCH")) != "" || strings.TrimSpace(a.env("Git_Branch")) != ""
	default:
		return true
	}
}

func (a *Agent) ShouldCleanUpRemotes() bool { return a.Name == "Jenkins" && a.env("BRANCH_NAME") != "" }

func ValueText(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func serviceEscape(value string) string {
	return strings.NewReplacer("|", "||", "'", "|'", "[", "|[", "]", "|]", "\r", "|r", "\n", "|n").Replace(value)
}

func (a *Agent) BuildNumber(ctx context.Context, variables map[string]any) (string, error) {
	version := ValueText(variables["FullSemVer"])
	switch a.Name {
	case "LocalBuild", "GitHubActions", "BuildKite", "SpaceAutomation":
		return "", nil
	case "TeamCity":
		return "##teamcity[buildNumber '" + serviceEscape(version) + "']", nil
	case "MyGet":
		return "##myget[buildNumber '" + serviceEscape(version) + "']", nil
	case "ContinuaCi":
		return "@@continua[setBuildVersion value='" + version + "']", nil
	case "AzurePipelines":
		build := a.env("BUILD_BUILDNUMBER")
		if strings.TrimSpace(build) == "" {
			return version, nil
		}
		names := slices.Clone(output.VariableNames)
		sort.Strings(names)
		replaced := build
		for _, key := range names {
			value, exists := variables[key]
			if !exists || value == nil {
				continue
			}
			expression, err := patterns.Compile(`\$\(GITVERSION[_\.]`+key+`\)`, patterns.IgnoreCase)
			if err != nil {
				return "", err
			}
			replaced, err = expression.Replace(replaced, ValueText(value), -1, -1)
			if err != nil {
				return "", err
			}
		}
		if replaced == build {
			replaced = strings.TrimSuffix(version, "+0")
		}
		return "##vso[build.updatebuildnumber]" + replaced, nil
	case "AppVeyor":
		if a.env("APPVEYOR_API_URL") == "" {
			return "", fmt.Errorf("APPVEYOR_API_URL environment variable not set")
		}
		err := a.request(ctx, http.MethodPut, "api/build", BuildNumberRequest(version+".build."+a.env("APPVEYOR_BUILD_NUMBER")))
		if err != nil {
			return fmt.Sprintf("Failed to set AppVeyor build number to '%s'. The error was: %s", version, err), nil
		}
		return "Set AppVeyor build number to '" + version + "'.", nil
	default:
		return version, nil
	}
}

func (a *Agent) OutputVariables(ctx context.Context, name, value string) ([]string, error) {
	switch a.Name {
	case "LocalBuild", "GitHubActions", "BuildKite", "SpaceAutomation", "BuildAgent":
		return []string{}, nil
	case "AzurePipelines":
		return []string{"##vso[task.setvariable variable=GitVersion_" + name + "]" + value, "##vso[task.setvariable variable=GitVersion_" + name + ";isOutput=true]" + value}, nil
	case "TeamCity":
		return []string{"##teamcity[setParameter name='GitVersion." + name + "' value='" + serviceEscape(value) + "']", "##teamcity[setParameter name='system.GitVersion." + name + "' value='" + serviceEscape(value) + "']"}, nil
	case "MyGet":
		return []string{"##myget[setParameter name='GitVersion." + name + "' value='" + serviceEscape(value) + "']"}, nil
	case "ContinuaCi":
		return []string{"@@continua[setVariable name='GitVersion_" + name + "' value='" + value + "' skipIfNotDefined='true']"}, nil
	case "EnvRun":
		return []string{"@@envrun[set name='GitVersion_" + name + "' value='" + value + "']"}, nil
	case "BitBucketPipelines":
		return []string{"GITVERSION_" + strings.ToUpper(name) + "=" + value}, nil
	case "AppVeyor":
		if err := a.request(ctx, http.MethodPost, "api/build/variables", VariableRequest("GitVersion_"+name, value)); err != nil {
			return nil, err
		}
		return []string{"Adding Environment Variable. name='GitVersion_" + name + "' value='" + value + "']"}, nil
	default:
		return []string{"GitVersion_" + name + "=" + value}, nil
	}
}

func BuildNumberRequest(version string) []byte {
	return []byte(`{"version":` + string(formatting.JSONString(version)) + `}`)
}
func VariableRequest(name, value string) []byte {
	return []byte(`{"name":` + string(formatting.JSONString(name)) + `,"value":` + string(formatting.JSONString(value)) + `}`)
}

func (a *Agent) request(ctx context.Context, method, path string, body []byte) error {
	base := a.env("APPVEYOR_API_URL")
	if base == "" {
		return fmt.Errorf("APPVEYOR_API_URL environment variable not set")
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+"/"+path, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	client := a.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 100 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("HTTP status %d", response.StatusCode)
	}
	return nil
}

func (a *Agent) path(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(a.Directory, name)
}

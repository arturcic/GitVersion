package buildagent

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/output"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
)

func TestCSharpBuildAgents(t *testing.T) {
	var corpus struct {
		Suites map[string]struct {
			Methods map[string]struct {
				Cases []struct {
					ID        string
					Arguments []json.RawMessage
				}
			}
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-build-agents.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	for suite, data := range corpus.Suites {
		t.Run(suite, func(t *testing.T) {
			for name, item := range data.Methods {
				t.Run(name, func(t *testing.T) {
					for _, c := range item.Cases {
						t.Run(c.ID, func(t *testing.T) { checkAgentCase(t, suite, name, c.Arguments) })
					}
				})
			}
		})
	}
}

func arg[T any](t *testing.T, data json.RawMessage) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func checkAgentCase(t *testing.T, suite, method string, args []json.RawMessage) {
	t.Helper()
	name := strings.TrimSuffix(suite, "Tests")
	if name == "BuildServerBase" {
		name = "BuildAgent"
	}
	env := map[string]string{}
	dir := t.TempDir()
	if name == "EnvRun" {
		path := filepath.Join(dir, "envrun.db")
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		env["ENVRUN_DATABASE"] = path
	}
	lookup := func(key string) (string, bool) { value, exists := env[key]; return value, exists }
	a := New(name, dir, lookup)
	if suite == "AppVeyorJsonTests" {
		switch method {
		case "BuildNumberRequestPreservesWireNameAndVersion":
			if got := string(BuildNumberRequest("1.2.3-alpha.4.build.42")); got != `{"version":"1.2.3-alpha.4.build.42"}` {
				t.Fatal(got)
			}
		case "VariableRequestPreservesWireNamesAndEscaping":
			got := string(VariableRequest("GitVersion_FullSemVer", arg[string](t, args[0])))
			if got != arg[string](t, args[1]) {
				t.Fatal(got)
			}
		default:
			t.Fatal(method)
		}
		return
	}
	checkString := func(got *string, want *string) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("branch/tag got %v, want %v", valueOrNull(got), valueOrNull(want))
		}
	}
	str := func(value string) *string { return &value }
	if strings.Contains(method, "CanApply") || strings.Contains(method, "CanNotApply") || method == "CorrectlyIdentifiesCodeBuildPresenceFromSourceVersion" || method == "CorrectlyIdentifiesCodeBuildPresenceFromWebHook" {
		for _, registered := range registry {
			if registered.name == name {
				env[registered.variable] = "true"
			}
		}
		expected := true
		if strings.Contains(method, "NotSet") || strings.Contains(method, "NotApply") || strings.Contains(method, "IsNotSet") {
			clear(env)
			expected = false
		}
		if method == "CorrectlyIdentifiesCodeBuildPresenceFromSourceVersion" {
			clear(env)
			env["CODEBUILD_SOURCE_VERSION"] = "a value"
		}
		if method == "CorrectlyIdentifiesCodeBuildPresenceFromWebHook" {
			clear(env)
			env["CODEBUILD_WEBHOOK_HEAD_REF"] = "a value"
		}
		if name == "EnvRun" && expected {
			path := filepath.Join(dir, "envrun.db")
			env["ENVRUN_DATABASE"] = path
		}
		if got := a.CanApply(); got != expected {
			t.Fatalf("presence %t, want %t", got, expected)
		}
		return
	}

	switch method {
	case "GetCurrentTagOnlyReturnsAnExplicitTagReference":
		for i, key := range []string{"GITHUB_REF_TYPE", "GITHUB_REF"} {
			if value := arg[*string](t, args[i]); value != nil {
				env[key] = *value
			}
		}
		checkString(a.CurrentTag(), arg[*string](t, args[2]))
		return
	case "GetCurrentTagShouldUseCommitTagName":
		if value := arg[*string](t, args[0]); value != nil {
			env["CI_COMMIT_TAG"] = *value
		}
		checkString(a.CurrentTag(), arg[*string](t, args[1]))
		return
	case "GetCurrentBranchShouldHandleBranches":
		switch name {
		case "AzurePipelines":
			env["BUILD_SOURCEBRANCH"] = "refs/heads/main"
			checkString(a.CurrentBranch(), str("refs/heads/main"))
		case "GitHubActions":
			env["GITHUB_REF_TYPE"] = "branch"
			env["GITHUB_REF"] = "refs/heads/main"
			checkString(a.CurrentBranch(), str("refs/heads/main"))
		case "BitBucketPipelines":
			env["BITBUCKET_BRANCH"] = "refs/heads/feature/my-work"
			checkString(a.CurrentBranch(), str("refs/heads/feature/my-work"))
		case "BuildKite":
			env["BUILDKITE_BRANCH"] = "main"
			env["BUILDKITE_PULL_REQUEST"] = "false"
			checkString(a.CurrentBranch(), str("main"))
		case "SpaceAutomation":
			env["JB_SPACE_GIT_BRANCH"] = "refs/heads/master"
			checkString(a.CurrentBranch(), str("refs/heads/master"))
		case "GitLabCi":
			env["CI_COMMIT_REF_NAME"] = arg[string](t, args[0])
			checkString(a.CurrentBranch(), arg[*string](t, args[1]))
			checkString(a.CurrentTag(), nil)
		default:
			t.Fatal(name)
		}
		return
	case "GetCurrentBranchShouldHandleTags":
		switch name {
		case "AzurePipelines":
			env["BUILD_SOURCEBRANCH"] = "refs/tags/1.0.0"
			checkString(a.CurrentBranch(), nil)
		case "GitHubActions":
			env["GITHUB_REF_TYPE"] = "tag"
			env["GITHUB_REF"] = "refs/tags/1.0.0"
			checkString(a.CurrentBranch(), nil)
		case "BitBucketPipelines":
			env["BITBUCKET_TAG"] = "refs/heads/tags/1.2.3"
			checkString(a.CurrentBranch(), nil)
		case "SpaceAutomation":
			env["JB_SPACE_GIT_BRANCH"] = "refs/tags/1.0.0"
			checkString(a.CurrentBranch(), str("refs/tags/1.0.0"))
		case "GitLabCi":
			env["CI_COMMIT_REF_NAME"], env["CI_COMMIT_TAG"] = arg[string](t, args[0]), arg[string](t, args[1])
			checkString(a.CurrentBranch(), arg[*string](t, args[2]))
		default:
			t.Fatal(name)
		}
		return
	case "GetCurrentBranchShouldHandlePullRequests":
		want := str("refs/pull/1/merge")
		switch name {
		case "AzurePipelines":
			env["BUILD_SOURCEBRANCH"] = *want
		case "GitHubActions":
			env["GITHUB_REF_TYPE"] = "branch"
			env["GITHUB_REF"] = *want
		case "SpaceAutomation":
			env["JB_SPACE_GIT_BRANCH"] = *want
		case "BitBucketPipelines":
			env["BITBUCKET_PR_ID"] = *want
			want = nil
		case "BuildKite":
			env["BUILDKITE_BRANCH"] = "feature/new"
			env["BUILDKITE_PULL_REQUEST"] = "55"
			want = str("refs/pull/55/head")
		default:
			t.Fatal(name)
		}
		checkString(a.CurrentBranch(), want)
		return
	case "GetCurrentBranchShouldHandleMergeRequestRefPaths":
		env["CI_MERGE_REQUEST_REF_PATH"] = arg[string](t, args[0])
		checkString(a.CurrentBranch(), arg[*string](t, args[1]))
		checkString(a.CurrentTag(), nil)
		return
	case "GetCurrentBranchShouldPreferTagOverMergeRequestRefPath":
		env["CI_COMMIT_TAG"] = "v1.0.0"
		env["CI_MERGE_REQUEST_REF_PATH"] = "refs/merge-requests/1/head"
		checkString(a.CurrentBranch(), nil)
		return
	case "GetCurrentBranchShouldFallBackToRefNameWhenMergeRequestRefPathIsEmpty":
		env["CI_MERGE_REQUEST_REF_PATH"] = ""
		env["CI_COMMIT_REF_NAME"] = "developer"
		checkString(a.CurrentBranch(), str("developer"))
		return
	case "PicksUpBranchNameFromEnvironmentFromSourceVersion":
		env["CODEBUILD_SOURCE_VERSION"] = "refs/heads/main"
		checkString(a.CurrentBranch(), str("refs/heads/main"))
		return
	case "PicksUpBranchNameFromEnvironmentFromWebHook":
		env["CODEBUILD_WEBHOOK_HEAD_REF"] = "refs/heads/main"
		checkString(a.CurrentBranch(), str("refs/heads/main"))
		return
	case "JenkinsTakesLocalBranchNameNotRemoteName":
		env["GIT_BRANCH"] = "origin/main"
		checkString(a.CurrentBranch(), str("origin/main"))
		env["GIT_LOCAL_BRANCH"] = "main"
		checkString(a.CurrentBranch(), str("main"))
		return
	case "JenkinsTakesBranchNameInPipelineAsCode":
		env["BRANCH_NAME"] = "main"
		checkString(a.CurrentBranch(), str("main"))
		return
	}
	if strings.HasPrefix(method, "GetCurrentBranchShould") && name == "Drone" {
		want := "droneBranch"
		env["DRONE_BRANCH"] = want
		switch method {
		case "GetCurrentBranchShouldDroneBranchInCaseOfPush":
			env["DRONE_PULL_REQUEST"] = ""
		case "GetCurrentBranchShouldUseDroneSourceBranchInCaseOfPullRequestAndNonEmptyDroneSourceBranch":
			env["DRONE_PULL_REQUEST"] = "1"
			env["DRONE_SOURCE_BRANCH"] = "droneSourceBranch"
			want = "droneSourceBranch"
		case "GetCurrentBranchShouldUseSourceBranchFromCiCommitRefSpecInCaseOfPullRequestAndEmptyDroneSourceBranch":
			env["DRONE_PULL_REQUEST"] = "1"
			env["DRONE_SOURCE_BRANCH"] = ""
			env["CI_COMMIT_REFSPEC"] = "droneSourceBranch:droneDestinationBranch"
			want = "droneSourceBranch"
		case "GetCurrentBranchShouldUseDroneBranchInCaseOfPullRequestAndEmptyDroneSourceBranchAndCiCommitRefSpec":
			env["DRONE_PULL_REQUEST"] = "1"
			env["DRONE_SOURCE_BRANCH"] = ""
			env["CI_COMMIT_REFSPEC"] = ""
		case "GetCurrentBranchShouldUseDroneBranchInCaseOfPullRequestAndEmptyDroneSourceBranchAndInvalidFormatOfCiCommitRefSpec":
			env["DRONE_PULL_REQUEST"] = "1"
			env["DRONE_SOURCE_BRANCH"] = ""
			env["CI_COMMIT_REFSPEC"] = "droneSourceBranch;droneDestinationBranch"
		default:
			t.Fatal(method)
		}
		checkString(a.CurrentBranch(), str(want))
		return
	}
	switch method {
	case "ShouldSetBuildNumber", "MissingEnvShouldNotBlowUp", "AzurePipelinesBuildNumberWithFullSemVer", "AzurePipelinesBuildNumberWithSemVer",
		"CalculateVersionOnMainBranch", "CalculateVersionOnDevelopBranch", "CalculateVersionOnFeatureBranch":
		checkBuildNumber(t, a, env, method, args)
	case "ShouldSetOutputVariables", "SkipEmptyOutputVariables":
		checkOutputVariables(t, a, method, args)
	case "BuildNumberIsFullSemVer", "WriteAllVariablesToTheTextWriter", "ShouldWriteIntegration", "ShouldNotWriteIntegration":
		checkIntegration(t, a, env, method)
	default:
		t.Fatal(suite, method)
	}
}

func valueOrNull(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func checkBuildNumber(t *testing.T, a *Agent, env map[string]string, method string, args []json.RawMessage) {
	t.Helper()
	version, want := "0.0.0-Beta4.7", "0.0.0-Beta4.7"
	variables := map[string]any{}
	switch a.Name {
	case "TeamCity":
		version, want = "0.0.0-Unstable4", "##teamcity[buildNumber '0.0.0-Unstable4']"
	case "MyGet":
		version, want = "0.0.0-Unstable4", "##myget[buildNumber '0.0.0-Unstable4']"
	case "ContinuaCi":
		want = "@@continua[setBuildVersion value='0.0.0-Beta4.7']"
	case "GitHubActions", "BuildKite", "SpaceAutomation":
		version, want = "1.0.0", ""
	case "EnvRun":
		version, want = arg[string](t, args[0]), arg[string](t, args[0])
	case "AzurePipelines":
		version = "0.0.0-Unstable4"
		if method == "MissingEnvShouldNotBlowUp" {
			want = version
		} else if method == "ShouldSetBuildNumber" {
			env["BUILD_BUILDNUMBER"] = "Some Build_Value $(GitVersion_FullSemVer) 20151310.3 $(UnknownVar) Release"
			want = "##vso[build.updatebuildnumber]Some Build_Value 0.0.0-Unstable4 20151310.3 $(UnknownVar) Release"
		} else {
			env["BUILD_BUILDNUMBER"] = arg[string](t, args[0])
			version = arg[string](t, args[1])
			want = "##vso[build.updatebuildnumber]" + arg[string](t, args[2])
			if method == "AzurePipelinesBuildNumberWithSemVer" {
				variables["SemVer"] = version
				version = ""
			}
		}
	case "BitBucketPipelines":
		switch method {
		case "CalculateVersionOnMainBranch":
			env["BITBUCKET_BRANCH"] = "refs/heads/main"
			version = "1.2.3"
		case "CalculateVersionOnDevelopBranch":
			env["BITBUCKET_BRANCH"] = "refs/heads/develop"
			version = "1.2.3-unstable.4"
		case "CalculateVersionOnFeatureBranch":
			env["BITBUCKET_BRANCH"] = "refs/heads/feature/my-work"
			version = "1.2.3-beta.4"
		default:
			t.Fatal(method)
		}
		want = version
	}
	variables["FullSemVer"] = version
	got, err := a.BuildNumber(t.Context(), variables)
	if err != nil || got != want {
		t.Fatalf("%s build number %q != %q, %v", a.Name, got, want, err)
	}
}

func checkOutputVariables(t *testing.T, a *Agent, method string, args []json.RawMessage) {
	t.Helper()
	name, value := "name", "value"
	want := []string{"GitVersion_name=value"}
	switch a.Name {
	case "GitHubActions":
		name, value = "GitVersion_Something", "1.0.0"
		if method == "SkipEmptyOutputVariables" {
			name, value = "Hello", ""
		}
		want = []string{}
	case "BuildKite":
		name, value = "Foo", "Bar"
		want = []string{}
	case "AzurePipelines":
		name, value = "Foo", arg[string](t, args[0])
		want = []string{"##vso[task.setvariable variable=GitVersion_Foo]" + value, "##vso[task.setvariable variable=GitVersion_Foo;isOutput=true]" + value}
	case "TeamCity", "MyGet":
		name, value = "Foo", "0.8.0-unstable568 Branch:'develop' Sha:'ee69bff1087ebc95c6b43aa2124bd58f5722e0cb'"
		escaped := "0.8.0-unstable568 Branch:|'develop|' Sha:|'ee69bff1087ebc95c6b43aa2124bd58f5722e0cb|'"
		if a.Name == "MyGet" {
			want = []string{"##myget[setParameter name='GitVersion.Foo' value='" + escaped + "']"}
		} else {
			want = []string{"##teamcity[setParameter name='GitVersion.Foo' value='" + escaped + "']", "##teamcity[setParameter name='system.GitVersion.Foo' value='" + escaped + "']"}
		}
	case "EnvRun":
		name, value = arg[string](t, args[0]), arg[string](t, args[1])
		want = []string{arg[string](t, args[2])}
	}
	if a.env("GitVersion_Something") != "" {
		t.Fatal("unexpected environment mutation")
	}
	got, err := a.OutputVariables(t.Context(), name, value)
	if err != nil || !slices.Equal(got, want) || a.env("GitVersion_Something") != "" {
		t.Fatalf("%v != %v, %v", got, want, err)
	}
}

func checkIntegration(t *testing.T, a *Agent, env map[string]string, method string) {
	t.Helper()
	var writes bytes.Buffer
	if a.Name == "GitHubActions" {
		file := filepath.Join(a.Directory, "github.env")
		env["GITHUB_ENV"] = file
		variables := map[string]any{"Major": "1.0.0"}
		update := method != "ShouldNotWriteIntegration"
		if !update {
			variables = map[string]any{"FullSemVer": "1.0.0"}
		}
		if a.env("GitVersion_Major") != "" {
			t.Fatal("unexpected environment mutation")
		}
		if err := a.WriteIntegration(t.Context(), &writes, variables, update); err != nil {
			t.Fatal(err)
		}
		if update {
			expected := "Set Build Number for 'GitHubActions'.\n\nSet Output Variables for 'GitHubActions'.\nWriting version variables to $GITHUB_ENV file for 'GitHubActions'.\n"
			if writes.String() != expected {
				t.Fatal(writes.String())
			}
			content, err := os.ReadFile(file)
			if err != nil || string(content) != "GitVersion_Major=1.0.0"+integrationNewline() {
				t.Fatalf("%s: %v", content, err)
			}
		} else if strings.Contains(writes.String(), "Set Build Number for ") {
			t.Fatal(writes.String())
		}
		if a.env("GitVersion_Major") != "" {
			t.Fatal("environment mutated")
		}
		return
	}
	number := int64(1)
	build := int64(5)
	version := semver.Version{Major: 1, Minor: 2, Patch: 3, Label: "beta", Number: &number}
	date := time.Date(2014, 3, 6, 23, 59, 59, 0, time.UTC)
	sha := "commitSha"
	if a.Name == "BitBucketPipelines" {
		sha = "f28807e615e9f06aec8a33c87780374e0c1f6fb8"
		date = time.Date(2022, 4, 6, 16, 10, 59, 0, time.FixedZone("fixture", 10*60*60))
	}
	variables, err := output.FromSemanticVersion(output.VariableInput{Version: version, Build: &build, SHA: sha, Date: &date}, config.OutputSettings{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.WriteIntegration(t.Context(), &writes, variables, true); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(writes.String(), "\n")
	if len(lines) < 2 || lines[1] != "1.2.3-beta.1+5" {
		t.Fatal(writes.String())
	}
	if method == "BuildNumberIsFullSemVer" {
		writes.Reset()
		if err := a.WriteIntegration(t.Context(), &writes, variables, false); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(writes.String(), "Set Build Number for ") {
			t.Fatal(writes.String())
		}
		return
	}
	if a.PropertyFile == nil {
		t.Fatal("missing property file")
	}
	content, err := os.ReadFile(a.path(*a.PropertyFile))
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"GitVersion_Major=1", "GitVersion_Minor=2"}
	if a.Name == "BitBucketPipelines" {
		expected = []string{"export GITVERSION_MAJOR=1", "export GITVERSION_MINOR=2", "export GITVERSION_SHA=" + sha, "export GITVERSION_COMMITDATE=2022-04-06"}
		ps, err := os.ReadFile(a.path(*a.PowerShellFile))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`$GITVERSION_MAJOR = "1"`, `$GITVERSION_MINOR = "2"`, `$GITVERSION_SHA = "` + sha + `"`, `$GITVERSION_COMMITDATE = "2022-04-06"`} {
			if !bytes.Contains(ps, []byte(want)) {
				t.Fatalf("missing %q in %s", want, ps)
			}
		}
	}
	for _, want := range expected {
		if !bytes.Contains(content, []byte(want)) {
			t.Fatalf("missing %q in %s", want, content)
		}
	}
}

package buildagent

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAppVeyorUsesLocalRequestPayloads(t *testing.T) {
	var methods, paths, bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		methods = append(methods, r.Method)
		paths = append(paths, r.URL.Path)
		bodies = append(bodies, string(body))
		if r.Header.Get("Content-Type") != "application/json; charset=utf-8" {
			t.Error(r.Header)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	agent := New("AppVeyor", t.TempDir(), func(key string) (string, bool) {
		switch key {
		case "APPVEYOR_API_URL":
			return server.URL + "/", true
		case "APPVEYOR_BUILD_NUMBER":
			return "42", true
		}
		return "", false
	})
	agent.HTTPClient = server.Client()
	number, err := agent.BuildNumber(t.Context(), map[string]any{"FullSemVer": "1.2.3"})
	if err != nil || number != "Set AppVeyor build number to '1.2.3'." {
		t.Fatal(number, err)
	}
	lines, err := agent.OutputVariables(t.Context(), "Foo", "quoted \"value\"")
	if err != nil || len(lines) != 1 {
		t.Fatal(lines, err)
	}
	if strings.Join(methods, ",") != "PUT,POST" || strings.Join(paths, ",") != "/api/build,/api/build/variables" ||
		bodies[0] != `{"version":"1.2.3.build.42"}` ||
		bodies[1] != `{"name":"GitVersion_Foo","value":"quoted \u0022value\u0022"}` {
		t.Fatal(methods, paths, bodies)
	}
}

func TestAgentPreservesEmptyBranchValue(t *testing.T) {
	for name, key := range map[string]string{
		"AzurePipelines": "BUILD_SOURCEBRANCH", "GitHubActions": "GITHUB_REF", "GitLabCi": "CI_COMMIT_REF_NAME",
		"BuildKite": "BUILDKITE_BRANCH", "CodeBuild": "CODEBUILD_SOURCE_VERSION", "Drone": "DRONE_BRANCH",
		"Jenkins": "GIT_LOCAL_BRANCH", "SpaceAutomation": "JB_SPACE_GIT_BRANCH", "AppVeyor": "APPVEYOR_REPO_BRANCH",
	} {
		t.Run(name, func(t *testing.T) {
			agent := New(name, t.TempDir(), func(k string) (string, bool) { return "", k == key })
			if value := agent.CurrentBranch(); value == nil || *value != "" {
				t.Fatal(value)
			}
			agent.Environment = func(string) (string, bool) { return "", false }
			if value := agent.CurrentBranch(); value != nil {
				t.Fatal(value)
			}
		})
	}
}

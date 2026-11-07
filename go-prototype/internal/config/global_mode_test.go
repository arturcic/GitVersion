package config

import (
	"fmt"
	"testing"
)

func TestGlobalDeploymentModePrecedence(t *testing.T) {
	for _, workflow := range []string{"GitHubFlow/v1", "GitFlow/v1"} {
		for _, mode := range []Mode{Manual, Delivery, Deployment} {
			t.Run(workflow+"/"+string(mode), func(t *testing.T) {
				cfg, err := Parse([]byte(fmt.Sprintf("workflow: %s\ncalculation:\n  mode: %s\n", workflow, mode)))
				if err != nil {
					t.Fatal(err)
				}
				for _, kind := range cfg.Kinds() {
					want := Manual
					switch kind {
					case "main", "support":
						want = mode
					case "develop", "pull-request":
						want = Delivery
					}
					if got := cfg.Options(kind).Mode; got != want {
						t.Errorf("%s: got %s, want %s", kind, got, want)
					}
				}
				for _, override := range []Mode{Manual, Delivery, Deployment} {
					cfg, err = Parse([]byte(fmt.Sprintf("workflow: %s\ncalculation:\n  branches:\n    main:\n      mode: %s\n  mode: %s\n", workflow, override, mode)))
					if err != nil {
						t.Fatal(err)
					}
					if cfg.Main.Mode != override {
						t.Fatalf("branch override: got %s, want %s", cfg.Main.Mode, override)
					}
				}
			})
		}
	}
}

func TestGlobalDeploymentModeValidation(t *testing.T) {
	for _, value := range []string{"Mainline", "manualdeployment", "null", "123", "[]", "{}", "''"} {
		t.Run(value, func(t *testing.T) {
			if _, err := Parse([]byte("workflow: GitFlow/v1\ncalculation:\n  mode: " + value + "\n")); err == nil {
				t.Fatal("invalid mode accepted")
			}
		})
	}
}

package fixtures

import (
	"reflect"
	"strings"
	"testing"
)

func TestResetFixturesDeterministic(t *testing.T) {
	recipes := ResetRecipes()
	if len(recipes) != 72 || !reflect.DeepEqual(recipes, ResetRecipes()) {
		t.Fatal("reset recipe inventory or determinism")
	}
	checkpoints := 0
	for _, r := range recipes {
		for _, s := range r.Steps {
			if s.Op == "checkpoint" {
				checkpoints++
			}
		}
	}
	if checkpoints != 192 {
		t.Fatalf("checkpoints: %d", checkpoints)
	}
	a, err := GeneratedResets(20261106, 100)
	if err != nil {
		t.Fatal(err)
	}
	b, err := GeneratedResets(20261106, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("generated resets are nondeterministic")
	}
	original, err := GeneratedPolicies(20261106, 100)
	if err != nil {
		t.Fatal(err)
	}
	resets := 0
	for i, r := range a {
		if r.Config != original[i].Config || len(r.Steps) != len(original[i].Steps) {
			t.Fatal("reset layer changed configuration or graph inventory")
		}
		for j, s := range r.Steps {
			if strings.Contains(strings.ToLower(s.Message), "=semver:") {
				resets++
			}
			s.Message = original[i].Steps[j].Message
			if !reflect.DeepEqual(s, original[i].Steps[j]) {
				t.Fatal("reset layer changed graph")
			}
		}
	}
	if resets == 0 {
		t.Fatal("no reset directives generated")
	}
}

package fixtures

import (
	"reflect"
	"testing"
)

func TestWorkflowCorpusDeterminism(t *testing.T) {
	a := WorkflowRecipes()
	if !reflect.DeepEqual(a, WorkflowRecipes()) {
		t.Fatal("nondeterministic workflow recipes")
	}
	count := 0
	for _, r := range a {
		for _, s := range r.Steps {
			if s.Op == "checkpoint" {
				count++
			}
		}
	}
	if len(a) != 29 || count != 166 {
		t.Fatalf("inventory %d/%d", len(a), count)
	}
	b, err := GeneratedWorkflows(20261106, 64)
	if err != nil {
		t.Fatal(err)
	}
	c, err := GeneratedWorkflows(20261106, 64)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b, c) {
		t.Fatal("nondeterministic generated workflows")
	}
}

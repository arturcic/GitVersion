package fixtures

import (
	"errors"
	"reflect"
	"testing"
)

func TestRecipeReductionPreservesFailureAndInput(t *testing.T) {
	r := Recipe{ID: "reduce", Config: "unchanged", Steps: []Step{{Op: "commit", ID: "noise"}, {Op: "commit", ID: "required"}, {Op: "tag", Name: "1.2.3"}, {Op: "checkpoint", ID: "head"}}}
	before := append([]Step(nil), r.Steps...)
	predicate := func(r Recipe) (bool, error) {
		for _, s := range r.Steps {
			if s.ID == "required" {
				return true, nil
			}
		}
		return false, nil
	}
	got, calls, err := Reduce(r, predicate)
	if err != nil || calls < 3 || len(got.Steps) != 2 || got.Steps[0].ID != "required" || got.Steps[1].Op != "checkpoint" || got.Config != "unchanged" {
		t.Fatalf("invalid reduction %+v %d %v", got, calls, err)
	}
	if !reflect.DeepEqual(before, r.Steps) {
		t.Fatal("mutated input recipe")
	}
	for i, s := range got.Steps {
		if s.Op == "checkpoint" {
			continue
		}
		candidate := got
		candidate.Steps = append(append([]Step(nil), got.Steps[:i]...), got.Steps[i+1:]...)
		if failed, _ := predicate(candidate); failed {
			t.Fatal("not one-step minimal")
		}
	}
}

func TestRecipeReductionNoFailureAndErrors(t *testing.T) {
	r := Recipe{ID: "no-failure", Steps: []Step{{Op: "commit", ID: "root"}, {Op: "checkpoint", ID: "head"}}}
	got, calls, err := Reduce(r, func(Recipe) (bool, error) { return false, nil })
	if err != nil || calls != 1 || !reflect.DeepEqual(got, r) {
		t.Fatal("changed nonfailing case")
	}
	want := errors.New("oracle unavailable")
	_, calls, err = Reduce(r, func(Recipe) (bool, error) { return false, want })
	if !errors.Is(err, want) || calls != 1 {
		t.Fatal("lost oracle failure")
	}
	invocations := 0
	got, calls, err = Reduce(r, func(Recipe) (bool, error) {
		invocations++
		if invocations == 1 {
			return true, nil
		}
		return false, want
	})
	if !errors.Is(err, want) || calls != 2 || !reflect.DeepEqual(got, r) {
		t.Fatal("lost failure during reduction or accepted an unchecked candidate")
	}
}

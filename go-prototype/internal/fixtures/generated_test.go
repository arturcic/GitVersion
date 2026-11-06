package fixtures

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestGeneratedRecipesDeterministic(t *testing.T) {
	a, err := Generated(20261106, 12)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Generated(20261106, 12)
	c, _ := Generated(20261107, 12)
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	cj, _ := json.Marshal(c)
	if string(aj) != string(bj) || string(aj) == string(cj) {
		t.Fatal("seed determinism failed")
	}
	root := t.TempDir()
	x, err := Generate(a, filepath.Join(root, "first"))
	if err != nil {
		t.Fatal(err)
	}
	y, err := Generate(b, filepath.Join(root, "second"))
	if err != nil {
		t.Fatal(err)
	}
	if len(x) != 12 || len(y) != 12 {
		t.Fatal("lost checkpoint")
	}
	for i := range x {
		if x[i].Head != y[i].Head || x[i].Digest != y[i].Digest {
			t.Fatalf("nonrepeatable %s", x[i].ID)
		}
	}
}

func TestGeneratedRecipeBounds(t *testing.T) {
	for _, n := range []int{-1, 0, 10001} {
		if _, err := Generated(1, n); err == nil {
			t.Fatalf("accepted %d", n)
		}
	}
	for _, n := range []int{1, 1000, 10000} {
		r, err := Generated(1, n)
		if err != nil || len(r) != n {
			t.Fatalf("count %d: %v", n, err)
		}
	}
}

package fixtures

import (
	"encoding/json"
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"path/filepath"
	"testing"
)

func TestPolicyCorpusAndGeneratedOverrides(t *testing.T) {
	recipes, err := PolicyRecipes()
	if err != nil {
		t.Fatal(err)
	}
	if len(recipes) != 50 {
		t.Fatalf("policy recipe inventory %d", len(recipes))
	}
	cps, err := Generate(recipes, filepath.Join(t.TempDir(), "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cps) != 250 {
		t.Fatalf("checkpoint inventory %d", len(cps))
	}
	for _, r := range recipes {
		if _, err := config.Parse([]byte(r.Config)); err != nil {
			t.Fatalf("%s: %v", r.ID, err)
		}
	}
	a, err := GeneratedPolicies(20261106, 20)
	if err != nil {
		t.Fatal(err)
	}
	b, err := GeneratedPolicies(20261106, 20)
	if err != nil {
		t.Fatal(err)
	}
	aj, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	bj, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(aj) != string(bj) {
		t.Fatal("expanded seed not deterministic")
	}
	modes := map[config.Mode]bool{}
	messages := map[string]bool{}
	explicit, inherited := false, false
	for _, r := range a {
		c, err := config.Parse([]byte(r.Config))
		if err != nil {
			t.Fatal(err)
		}
		for _, branch := range []string{"main", "feature/topic"} {
			e, err := c.Effective(branch)
			if err != nil {
				t.Fatal(err)
			}
			modes[e.Mode] = true
			messages[e.Messages] = true
			if branch != "main" {
				if e.Inherit {
					inherited = true
				} else {
					explicit = true
				}
			}
		}
	}
	if len(modes) != 3 || len(messages) != 3 || !explicit || !inherited {
		t.Fatal("generated policy partitions missing")
	}
	if _, err := GeneratedPolicies(1, 0); err == nil {
		t.Fatal("accepted empty generated corpus")
	}
}

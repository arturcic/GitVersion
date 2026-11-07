package semver

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

func TestCSharpSemanticVersionIncrements(t *testing.T) {
	var corpus struct {
		Revision string `json:"revision"`
		Methods  map[string][]struct {
			ID        string  `json:"id"`
			Value     string  `json:"value"`
			Increment string  `json:"increment"`
			Label     *string `json:"label"`
			Mode      string  `json:"mode"`
			Expected  string  `json:"expected"`
		} `json:"methods"`
	}
	data, err := os.ReadFile("../../testdata/csharp-semver-increments.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Methods) != 12 {
		t.Fatal("source inventory differs")
	}
	names := []string{}
	for name := range corpus.Methods {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, method := range names {
		t.Run(method, func(t *testing.T) {
			cases := corpus.Methods[method]
			if len(cases) != 96 {
				t.Fatal("parameter inventory differs")
			}
			for _, tc := range cases {
				t.Run(tc.ID, func(t *testing.T) {
					v, err := Parse(tc.Value)
					if err != nil {
						t.Fatal(err)
					}
					inc, err := ParseIncrement(tc.Increment)
					if err != nil {
						t.Fatal(err)
					}
					modes := map[string]IncrementMode{"Standard": Standard, "Force": Force, "EnsureIntegrity": EnsureIntegrity}
					mode, ok := modes[tc.Mode]
					if !ok {
						t.Fatal("unknown mode", tc.Mode)
					}
					got, err := v.BumpWithMode(inc, tc.Label, mode)
					if err != nil {
						t.Fatal(err)
					}
					if got.String() != tc.Expected {
						t.Fatalf("input=%s increment=%s label=%v mode=%s: want %s, got %s", tc.Value, tc.Increment, tc.Label, tc.Mode, tc.Expected, got)
					}
					if v.String() != tc.Value {
						t.Fatal("increment mutated original version")
					}
				})
			}
		})
	}
}

func TestIncrementModeValidation(t *testing.T) {
	v, err := Parse("1.2.3-beta.4")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []IncrementMode{-1, EnsureIntegrity + 1} {
		if _, err := v.BumpWithMode(Patch, nil, mode); err == nil {
			t.Fatal("invalid mode accepted", mode)
		}
	}
	if _, err := v.BumpWithMode(Major+1, nil, Standard); err == nil {
		t.Fatal("invalid increment accepted")
	}
	if v.String() != "1.2.3-beta.4" {
		t.Fatal("invalid increment mutated original")
	}
}

func TestCSharpSemanticVersionParsingAndSorting(t *testing.T) {
	var corpus struct {
		Revision string `json:"revision"`
		Methods  map[string][]struct {
			ID        string  `json:"id"`
			Value     string  `json:"value"`
			TagPrefix *string `json:"tag_prefix"`
			Left      string  `json:"left"`
			Right     string  `json:"right"`
			Sign      int     `json:"sign"`
		} `json:"methods"`
	}
	data, err := os.ReadFile("../../testdata/csharp-semver-basic.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Methods) != 2 {
		t.Fatal("source inventory differs")
	}
	for _, method := range []string{"ValidateInvalidVersionParsing", "VersionSorting"} {
		t.Run(method, func(t *testing.T) {
			cases := corpus.Methods[method]
			if len(cases) != 3 {
				t.Fatal("assertion inventory differs")
			}
			for _, tc := range cases {
				t.Run(tc.ID, func(t *testing.T) {
					if method == "ValidateInvalidVersionParsing" {
						if tc.TagPrefix == nil {
							if _, err := Parse(tc.Value); err == nil {
								t.Fatal("invalid version accepted")
							}
						} else {
							if *tc.TagPrefix != "[vV]?" {
								t.Fatal("unsupported port tag prefix")
							}
							if _, ok, err := ParseTag(tc.Value); ok || err != nil {
								t.Fatal("invalid tag accepted or misclassified", ok, err)
							}
						}
					} else {
						left, err := Parse(tc.Left)
						if err != nil {
							t.Fatal(err)
						}
						right, err := Parse(tc.Right)
						if err != nil {
							t.Fatal(err)
						}
						result := left.Compare(right)
						if (tc.Sign > 0 && result <= 0) || (tc.Sign < 0 && result >= 0) || tc.Sign == 0 {
							t.Fatalf("%s compared to %s: want sign %d, got %d", left, right, tc.Sign, result)
						}
					}
				})
			}
		})
	}
}

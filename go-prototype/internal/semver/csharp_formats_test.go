package semver

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestCSharpSemanticVersionParsingAndFormats(t *testing.T) {
	var corpus struct {
		Revision string `json:"revision"`
		Methods  map[string][]struct {
			ID, Operation, Value, Expected, Format string
			Mode                                   ParseMode
			Prefix, Label, Branch, SHA, Other      *string
			Major, Minor, Patch                    int64
			Number, Count                          *int64
			Error                                  bool
		} `json:"methods"`
	}
	data, err := os.ReadFile("../../testdata/csharp-semver-formats.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Methods) != 7 {
		t.Fatal("source inventory differs")
	}
	names := []string{}
	total := 0
	for name, cases := range corpus.Methods {
		names = append(names, name)
		total += len(cases)
	}
	sort.Strings(names)
	if total != 49 {
		t.Fatal("parameter inventory differs")
	}
	for _, method := range names {
		t.Run(method, func(t *testing.T) {
			for _, tc := range corpus.Methods[method] {
				t.Run(tc.ID, func(t *testing.T) {
					var v Version
					var err error
					if tc.Operation == "parse" {
						v, err = ParseDetailed(tc.Value, tc.Prefix, tc.Mode)
						if err != nil {
							t.Fatal(err)
						}
						if v.Major != tc.Major || v.Minor != tc.Minor || v.Patch != tc.Patch || tc.Label == nil || v.Label != *tc.Label || !reflect.DeepEqual(v.Number, tc.Number) {
							t.Fatalf("parsed version differs: %+v", v)
						}
						want := BuildMetadata{CommitsSinceTag: tc.Count, Branch: tc.Branch, SHA: tc.SHA, Other: tc.Other}
						if !reflect.DeepEqual(v.Metadata, want) {
							t.Fatalf("parsed metadata: want %+v, got %+v", want, v.Metadata)
						}
						actual, err := v.Format("i")
						if err != nil || actual != tc.Expected {
							t.Fatalf("informational format: want %q, got %q (%v)", tc.Expected, actual, err)
						}
					} else if tc.Operation == "format" {
						v = Version{Major: tc.Major, Minor: tc.Minor, Patch: tc.Patch}
						// Preserve the original C# BuildSemVer helper's nullable construction rules.
						if tc.Label != nil {
							v.Label = *tc.Label
							v.Number = tc.Number
						}
						if tc.Count != nil {
							v.Metadata = BuildMetadata{CommitsSinceTag: tc.Count, Branch: tc.Branch, SHA: tc.SHA, Other: tc.Other}
						}
						actual, err := v.Format(tc.Format)
						if tc.Error {
							if err == nil {
								t.Fatal("invalid format accepted")
							}
							return
						}
						if err != nil || actual != tc.Expected {
							t.Fatalf("format: want %q, got %q (%v)", tc.Expected, actual, err)
						}
					} else {
						t.Fatal("unknown operation", tc.Operation)
					}
				})
			}
		})
	}
}

func TestDetailedVersionParsingBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		mode        ParseMode
	}{
		{"strict-leading-zero", "01.2.3", Strict}, {"strict-two-components", "1.2", Strict},
		{"component-overflow", "9223372036854775808.0.0", Strict}, {"prerelease-overflow", "1.2.3-a.9223372036854775808", Strict},
		{"fourth-part-overflow", "1.2.3.2147483648", Loose}, {"unknown-mode", "1.2.3", ParseMode("other")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseDetailed(tc.value, nil, tc.mode); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
	for _, value := range []string{"0.0.0", "9223372036854775807.0.0", "1.2.3-a.9223372036854775807"} {
		v, err := ParseDetailed(value, nil, Strict)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := v.Format("i"); err != nil || got != value {
			t.Fatal("64-bit boundary changed", got, err)
		}
	}
	prefix := "["
	if _, err := ParseDetailed("1.2.3", &prefix, Strict); err == nil {
		t.Fatal("invalid prefix accepted")
	}
	prefix = "package/"
	if _, err := ParseDetailed("1.2.3", &prefix, Strict); err == nil {
		t.Fatal("missing required prefix accepted")
	}
	if _, err := ParseDetailed("package/1.2.3", &prefix, Strict); err != nil {
		t.Fatal(err)
	}
	branch, other := "topic/😀", "meta+data"
	metadata := BuildMetadata{Branch: &branch, Other: &other}
	if got, err := metadata.Format("f"); err != nil || got != "Branch.topic---.meta-data" {
		t.Fatal("metadata sanitization changed", got, err)
	}
	if _, err := metadata.Format("invalid"); err == nil {
		t.Fatal("invalid metadata format accepted")
	}
	v, err := ParseDetailed("1.2.3+Branch.foo.Sha.abc", nil, Strict)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := v.Format("I"); err != nil || got != "1.2.3+Branch.foo.Sha.abc" {
		t.Fatal("metadata without count lost", got, err)
	}
	if got, err := v.Format("F"); err != nil || got != "1.2.3" {
		t.Fatal("full format must include only metadata count", got, err)
	}
}

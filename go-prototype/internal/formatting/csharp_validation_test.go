package formatting

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type originalCase struct {
	ID, Operation, Input, Expected string
	Template                       string
	SourceKind                     string `json:"source_kind"`
	Properties                     map[string]any
	Environment                    map[string]*string
	Value                          any
	Format                         string
	Success                        bool
	Priority                       int
	Error                          bool
	ExpectedTokens                 []string `json:"expected_tokens"`
}

func originalCases(t *testing.T, file string, methods, cases int) map[string][]originalCase {
	t.Helper()
	var corpus struct {
		Revision string
		Methods  map[string][]originalCase
	}
	raw, err := os.ReadFile("../../testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, cs := range corpus.Methods {
		n += len(cs)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Methods) != methods || n != cases {
		t.Fatal("pinned inventory changed")
	}
	return corpus.Methods
}
func runCases(t *testing.T, methods map[string][]originalCase, run func(*testing.T, originalCase)) {
	t.Helper()
	names := []string{}
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			for _, tc := range methods[name] {
				t.Run(tc.ID, func(t *testing.T) { run(t, tc) })
			}
		})
	}
}
func TestCSharpLabelTokens(t *testing.T) {
	runCases(t, originalCases(t, "csharp-label-tokenizer.json", 4, 33), func(t *testing.T, tc originalCase) {
		got, err := ParseTokens(tc.Input)
		if tc.Error {
			if err == nil {
				t.Fatal("malformed expression accepted")
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		names := []string{}
		for _, token := range got {
			names = append(names, token.Name)
		}
		if !reflect.DeepEqual(names, tc.ExpectedTokens) {
			t.Fatalf("want %q, got %q", tc.ExpectedTokens, names)
		}
	})
}
func TestCSharpInputSanitizers(t *testing.T) {
	for _, spec := range []struct {
		file           string
		methods, cases int
	}{{"csharp-sanitize-format.json", 7, 20}, {"csharp-sanitize-env-var-name.json", 6, 13}, {"csharp-sanitize-member-name.json", 7, 16}} {
		runCases(t, originalCases(t, spec.file, spec.methods, spec.cases), func(t *testing.T, tc originalCase) {
			var got string
			var err error
			s := InputSanitizer{}
			switch tc.Operation {
			case "SanitizeFormat":
				got, err = s.SanitizeFormat(tc.Input)
			case "SanitizeEnvVarName":
				got, err = s.SanitizeEnvVarName(&tc.Input)
			case "SanitizeMemberName":
				got, err = s.SanitizeMemberName(&tc.Input)
			default:
				t.Fatal("unknown operation")
			}
			if tc.Error {
				if err == nil || err.Error() != tc.Expected {
					t.Fatalf("want error %q, got %v", tc.Expected, err)
				}
				return
			}
			if err != nil || got != tc.Expected {
				t.Fatalf("want %q, got %q (%v)", tc.Expected, got, err)
			}
		})
	}
}
func TestLabelTokenKindsFormatsAndBoundaries(t *testing.T) {
	ptr := func(s string) *string { return &s }
	for _, tc := range []struct {
		name, input string
		want        []Token
	}{
		{"empty", "", []Token{}}, {"whitespace", " \t\u00a0", []Token{}},
		{"environment-format", `ENV:BUILD_LABEL:D4 ?? "fallback"`, []Token{{"BUILD_LABEL", Environment, ptr("D4")}, {"fallback", Literal, nil}}},
		{"empty-format", `Property:`, []Token{{"Property", Property, ptr("")}}},
		{"empty-literal", `""`, []Token{{"", Literal, nil}}},
		{"quoted-environment", `"env:NAME"`, []Token{{"env:NAME", Literal, nil}}},
		{"integer-boundaries", `-2147483648 ?? +2147483647 ?? 2147483648`, []Token{{"-2147483648", Literal, nil}, {"+2147483647", Literal, nil}, {"2147483648", Property, nil}}},
		{"one-trailing-question", `Property?`, []Token{{"Property", Property, nil}}},
		{"quoted-unicode", `"😀 A"`, []Token{{"😀 A", Literal, nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTokens(tc.input)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("want %+v, got %+v (%v)", tc.want, got, err)
			}
		})
	}
	for _, value := range []string{"x:a:b", "env:X:a:b", "?", "??", "x??", "x? ", `"x"y`} {
		if _, err := ParseTokens(value); err == nil {
			t.Fatalf("accepted malformed %q", value)
		}
	}
}
func TestSanitizerBoundariesAndNullNames(t *testing.T) {
	s := InputSanitizer{}
	for _, tc := range []struct {
		name  string
		call  func(*string) (string, error)
		max   int
		empty string
	}{{"environment", s.SanitizeEnvVarName, 200, "Environment variable name cannot be null or empty."}, {"member", s.SanitizeMemberName, 100, "Member name cannot be empty."}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.call(nil); err == nil || err.Error() != tc.empty {
				t.Fatalf("null: %v", err)
			}
			for _, n := range []int{1, tc.max - 1, tc.max} {
				v := strings.Repeat("A", n)
				got, err := tc.call(&v)
				if err != nil || got != v {
					t.Fatal(n, got, err)
				}
			}
			v := strings.Repeat("A", tc.max+1)
			if _, err := tc.call(&v); err == nil {
				t.Fatal("limit exceeded")
			}
			v = strings.Repeat("A", tc.max-1) + "😀"
			if _, err := tc.call(&v); err == nil || !strings.Contains(err.Error(), "too long") {
				t.Fatal("must count UTF16 units before validating characters", err)
			}
		})
	}
	for _, v := range []string{strings.Repeat("x", 49), strings.Repeat("x", 50), strings.Repeat("😀", 25)} {
		if got, err := s.SanitizeFormat(v); err != nil || got != v {
			t.Fatal("valid UTF16 length rejected", got, err)
		}
	}
	if _, err := s.SanitizeFormat(strings.Repeat("😀", 25) + "x"); err == nil {
		t.Fatal("UTF16 format limit ignored")
	}
	for _, v := range []string{"a\u007fb", "a\u0085b"} {
		if _, err := s.SanitizeFormat(v); err == nil {
			t.Fatal("control accepted")
		}
	}
	env := "build:LABEL-v1.0"
	if got, err := s.SanitizeEnvVarName(&env); err != nil || got != env {
		t.Fatal("allowed environment characters rejected", got, err)
	}
	member := "A-B"
	if _, err := s.SanitizeMemberName(&member); err == nil {
		t.Fatal("member hyphen accepted")
	}
}

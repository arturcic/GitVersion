package formatting

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCSharpTemplateFormatting(t *testing.T) {
	runCases(t, originalCases(t, "csharp-string-format-with-extension.json", 34, 43), func(t *testing.T, tc originalCase) {
		var source any = tc.Properties
		if tc.SourceKind == "semantic-object" {
			// C# semantic-object properties become exported Go fields. Keep the nested
			// metadata structure: flattening the fixture would bypass recursive lookup.
			source = struct {
				Major, Minor, Patch int64
				PreReleaseTag       struct {
					Name      string
					Number    int64
					HasNumber bool
				}
				BuildMetaData struct {
					Branch, VersionSourceSha, Sha, ShortSha string
					VersionSourceDistance                   int64
					CommitDate                              time.Time
				}
			}{Major: int64(tc.Properties["Major"].(float64)), Minor: int64(tc.Properties["Minor"].(float64)), Patch: int64(tc.Properties["Patch"].(float64)), PreReleaseTag: struct {
				Name      string
				Number    int64
				HasNumber bool
			}{tc.Properties["PreReleaseName"].(string), int64(tc.Properties["PreReleaseNumber"].(float64)), true}, BuildMetaData: struct {
				Branch, VersionSourceSha, Sha, ShortSha string
				VersionSourceDistance                   int64
				CommitDate                              time.Time
			}{tc.Properties["Branch"].(string), tc.Properties["VersionSourceSha"].(string), tc.Properties["Sha"].(string), tc.Properties["ShortSha"].(string), int64(tc.Properties["VersionSourceDistance"].(float64)), time.Date(2014, 3, 6, 23, 59, 59, 0, time.UTC)}}
		}
		lookup := func(name string) (string, bool) {
			v := tc.Environment[name]
			if v == nil {
				return "", false
			}
			return *v, true
		}
		got, err := FormatWith(tc.Template, source, lookup)
		if tc.Error {
			if err == nil {
				t.Fatal("invalid/missing placeholder accepted")
			}
			return
		}
		if err != nil || got != tc.Expected {
			t.Fatalf("want %q, got %q (%v)", tc.Expected, got, err)
		}
	})
}
func TestCSharpStringAndNumericFormatters(t *testing.T) {
	for _, spec := range []struct {
		file           string
		methods, cases int
	}{{"csharp-string-formatter.json", 6, 27}, {"csharp-numeric-formatter.json", 4, 10}} {
		runCases(t, originalCases(t, spec.file, spec.methods, spec.cases), func(t *testing.T, tc originalCase) {
			var got string
			var ok bool
			var err error
			switch tc.Operation {
			case "priority-string":
				if (StringFormatter{}).Priority() != tc.Priority {
					t.Fatal("string priority changed")
				}
				return
			case "priority-numeric":
				if (NumericFormatter{}).Priority() != tc.Priority {
					t.Fatal("numeric priority changed")
				}
				return
			case "string":
				got, ok, err = (StringFormatter{}).TryFormat(tc.Value, tc.Format)
			case "numeric":
				got, ok, err = (NumericFormatter{}).TryFormat(tc.Value, tc.Format)
			default:
				t.Fatal("unknown operation")
			}
			if err != nil || ok != tc.Success || got != tc.Expected {
				t.Fatalf("want %q, handled=%t; got %q, handled=%t (%v)", tc.Expected, tc.Success, got, ok, err)
			}
		})
	}
}
func TestCSharpSanitizerEdgeCases(t *testing.T) {
	runCases(t, originalCases(t, "csharp-edge-case.json", 6, 9), func(t *testing.T, tc originalCase) {
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
			t.Fatal("unknown sanitizer")
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
func TestTemplateFallbackInteractions(t *testing.T) {
	source := map[string]any{"Null": nil, "Empty": "", "Number": 42, "Word": "Hello", "Flag": true, "nested.key": "exact"}
	for _, tc := range []struct {
		name, template, want string
		fail                 bool
	}{
		{"present-null", "{Null}", "", false}, {"null-fallback", `{Null ?? "yes"}`, "yes", false},
		{"empty-stops", `{Empty ?? "no"}`, "", false}, {"missing-then-null", `{Missing ?? Null}`, "", true},
		{"late-malformed", `{Word ?? "unterminated}`, "", true}, {"format-error-fallback", `{Number:Q ?? "yes"}`, "yes", false},
		{"unknown-text-format", `{Word:unknown}`, "Hello", false}, {"repeated", "{Word}-{Word}", "Hello-Hello", false},
		{"nested-braces", "{{Word}} {}", "{Hello} {}", false}, {"dictionary-dot", "{nested.key}", "exact", false},
		{"bool", "{Flag}", "True", false}, {"whitespace-expression", "{ }", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FormatWith(tc.template, source, nil)
			if tc.fail {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("want %q got %q (%v)", tc.want, got, err)
			}
		})
	}
	reads := []string{}
	lookup := func(name string) (string, bool) {
		reads = append(reads, name)
		if name == "EMPTY" {
			return "", true
		}
		return "", false
	}
	if got, err := FormatWith(`{env:EMPTY ?? env:UNUSED}`, source, lookup); err != nil || got != "" || fmt.Sprint(reads) != "[EMPTY]" {
		t.Fatal("fallback must stop on present empty environment value", got, reads, err)
	}
	for _, v := range []any{nil, (*struct{})(nil), map[string]any(nil)} {
		if _, err := FormatWith("plain", v, nil); err == nil {
			t.Fatal("nil source accepted")
		}
	}
	object := struct {
		Nested  struct{ Name string }
		Other   struct{ Count int }
		private string
	}{}
	object.Nested.Name = "nested"
	object.Other.Count = 42
	if got, err := FormatWith("{Nested.Name}-{Count:0000}", object, nil); err != nil || got != "nested-0042" {
		t.Fatal("recursive source lookup", got, err)
	}
	type Embedded struct{ Value string }
	nilEmbedded := struct{ *Embedded }{}
	if got, err := FormatWith(`{Value ?? "fallback"}`, nilEmbedded, nil); err != nil || got != "fallback" {
		t.Fatal("nil promoted field must permit fallback", got, err)
	}
	if _, err := FormatWith("{private}", object, nil); err == nil {
		t.Fatal("unexported field exposed")
	}
}
func TestNumericAndStringFormatterBoundaries(t *testing.T) {
	for _, tc := range []struct{ value, format, want string }{{"-1", "x8", "ffffffff"}, {"-1", "b", "11111111111111111111111111111111"}, {"-42", "d5", "-00042"}, {"2147483647", "d", "2147483647"}, {"-2147483648", "d", "-2147483648"}, {"42", "0000", "0042"}, {"42", "10", "142"}, {"42", "111", "111"}, {"1.5", "e2", "1.50e+000"}, {"12345.67", "n1", "12,345.7"}, {"0.125", "p1", "12.5 %"}, {"-123.456", "c2", "(¤123.46)"}, {"1.25", "g", "1.25"}} {
		got, ok, err := (NumericFormatter{}).TryFormat(tc.value, tc.format)
		if err != nil || !ok || got != tc.want {
			t.Fatalf("%q:%s: want %q got %q handled=%t (%v)", tc.value, tc.format, tc.want, got, ok, err)
		}
	}
	if _, ok, err := (NumericFormatter{}).TryFormat("2147483648", "d"); err == nil || ok {
		t.Fatal("32-bit integer range not enforced")
	}
	for _, tc := range []struct {
		value  any
		format string
	}{{nil, "n"}, {123.4, "n"}, {"not numeric", "n"}} {
		got, ok, err := (NumericFormatter{}).TryFormat(tc.value, tc.format)
		if err != nil || ok || got != "" {
			t.Fatal("non-string or nonnumeric handled", got, ok, err)
		}
	}
	if got, ok, err := (NumericFormatter{}).TryFormat("1", "f1000"); err != nil || !ok || got != "1."+strings.Repeat("0", 1000) {
		t.Fatal("valid large precision rejected", ok, err)
	}
	for _, f := range []string{"f1000000000", "Q", "d1000000000"} {
		if _, _, err := (NumericFormatter{}).TryFormat("1", f); err == nil {
			t.Fatal("invalid/unsupported precision accepted", f)
		}
	}
	for _, tc := range []struct{ value, format, want string }{{"HELLO_world-test", "c", "HelloWorldTest"}, {"hELLO WORLD", "t", "Hello World"}, {"a", "s", "A"}, {" \t", "invalid", ""}, {"foo😀bar", "c", "FooBar"}} {
		got, ok, err := (StringFormatter{}).TryFormat(tc.value, tc.format)
		if err != nil || !ok || got != tc.want {
			t.Fatal(tc, got, ok, err)
		}
	}
	if got, ok, err := (StringFormatter{}).TryFormat("abc", strings.ToUpper("u")); err != nil || ok || got != "" {
		t.Fatal("string specifiers must remain case-sensitive")
	}
}

package parity

import (
	"strings"
	"testing"
)

func TestCompareStrictJSON(t *testing.T) {
	tests := []struct{ name, expected, actual, field string }{
		{"ordering-and-whitespace", `{"SemVer":"1.2.3","Major":1}`, "{\n\"Major\":1,\"SemVer\":\"1.2.3\"}", ""},
		{"incorrect-version", `{"SemVer":"1.2.3"}`, `{"SemVer":"1.2.4"}`, "SemVer"},
		{"incorrect-count", `{"CommitCountSourceDistance":8}`, `{"CommitCountSourceDistance":9}`, "CommitCountSourceDistance"},
		{"incorrect-source", `{"SemVerSourceSha":"abc"}`, `{"SemVerSourceSha":"def"}`, "SemVerSourceSha"},
		{"number-versus-string", `{"Major":1}`, `{"Major":"1"}`, "Major"},
		{"null-versus-empty", `{"Label":null}`, `{"Label":""}`, "Label"},
		{"missing-versus-null", `{"SemVer":"1.0.0","Label":null}`, `{"SemVer":"1.0.0"}`, "Label"},
		{"unexpected-key", `{"SemVer":"1.0.0"}`, `{"SemVer":"1.0.0","Label":null}`, "Label"},
		{"large-integer", `{"Count":9007199254740992}`, `{"Count":9007199254740993}`, "Count"},
		{"number-representation", `{"Count":1}`, `{"Count":1.0}`, "Count"},
		{"nested-array", `{"A":[{"n":1},null]}`, `{"A":[{"n":2},null]}`, "A"},
		{"escaped-equivalent", `{"Label":"caf\u00e9"}`, `{"Label":"café"}`, ""},
		{"surrogate-pair", `{"Label":"\ud83d\ude00"}`, `{"Label":"😀"}`, ""},
		{"literal-escape", `{"Label":"\\ud800"}`, `{"Label":"\\ud800"}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Compare([]byte(tt.expected), []byte(tt.actual))
			if err != nil {
				t.Fatal(err)
			}
			if tt.field == "" {
				if len(d) != 0 {
					t.Fatalf("unexpected differences: %+v", d)
				}
			} else if len(d) != 1 || d[0].Field != tt.field {
				t.Fatalf("got %+v, want one difference for %s", d, tt.field)
			}
		})
	}
}

func TestObjectRejectsInvalidJSON(t *testing.T) {
	for _, tt := range []struct{ name, input string }{
		{"duplicate-root", `{"Major":1,"Major":2}`}, {"duplicate-nested", `{"A":[{"n":1,"n":2}]}`},
		{"trailing-value", `{"A":1} {"A":2}`}, {"malformed", `{"A":`}, {"array-root", `[1]`}, {"null-root", `null`}, {"empty-object", `{}`},
		{"unterminated-array", `{"A":[1}`}, {"trailing-garbage", `{"A":1} junk`},
		{"invalid-utf8", "{\"Label\":\"\xff\"}"},
		{"unpaired-high-surrogate", `{"Label":"\ud800"}`},
		{"unpaired-low-surrogate", `{"Label":"\udfff"}`},
		{"invalid-surrogate-pair", `{"Label":"\ud800\u1234"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Object([]byte(tt.input)); err == nil {
				t.Fatal("accepted invalid version output")
			}
			if _, err := Compare([]byte(tt.input), []byte(`{"A":1}`)); err == nil {
				t.Fatal("accepted invalid expected JSON")
			}
			if _, err := Compare([]byte(`{"A":1}`), []byte(tt.input)); err == nil {
				t.Fatal("accepted invalid actual JSON")
			}
		})
	}
}

func TestCompareReportsDeterministicFieldOrder(t *testing.T) {
	d, err := Compare([]byte(`{"Z":1,"A":2}`), []byte(`{"Z":3,"B":4}`))
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, v := range d {
		keys = append(keys, v.Field)
	}
	if strings.Join(keys, ",") != "A,B,Z" {
		t.Fatalf("order %v", keys)
	}
	if d[0].Reason != "missing key" || d[1].Reason != "unexpected key" {
		t.Fatalf("presence distinction lost: %+v", d)
	}
}

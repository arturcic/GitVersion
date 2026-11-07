package gitrepo

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestCSharpReferenceName(t *testing.T) {
	var corpus struct {
		Revision string
		Methods  map[string][]struct {
			ID        string
			Arguments []json.RawMessage
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-reference-names.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Revision != "14d222235b7dabb7033473528a76fd18478a9217" || len(corpus.Methods) != 6 {
		t.Fatal("original source inventory differs")
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.ID, func(t *testing.T) {
					arg := func(i int) string {
						var value string
						if err := json.Unmarshal(tc.Arguments[i], &value); err != nil {
							t.Fatal(err)
						}
						return value
					}
					switch method {
					case "FromTagName_PreservesLiteralName":
						input := arg(0)
						got, err := ReferenceFromTagName(&input)
						if err != nil {
							t.Fatal(err)
						}
						if got.Canonical != arg(1) || got.Friendly != input || !got.IsTag || got.IsLocalBranch {
							t.Fatalf("literal tag reference differs: %+v", got)
						}
					case "FromTagName_RejectsNull", "FromTagName_RejectsEmpty":
						var input *string
						kind := "null"
						if method == "FromTagName_RejectsEmpty" {
							value := ""
							input = &value
							kind = "empty"
						}
						_, err := ReferenceFromTagName(input)
						var typed *ReferenceArgumentError
						if !errors.As(err, &typed) || typed.ParamName != "tagName" || typed.Kind != kind {
							t.Fatalf("original exception/parameter contract differs: %v", err)
						}
					case "EquivalentTo_UsesOriginStrippedName":
						var expected bool
						if err := json.Unmarshal(tc.Arguments[2], &expected); err != nil {
							t.Fatal(err)
						}
						name := arg(1)
						if got := NewReferenceName(arg(0)).EquivalentTo(&name); got != expected {
							t.Fatalf("origin equivalence: want %v, got %v", expected, got)
						}
					case "WithoutOrigin_ReturnsExpectedName":
						if got := NewReferenceName(arg(0)).WithoutOrigin; got != arg(1) {
							t.Fatalf("origin-stripped name: want %q, got %q", arg(1), got)
						}
					case "WithoutRemote_ReturnsExpectedName":
						if got := NewReferenceName(arg(0)).WithoutRemote; got != arg(1) {
							t.Fatalf("remote-stripped name: want %q, got %q", arg(1), got)
						}
					default:
						t.Fatal("unhandled original method", method)
					}
				})
			}
		})
	}
}

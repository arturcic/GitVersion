package calculate

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCSharpMergeMessages(t *testing.T) {
	var corpus struct {
		Methods map[string][]struct {
			ID                                       string
			Message, Format, Branch, Target, Version *string
			PR                                       *int
			Custom                                   []MergeFormat
			Error                                    bool
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-merge-messages.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Methods) != 15 {
		t.Fatal("source method count changed")
	}
	for name, cases := range corpus.Methods {
		t.Run(name, func(t *testing.T) {
			for _, c := range cases {
				t.Run(c.ID, func(t *testing.T) {
					got, err := ParseMergeMessage(c.Message, c.Custom)
					if (err != nil) != c.Error {
						t.Fatal("error", err)
					}
					if c.Error {
						return
					}
					var v *string
					if got.Version != nil {
						x := got.Version.String()
						v = &x
					}
					for _, pair := range []struct {
						name      string
						got, want any
					}{{"format", got.FormatName, c.Format}, {"branch", got.MergedBranch, c.Branch}, {"target", got.TargetBranch, c.Target}, {"version", v, c.Version}, {"pr", got.PullRequestNumber, c.PR}, {"pull", got.IsMergedPullRequest(), c.PR != nil}} {
						if !reflect.DeepEqual(pair.got, pair.want) {
							t.Errorf("%s: got %v want %v", pair.name, pair.got, pair.want)
						}
					}
				})
			}
		})
	}
}

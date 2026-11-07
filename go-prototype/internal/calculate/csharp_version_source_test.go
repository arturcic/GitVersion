package calculate

import (
	"encoding/json"
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/fixtures"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCSharpVersionSources(t *testing.T) {
	var corpus struct {
		Methods map[string][]struct {
			ID, Operation, Config string
			Recipe                fixtures.Recipe
			Expected              map[string]any
			Special               map[string]any
		}
	}
	raw, err := os.ReadFile("../../testdata/csharp-version-sources.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Methods) != 19 {
		t.Fatal("source method inventory changed")
	}
	parse := func(s string) semver.Version {
		t.Helper()
		v, err := semver.Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for method, cases := range corpus.Methods {
		t.Run(method, func(t *testing.T) {
			for _, c := range cases {
				t.Run(c.ID, func(t *testing.T) {
					got := map[string]any{}
					if c.Operation == "operator" {
						var b mlBase
						if method == "MainlineFoldKeepsExternalOriginWhenOperatorChangesCountAnchor" {
							b = mlBase{value: parse("5.0.0"), anchor: "countCommit", op: &mlOperator{inc: semver.Patch, labelNull: true, anchor: "countCommit"}}
							if err := b.apply(mlOperator{inc: semver.Minor, force: true, labelNull: true, anchor: "nextCountCommit"}); err != nil {
								t.Fatal(err)
							}
						} else {
							major := int64(c.Special["alternativeMajor"].(float64))
							b = mlBase{value: parse("1.0.0"), origin: "originalCommit", anchor: "originalCommit", op: &mlOperator{inc: semver.Patch, labelNull: true, alt: &mlOrigin{version: semver.Version{Major: major}, sha: "alternativeCommit"}}}
						}
						v, err := b.version()
						if err != nil {
							t.Fatal(err)
						}
						source, sha, inc := b.semantic()
						var sourceSHA any
						if sha != "" {
							sourceSHA = sha
						}
						got = map[string]any{"version": v.String(), "anchor": b.anchor, "source_version": source.String(), "source_sha": sourceSHA, "increment": inc.String()}
					} else {
						cps, err := fixtures.Generate([]fixtures.Recipe{c.Recipe}, filepath.Join(t.TempDir(), "fixtures"))
						if err != nil {
							t.Fatal(err)
						}
						aliases := map[string]string{}
						var path string
						for _, cp := range cps {
							id := cp.ID[strings.LastIndex(cp.ID, "--")+2:]
							aliases[id] = cp.Head
							if id == "head" {
								path = cp.Path
							}
						}
						resolve := func(v any) any {
							s, ok := v.(string)
							if !ok || !strings.HasPrefix(s, "@") {
								return v
							}
							sha, ok := aliases[strings.ToLower(s[1:])]
							if !ok {
								t.Fatal("unknown original SHA variable", s)
							}
							return sha
						}
						cfgText := c.Config

						// JSON SHA placeholders retain the original variable spelling.
						var document map[string]any
						if err := json.Unmarshal([]byte(cfgText), &document); err != nil {
							t.Fatal(err)
						}
						if calc, ok := document["calculation"].(map[string]any); ok {
							if ignore, ok := calc["ignore"].(map[string]any); ok {
								if shas, ok := ignore["sha"].([]any); ok {
									for i, v := range shas {
										shas[i] = resolve(v)
									}
								}
							}
						}
						cfgBytes, _ := json.Marshal(document)
						cfg, err := config.Parse(cfgBytes)
						if err != nil {
							t.Fatal(err)
						}
						before, err := fixtures.Digest(path)
						if err != nil {
							t.Fatal(err)
						}
						var trace Trace
						if c.Operation == "injected-strategy" {
							got, trace, err = calculateWithStrategy(path, cfg, func() ([]candidate, error) {
								stable := parse("1.0.0")
								pre := parse("4.0.0-alpha.1")
								external := parse("5.0.0")
								return []candidate{{base: external, semantic: external, version: external}, {base: pre, semantic: pre, version: pre, sha: aliases["prereleasesha"], semanticSHA: aliases["prereleasesha"]}, {base: stable, semantic: stable, version: stable, sha: aliases["stablesha"], semanticSHA: aliases["stablesha"]}}, nil
							})
						} else {
							got, trace, err = CalculateWithConfig(path, cfg)
						}
						if err != nil {
							t.Fatal(err)
						}
						after, err := fixtures.Digest(path)
						if err != nil || before != after {
							t.Fatal("calculation mutated original setup", err)
						}
						// C# model-null SHA maps to Go zero-value SHA; legacy JSON has a separate empty-string contract.
						if trace.CountSHA == "" {
							got["VersionSourceSha"] = nil
						} else {
							got["VersionSourceSha"] = trace.CountSHA
						}
						for k, v := range c.Expected {
							c.Expected[k] = resolve(v)
						}
					}
					normalized, _ := json.Marshal(got)
					var values map[string]any
					_ = json.Unmarshal(normalized, &values)
					for key, want := range c.Expected {
						if values[key] != want {
							t.Errorf("original assertion %s: got %#v want %#v", key, values[key], want)
						}
					}
				})
			}
		})
	}
}

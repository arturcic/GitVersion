package semver

import "testing"

func TestSemanticVersionParsingAndFormatting(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{{"stable", "12.34.56", "12.34.56"}, {"label", "1.0.0-beta", "1.0.0-beta"}, {"numeric", "1.0.0-12", "1.0.0-12"}, {"numbered", "2.3.4-rc.12", "2.3.4-rc.12"}, {"trailing-number", "1.2.3-rc12", "1.2.3-rc.12"}, {"dash-number", "1.2.3-rc-12", "1.2.3-rc-12"}} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := Parse(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if v.String() != tc.want {
				t.Fatalf("got %s want %s", v, tc.want)
			}
		})
	}
	for _, s := range []string{"1.2", "01.2.3", "1.2.3-01", "1.2.3-a..b", "2147483648.0.0", "1.0.0-a.2147483648"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("unsupported or invalid version accepted: %q", s)
		}
	}
	for _, s := range []string{"v2.5.7", "V2.5.7"} {
		v, ok, err := ParseTag(s)
		if err != nil || !ok || v.String() != "2.5.7" {
			t.Fatalf("tag %q: %v %t %v", s, v, ok, err)
		}
	}
	for _, s := range []string{"nightly", "vV2.5.7", "01.2.3"} {
		_, ok, err := ParseTag(s)
		if ok || err != nil {
			t.Fatalf("non-version tag %q", s)
		}
	}
	if v, ok, err := ParseTag("1.2.3+build"); !ok || err != nil || v.Metadata.Other == nil || *v.Metadata.Other != "build" {
		t.Fatal("tag metadata was not retained", v, ok, err)
	}
}
func TestSemanticVersionIncrementAndOrder(t *testing.T) {
	for _, tc := range []struct {
		name, input, label, want string
		inc                      Increment
		force                    bool
	}{
		{"patch", "1.2.3", "foo", "1.2.4-foo.1", Patch, false}, {"minor", "1.2.3", "", "1.3.0-1", Minor, false}, {"major", "1.2.3", "foo", "2.0.0-foo.1", Major, false},
		{"prerelease", "2.0.0-foo.7", "foo", "2.0.0-foo.8", Major, false}, {"force", "2.0.0-foo.7", "foo", "3.0.0-foo.1", Major, true}, {"label-change", "2.0.0-foo.7", "bar", "2.0.0-bar.1", Patch, false},
		{"configured", "3.0.0", "", "3.0.0-1", None, false}, {"bare-label", "2.0.0-foo", "foo", "2.0.0-foo", Patch, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := Parse(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			got, err := v.Bump(tc.inc, tc.label, tc.force)
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
			if v.String() != tc.input {
				t.Fatal("increment mutated input")
			}
		})
	}
	ordered := []string{"1.0.0-alpha", "1.0.0-alpha.2", "1.0.0-alpha.11", "1.0.0-beta.1", "1.0.0", "2.0.0-1"}
	for i := 1; i < len(ordered); i++ {
		a, _ := Parse(ordered[i-1])
		b, _ := Parse(ordered[i])
		if a.Compare(b) >= 0 || b.Compare(a) <= 0 {
			t.Errorf("wrong ordering %s / %s", a, b)
		}
	}
	a, _ := Parse("1.0.0-FOO.1")
	if !a.Matches("foo") || a.Matches("bar") {
		t.Fatal("label matching changed")
	}
	b, _ := Parse("1.0.0-foo.1")
	if a.Compare(b) != 0 {
		t.Fatal("case-insensitive prerelease comparison changed")
	}
	v, _ := Parse("2147483647.0.0")
	if _, err := v.Bump(Major, "", false); err == nil {
		t.Fatal("overflow accepted")
	}
	if _, err := ParseIncrement("Inherit"); err == nil {
		t.Fatal("unresolved increment accepted")
	}
}

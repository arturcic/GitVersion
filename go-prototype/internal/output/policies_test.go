package output

import (
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"math"
	"strings"
	"testing"
)

func TestOutputExplicitWeightsAndDeploymentMetadata(t *testing.T) {
	zero := int64(0)
	v, err := semver.Parse("1.2.3-preview.4")
	if err != nil {
		t.Fatal(err)
	}
	r := Result{Version: v, SHA: strings.Repeat("a", 40), Branch: "main", Main: true, Mode: config.Manual, Weight: &zero, TagWeight: &zero, Distance: 2}
	got, err := Variables(r)
	if err != nil {
		t.Fatal(err)
	}
	if got["WeightedPreReleaseNumber"] != int64(4) || got["FullSemVer"] != "1.2.3-preview.4+2" || got["BuildMetaData"] != int64(2) {
		t.Fatalf("zero/custom-mode contract: %+v", got)
	}
	r.Version.Label = ""
	r.Version.Number = nil
	r.Mode = config.Deployment
	got, err = Variables(r)
	if err != nil {
		t.Fatal(err)
	}
	if got["WeightedPreReleaseNumber"] != int64(0) || got["PreReleaseNumber"] != nil || got["BuildMetaData"] != nil || got["FullSemVer"] != "1.2.3" {
		t.Fatalf("stable deployment: %+v", got)
	}
	r.Version = v
	tooLarge := int64(math.MaxInt32)
	r.Weight = &tooLarge
	if _, err := Variables(r); err == nil {
		t.Fatal("weighted overflow accepted")
	}
	negative := int64(-1)
	r.Weight = &negative
	if _, err := Variables(r); err == nil {
		t.Fatal("negative weight accepted")
	}
	r.Weight = &zero
	r.TagWeight = &negative
	if _, err := Variables(r); err == nil {
		t.Fatal("negative tag weight accepted")
	}
}

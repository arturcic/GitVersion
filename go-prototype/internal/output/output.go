// Package output derives all 33 pinned v7 JSON variables from a calculation.
package output

import (
	"fmt"
	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
	"math"
	"strconv"
	"time"
)

type Result struct {
	Version           semver.Version
	SemanticSource    semver.Version
	SemanticSHA       string
	Increment         semver.Increment
	LegacySource      semver.Version
	CountSHA          string
	Distance          int64
	Branch, SHA       string
	CommitTime        int64
	Main, Tagged      bool
	Mode              config.Mode
	Weight, TagWeight *int64
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func prefix(s string) string {
	if s == "" {
		return ""
	}
	return "-" + s
}
func Variables(r Result) (map[string]any, error) {
	if len(r.SHA) != 40 || r.Distance < 0 || r.Distance > math.MaxInt32 {
		return nil, fmt.Errorf("unsupported output identity or count range")
	}
	v := r.Version
	var number any
	weight := int64(60000)
	if r.TagWeight != nil {
		weight = *r.TagWeight
	}
	if weight < 0 || weight > math.MaxInt32 {
		return nil, fmt.Errorf("unsupported tag weight range")
	}
	if r.Weight != nil && (*r.Weight < 0 || *r.Weight > math.MaxInt32) {
		return nil, fmt.Errorf("unsupported branch weight range")
	}
	if v.Number != nil {
		if *v.Number < 0 || *v.Number > math.MaxInt32-60000 {
			return nil, fmt.Errorf("unsupported prerelease output range")
		}
		number = *v.Number
		weight = 30000 + *v.Number
		if r.Main {
			weight = 55000 + *v.Number
		}
		if r.Weight != nil {
			weight = *r.Weight + *v.Number
		}
		if weight < 0 || weight > math.MaxInt32 {
			return nil, fmt.Errorf("unsupported weighted prerelease range")
		}
	}
	var build any
	fullMeta := "Branch." + config.Sanitize(r.Branch, true) + ".Sha." + r.SHA
	full := v.String()
	mode := r.Mode
	if mode == "" {
		mode = config.Manual
		if r.Main {
			mode = config.Delivery
		}
	}
	if mode != config.Manual && mode != config.Delivery && mode != config.Deployment {
		return nil, fmt.Errorf("unsupported output deployment mode")
	}
	if mode == config.Manual && !r.Tagged {
		build = r.Distance
		n := strconv.FormatInt(r.Distance, 10)
		full += "+" + n
		fullMeta = n + "." + fullMeta
	}
	pre := v.PreRelease()
	assembly := v.Core() + ".0"
	return map[string]any{
		"AssemblySemFileVer": assembly, "AssemblySemVer": assembly, "BranchName": r.Branch, "BuildMetaData": build,
		"CommitCountSourceDistance": r.Distance, "CommitCountSourceSha": nullable(r.CountSHA), "CommitDate": time.Unix(r.CommitTime, 0).UTC().Format("2006-01-02"), "CustomVersion": "",
		"EscapedBranchName": config.Sanitize(r.Branch, false), "FullBuildMetaData": fullMeta, "FullSemVer": full, "InformationalVersion": v.String() + "+" + fullMeta,
		"Major": v.Major, "MajorMinorPatch": v.Core(), "Minor": v.Minor, "Patch": v.Patch,
		"PreReleaseLabelName": v.Label, "PreReleaseLabelNameWithDash": prefix(v.Label), "PreReleaseNumber": number, "PreReleaseLabel": pre, "PreReleaseLabelWithDash": prefix(pre), "SemVer": v.String(),
		"SemVerSourceIncrement": r.Increment.String(), "SemVerSourceSemVer": r.SemanticSource.String(), "SemVerSourceSha": nullable(r.SemanticSHA), "Sha": r.SHA, "ShortSha": r.SHA[:7], "UncommittedChanges": 0,
		"VersionSourceDistance": r.Distance, "VersionSourceIncrement": "None", "VersionSourceSemVer": r.LegacySource.String(), "VersionSourceSha": r.CountSHA, "WeightedPreReleaseNumber": weight,
	}, nil
}

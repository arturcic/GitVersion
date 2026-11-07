// Package output derives all 33 pinned v7 JSON variables from a calculation.
package output

import (
	"fmt"
	"maps"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/GitTools/GitVersion/go-prototype/internal/config"
	"github.com/GitTools/GitVersion/go-prototype/internal/formatting"
	"github.com/GitTools/GitVersion/go-prototype/internal/semver"
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
	Output            config.OutputSettings
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
	tagWeight := int64(60000)
	if r.TagWeight != nil {
		tagWeight = *r.TagWeight
	}
	if tagWeight < 0 || tagWeight > math.MaxInt32 {
		return nil, fmt.Errorf("unsupported tag weight range")
	}
	if r.Weight != nil && (*r.Weight < 0 || *r.Weight > math.MaxInt32) {
		return nil, fmt.Errorf("unsupported branch weight range")
	}
	branchWeight := int64(30000)
	if r.Main {
		branchWeight = 55000
	}
	if r.Weight != nil {
		branchWeight = *r.Weight
	}
	if v.Number != nil {
		if *v.Number < 0 || *v.Number > math.MaxInt32-60000 {
			return nil, fmt.Errorf("unsupported prerelease output range")
		}
		if branchWeight+*v.Number > math.MaxInt32 {
			return nil, fmt.Errorf("unsupported weighted prerelease range")
		}
	}
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
	var build *int64
	if mode == config.Manual && !r.Tagged {
		build = &r.Distance
	}
	date := time.Unix(r.CommitTime, 0).UTC()
	return FromSemanticVersion(VariableInput{
		Version: v, SemanticSource: &r.SemanticSource, LegacySource: &r.LegacySource,
		Increment: r.Increment, SemanticSHA: r.SemanticSHA, CountSHA: r.CountSHA,
		Branch: r.Branch, SHA: r.SHA, ShortSHA: r.SHA[:7], Date: &date,
		Distance: r.Distance, BranchWeight: branchWeight, TagWeight: tagWeight, Build: build,
	}, r.Output, os.LookupEnv)
}

// VariableInput separates semantic metadata from repository input validation.
type VariableInput struct {
	Version                           semver.Version
	SemanticSource, LegacySource      *semver.Version
	Increment, LegacyIncrement        semver.Increment
	SemanticSHA, CountSHA             string
	Branch, SHA, ShortSHA             string
	Date                              *time.Time
	Distance, BranchWeight, TagWeight int64
	Build                             *int64
}

// FromSemanticVersion projects semantic values through the configured output formats.
func FromSemanticVersion(input VariableInput, settings config.OutputSettings, environment formatting.EnvironmentLookup) (map[string]any, error) {
	v := input.Version
	var number, build, semanticSource, commitDate any
	weight := input.TagWeight
	if v.Number != nil {
		number = *v.Number
		weight = input.BranchWeight + *v.Number
	}
	if input.SemanticSource != nil {
		semanticSource = input.SemanticSource.String()
	}
	legacySource := ""
	if input.LegacySource != nil {
		legacySource = input.LegacySource.String()
	}
	if input.Date != nil {
		date := input.Date.UTC()
		commitDate = date.Format("2006-01-02")
		if settings.CommitDateFormat != nil {
			text, _, err := (formatting.DateFormatter{}).TryFormat(date, *settings.CommitDateFormat)
			if err != nil {
				return nil, fmt.Errorf("unable to format CommitDate: %w", err)
			}
			commitDate = text
		}
	}
	var metadata []string
	full := v.String()
	if input.Build != nil {
		build = *input.Build
		n := strconv.FormatInt(*input.Build, 10)
		full += "+" + n
		metadata = append(metadata, n)
	}
	if input.Branch != "" {
		metadata = append(metadata, "Branch", config.Sanitize(input.Branch, true))
	}
	if input.SHA != "" {
		metadata = append(metadata, "Sha", input.SHA)
	}
	fullMeta := strings.Join(metadata, ".")
	informational := v.String()
	if fullMeta != "" {
		informational += "+" + fullMeta
	}
	pre := v.PreRelease()
	assemblyVersion := assembly(v, settings.AssemblyScheme)
	assemblyFileVersion := assembly(v, settings.AssemblyFileScheme)
	values := map[string]any{
		"AssemblySemFileVer": assemblyFileVersion, "AssemblySemVer": assemblyVersion, "BranchName": input.Branch, "BuildMetaData": build,
		"CommitCountSourceSha": nullable(input.CountSHA), "CommitDate": commitDate, "CustomVersion": "",
		"EscapedBranchName": config.Sanitize(input.Branch, false), "FullBuildMetaData": fullMeta, "FullSemVer": full, "InformationalVersion": informational,
		"Major": v.Major, "MajorMinorPatch": v.Core(), "Minor": v.Minor, "Patch": v.Patch,
		"PreReleaseLabelName": v.Label, "PreReleaseLabelNameWithDash": prefix(v.Label), "PreReleaseNumber": number, "PreReleaseLabel": pre, "PreReleaseLabelWithDash": prefix(pre), "SemVer": v.String(),
		"SemVerSourceIncrement": input.Increment.String(), "SemVerSourceSemVer": semanticSource, "SemVerSourceSha": nullable(input.SemanticSHA), "Sha": input.SHA, "ShortSha": input.ShortSHA, "UncommittedChanges": 0,
		"VersionSourceIncrement": input.LegacyIncrement.String(), "VersionSourceSemVer": legacySource, "VersionSourceSha": input.CountSHA, "WeightedPreReleaseNumber": weight,
	}
	setSourceDistance(values, input.Distance)
	formats := []struct{ key, property, template string }{
		{"InformationalVersion", "AssemblyInformationalVersion", settings.InformationalFormat},
		{"CustomVersion", "CustomVersionFormat", settings.CustomFormat},
		{"AssemblySemFileVer", "AssemblyFileVersioningFormat", settings.AssemblyFileFormat},
		{"AssemblySemVer", "AssemblyVersioningFormat", settings.AssemblyFormat},
	}
	if settings.InformationalFormat == "" && settings.CustomFormat == "" && settings.AssemblyFileFormat == "" && settings.AssemblyFormat == "" {
		return values, nil
	}
	source := maps.Clone(values)
	delete(source, "CustomVersion")
	source["AssemblySemVer"] = nullable(assemblyVersion)
	source["AssemblyFileSemVer"] = nullable(assemblyFileVersion)
	if number == nil {
		source["PreReleaseNumber"] = ""
	}
	if build == nil {
		source["BuildMetaData"] = ""
	}
	source["VersionSourceSemVer"] = nullable(legacySource)
	delete(source, "AssemblySemFileVer")
	for _, item := range formats {
		if item.template == "" {
			continue
		}
		text, err := formatting.FormatWith(item.template, source, environment)
		if err != nil {
			return nil, fmt.Errorf("Unable to format %s.  Check your format string: %w", item.property, err)
		}
		values[item.key] = sanitizeFormat(text)
	}
	return values, nil
}

func setSourceDistance(values map[string]any, distance int64) {
	values["VersionSourceDistance"] = distance
	values["CommitCountSourceDistance"] = distance
}

func assembly(version semver.Version, scheme string) string {
	switch scheme {
	case "None":
		return ""
	case "Major":
		return fmt.Sprintf("%d.0.0.0", version.Major)
	case "MajorMinor":
		return fmt.Sprintf("%d.%d.0.0", version.Major, version.Minor)
	case "MajorMinorPatchTag":
		if version.Number != nil {
			return fmt.Sprintf("%s.%d", version.Core(), *version.Number)
		}
	}
	return version.Core() + ".0"
}

func sanitizeFormat(value string) string {
	var result strings.Builder
	for _, unit := range utf16.Encode([]rune(value)) {
		c := rune(unit)
		switch {
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9', c == '-', c == '.', c == '+':
			result.WriteRune(c)
		default:
			result.WriteByte('-')
		}
	}
	return result.String()
}

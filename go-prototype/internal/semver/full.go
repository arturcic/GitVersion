package semver

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
)

// BuildMetadata retains the nullable fields in a parsed C# semantic version.
// Calculation-derived output metadata remains the responsibility of output.
type BuildMetadata struct {
	CommitsSinceTag    *int64
	Branch, SHA, Other *string
}

type ParseMode string

const (
	Strict ParseMode = "Strict"
	Loose  ParseMode = "Loose"
)

// ParseDetailed implements the version model's parsing contract, including
// metadata and 64-bit components. Parse remains the calculator's declared input
// boundary until its output and arithmetic support these extended values.
func ParseDetailed(value string, tagPrefix *string, mode ParseMode) (Version, error) {
	if mode != Strict && mode != Loose {
		return Version{}, fmt.Errorf("unknown semantic version format %q", mode)
	}
	prefix := ""
	if tagPrefix != nil {
		prefix = *tagPrefix
	}
	p, err := patterns.Compile("^("+prefix+")(?<version>.*)$", patterns.IgnoreCase)
	if err != nil {
		return Version{}, err
	}
	match, err := p.Find(value)
	if err != nil || match == nil {
		return Version{}, fmt.Errorf("invalid version prefix: %q: %v", value, err)
	}
	value, ok := match.Group("version")
	if !ok {
		return Version{}, fmt.Errorf("missing version capture")
	}
	key := "SemanticVersion.ParseStrictRegex"
	names := []string{"major", "minor", "patch", "prerelease", "buildmetadata"}
	if mode == Loose {
		key = "SemanticVersion.ParseLooseRegex"
		names = []string{"Major", "Minor", "Patch", "Tag", "BuildMetaData"}
	}
	p, err = patterns.Builtin(key)
	if err != nil {
		return Version{}, err
	}
	match, err = p.Find(value)
	if err != nil {
		return Version{}, err
	}
	if match == nil {
		return Version{}, fmt.Errorf("invalid semantic version %q", value)
	}
	v := Version{}
	for i, field := range []*int64{&v.Major, &v.Minor, &v.Patch} {
		if text, ok := match.Group(names[i]); ok {
			n, err := strconv.ParseInt(text, 10, 64)
			if err != nil {
				return Version{}, err
			}
			*field = n
		}
	}
	label, _ := match.Group(names[3])
	v.Label, v.Number, err = parseFullPrerelease(label)
	if err != nil {
		return Version{}, err
	}
	metadata, _ := match.Group(names[4])
	v.Metadata, err = parseMetadata(metadata)
	if err != nil {
		return Version{}, err
	}
	if fourth, ok := match.Group("FourthPart"); ok && v.Metadata.CommitsSinceTag == nil {
		n, err := strconv.ParseInt(fourth, 10, 32)
		if err != nil {
			return Version{}, err
		}
		v.Metadata.CommitsSinceTag = &n
	}
	return v, nil
}
func parseFullPrerelease(value string) (string, *int64, error) {
	if value == "" {
		return "", nil, nil
	}
	p, err := patterns.Builtin("SemanticVersion.ParsePreReleaseTagRegex")
	if err != nil {
		return "", nil, err
	}
	m, err := p.Find(value)
	if err != nil {
		return "", nil, err
	}
	if m == nil {
		return "", nil, fmt.Errorf("invalid prerelease %q", value)
	}
	name, _ := m.Group("name")
	var number *int64
	if text, ok := m.Group("number"); ok {
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return "", nil, err
		}
		number = &n
	}
	if strings.HasSuffix(name, "-") {
		return value, nil, nil
	}
	return name, number, nil
}
func parseMetadata(value string) (BuildMetadata, error) {
	out := BuildMetadata{}
	if value == "" {
		return out, nil
	}
	p, err := patterns.Builtin("SemanticVersion.ParseBuildMetaDataRegex")
	if err != nil {
		return out, err
	}
	m, err := p.Find(value)
	if err != nil {
		return out, err
	}
	if m == nil {
		return out, fmt.Errorf("invalid metadata")
	}
	if text, ok := m.Group("BuildNumber"); ok {
		if n, err := strconv.ParseInt(text, 10, 64); err == nil {
			out.CommitsSinceTag = &n
		}
	}
	if text, ok := m.Group("BranchName"); ok {
		out.Branch = &text
	}
	if text, ok := m.Group("Sha"); ok {
		out.SHA = &text
	}
	if text, ok := m.Group("Other"); ok && text != "" {
		text = strings.TrimLeft(text, ".")
		out.Other = &text
	}
	return out, nil
}
func metadataPart(s string) string {
	var out strings.Builder
	for _, r := range utf16.Encode([]rune(s)) {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '.' {
			out.WriteByte(byte(r))
		} else {
			out.WriteByte('-')
		}
	}
	return out.String()
}
func (m BuildMetadata) Format(format string) (string, error) {
	parts := []string{}
	if m.CommitsSinceTag != nil {
		parts = append(parts, strconv.FormatInt(*m.CommitsSinceTag, 10))
	}
	switch strings.ToLower(format) {
	case "", "b":
	case "s", "f":
		if strings.EqualFold(format, "f") && m.Branch != nil && *m.Branch != "" {
			parts = append(parts, "Branch", metadataPart(*m.Branch))
		}
		if m.SHA != nil && *m.SHA != "" {
			parts = append(parts, "Sha", *m.SHA)
		}
		if strings.EqualFold(format, "f") && m.Other != nil && *m.Other != "" {
			parts = append(parts, metadataPart(*m.Other))
		}
	default:
		return "", fmt.Errorf("unknown metadata format %q", format)
	}
	return strings.TrimLeft(strings.Join(parts, "."), "."), nil
}
func (v Version) Format(format string) (string, error) {
	switch strings.ToLower(format) {
	case "", "s", "t":
		return v.String(), nil
	case "j":
		return v.Core(), nil
	case "f", "i":
		metadataFormat := "b"
		if strings.EqualFold(format, "i") {
			metadataFormat = "f"
		}
		metadata, err := v.Metadata.Format(metadataFormat)
		if err != nil {
			return "", err
		}
		if metadata != "" {
			return v.String() + "+" + metadata, nil
		}
		return v.String(), nil
	default:
		return "", fmt.Errorf("unknown version format %q", format)
	}
}

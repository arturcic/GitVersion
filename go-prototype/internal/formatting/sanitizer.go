package formatting

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
)

type InputSanitizer struct{}

func (InputSanitizer) SanitizeFormat(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("Format string cannot be empty.")
	}
	units := utf16.Encode([]rune(value))
	if len(units) > 50 {
		return "", fmt.Errorf("Format string too long: '%s...'", string(utf16.Decode(units[:20])))
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\t' {
			return "", fmt.Errorf("Format string contains invalid control characters")
		}
	}
	return value, nil
}

var envNamePattern = mustPattern(`^[A-Za-z0-9_:\-\.]+$`)
var memberNamePattern = mustPattern(`^[A-Za-z0-9_\.]+$`)

func mustPattern(value string) *patterns.Pattern {
	p, err := patterns.Compile(value, patterns.IgnoreCase)
	if err != nil {
		panic(err)
	}
	return p
}

// Pointer inputs retain C# null-name validation rather than silently converting
// null to a valid empty value. No environment access or reflection occurs here.
func (InputSanitizer) SanitizeEnvVarName(name *string) (string, error) {
	return sanitizeName(name, 200, "Environment variable name cannot be null or empty.", "Environment variable name", envNamePattern)
}
func (InputSanitizer) SanitizeMemberName(name *string) (string, error) {
	return sanitizeName(name, 100, "Member name cannot be empty.", "Member name", memberNamePattern)
}
func sanitizeName(value *string, limit int, empty, kind string, p *patterns.Pattern) (string, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "", fmt.Errorf("%s", empty)
	}
	units := utf16.Encode([]rune(*value))
	if len(units) > limit {
		return "", fmt.Errorf("%s too long: '%s...'", kind, string(utf16.Decode(units[:20])))
	}
	m, err := p.Find(*value)
	if err != nil {
		return "", err
	}
	if m == nil {
		return "", fmt.Errorf("%s contains disallowed characters: '%s'", kind, *value)
	}
	return *value, nil
}

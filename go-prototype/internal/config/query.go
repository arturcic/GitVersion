package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf16"
)

const RequireV7Query = "Configuration property queries require v7 configuration. Set GITVERSION_CONFIGURATION_VERSION=v7."

// SerializeProperty exposes fixed scalar properties, including unset values.
// The public schema distinguishes a missing value from an unknown property.
func (c ResolvedConfiguration) SerializeProperty(path, version string) ([]byte, error) {
	if version != "v7" {
		return nil, fmt.Errorf("%s", RequireV7Query)
	}
	parts := strings.Split(path, ".")
	for _, part := range parts {
		if part == "" || strings.ContainsFunc(part, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-')
		}) {
			return nil, fmt.Errorf("Invalid configuration property path '%s'. Use dot-separated public configuration names.", path)
		}
	}
	unknown := func() ([]byte, error) {
		return nil, fmt.Errorf("Unknown configuration property '%s'. Use the public v7 configuration path.", path)
	}
	unsupported := func() ([]byte, error) {
		return nil, fmt.Errorf("Configuration property '%s' is not supported: only scalar properties of fixed objects can be queried; maps and collections are not supported.", path)
	}
	switch parts[0] {
	case "calculation", "output":
		if len(parts) == 1 || parts[1] == "branches" {
			return unsupported()
		}
		if parts[1] == "workflow" || containsProperty(outputProperties, parts[1]) != (parts[0] == "output") {
			return unknown()
		}
		parts = parts[1:]
	case "workflow":
	default:
		return unknown()
	}
	node := c.flat
	rules := rootField
	for i, part := range parts {
		rule := rules(part)
		switch rule.kind {
		case "":
			return unknown()
		case "list", "branches", "merge-message-formats", "strategies":
			return unsupported()
		case "ignore", "prevent-increment":
			if i == len(parts)-1 {
				return unsupported()
			}
			node = nodeField(node, part)
			rules = ignoreField
			if rule.kind == "prevent-increment" {
				rules = preventField
			}
		default:
			if i != len(parts)-1 {
				return unknown()
			}
			value := nodeField(node, part)
			if value == nil || value.Tag == "!!null" {
				return []byte("null"), nil
			}
			if rule.kind == "bool" || rule.kind == "int" {
				var scalar any
				if err := value.Decode(&scalar); err != nil {
					return nil, err
				}
				return json.Marshal(scalar)
			}
			return configurationJSONString(value.Value), nil
		}
	}
	return unknown()
}

func configurationJSONString(value string) []byte {
	var result strings.Builder
	result.WriteByte('"')
	for _, unit := range utf16.Encode([]rune(value)) {
		switch unit {
		case '\b':
			result.WriteString(`\b`)
		case '\f':
			result.WriteString(`\f`)
		case '\n':
			result.WriteString(`\n`)
		case '\r':
			result.WriteString(`\r`)
		case '\t':
			result.WriteString(`\t`)
		case '\\':
			result.WriteString(`\\`)
		default:
			if unit < 0x20 || unit > 0x7e || strings.ContainsRune("\"'&+<>`", rune(unit)) {
				fmt.Fprintf(&result, "\\u%04X", unit)
			} else {
				result.WriteByte(byte(unit))
			}
		}
	}
	result.WriteByte('"')
	return []byte(result.String())
}

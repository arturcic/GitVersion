package formatting

import (
	"fmt"
	"strings"
	"unicode/utf16"
)

// JSONString matches the pinned System.Text.Json scalar string encoding.

func JSONString(value string) []byte {
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

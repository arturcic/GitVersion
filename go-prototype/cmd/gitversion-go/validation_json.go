package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// marshalValidationEnvelope uses the pinned System.Text.Json escaping for the
// validation wire format while keeping Go's field order, nulls and numbers.
func marshalValidationEnvelope(result validationEnvelope) ([]byte, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	for i := 0; i < len(data); {
		switch c := data[i]; {
		case c == '\\':
			switch data[i+1] {
			case '"':
				output.WriteString(`\u0022`)
				i += 2
			case 'u':
				output.WriteString(`\u` + strings.ToUpper(string(data[i+2:i+6])))
				i += 6
			default:
				output.Write(data[i : i+2])
				i += 2
			}
		case c >= utf8.RuneSelf || c == 0x7f || c == '\'' || c == '+' || c == '`':
			value, size := utf8.DecodeRune(data[i:])
			for _, unit := range utf16.Encode([]rune{value}) {
				fmt.Fprintf(&output, `\u%04X`, unit)
			}
			i += size
		default:
			output.WriteByte(c)
			i++
		}
	}
	return output.Bytes(), nil
}

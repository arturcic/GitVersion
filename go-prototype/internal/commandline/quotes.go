package commandline

import "strings"

// SplitUnquoted keeps quoted segments intact and removes empty entries.
func SplitUnquoted(input string, separator byte) []string {
	parts := []string{}
	start := 0
	quoted := false
	for i := 0; i < len(input); i++ {
		if input[i] == '"' && (i == 0 || input[i-1] != '\\') {
			quoted = !quoted
		}
		if input[i] == separator && !quoted {
			if i > start {
				parts = append(parts, input[start:i])
			}
			start = i + 1
		}
	}
	if start < len(input) {
		parts = append(parts, input[start:])
	}
	return parts
}

func UnquoteText(input string) string {
	input = strings.TrimPrefix(input, `"`)
	if strings.HasSuffix(input, `"`) && (len(input) == 1 || input[len(input)-2] != '\\') {
		input = input[:len(input)-1]
	}
	return strings.ReplaceAll(input, `\"`, `"`)
}

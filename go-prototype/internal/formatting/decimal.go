package formatting

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Decimal keeps decimal input exact when adapting C# decimal constants.
type Decimal string

func (d Decimal) Format(format string) (string, error) {
	text := string(d)
	if strings.ContainsAny(text, "/eE") {
		return "", fmt.Errorf("invalid decimal %q", text)
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return "", fmt.Errorf("invalid decimal %q", text)
	}
	if format == "" || format == "g" || format == "G" {
		return text, nil
	}
	code, precision, standard, err := standardNumberFormat(format)
	if err != nil {
		return "", err
	}
	if !standard {
		return customIntegral(value, format)
	}
	switch code {
	case 'F', 'N', 'C', 'P':
		if precision < 0 {
			precision = 2
		}
		if code == 'P' {
			value.Mul(value, big.NewRat(100, 1))
		}
		result := fixedDecimal(value, precision)
		if code != 'F' {
			result = groupThousands(result)
		}
		if code == 'C' {
			if strings.HasPrefix(result, "-") {
				result = "(¤" + result[1:] + ")"
			} else {
				result = "¤" + result
			}
		}
		if code == 'P' {
			result += " %"
		}
		return result, nil
	default:
		return "", fmt.Errorf("unsupported decimal format %q", format)
	}
}
func fixedDecimal(value *big.Rat, precision int) string {
	sign := value.Sign()
	absolute := new(big.Rat).Abs(value)
	power := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(precision)), nil)
	scaled := new(big.Int).Mul(absolute.Num(), power)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(scaled, absolute.Denom(), remainder)
	twice := new(big.Int).Lsh(remainder, 1)
	cmp := twice.Cmp(absolute.Denom())
	if cmp > 0 || cmp == 0 && quotient.Bit(0) == 1 {
		quotient.Add(quotient, big.NewInt(1))
	}
	digits := quotient.String()
	if precision > 0 {
		if len(digits) <= precision {
			digits = strings.Repeat("0", precision+1-len(digits)) + digits
		}
		digits = digits[:len(digits)-precision] + "." + digits[len(digits)-precision:]
	}
	if sign < 0 && quotient.Sign() != 0 {
		digits = "-" + digits
	}
	return digits
}
func standardNumberFormat(format string) (byte, int, bool, error) {
	if format == "" {
		return 'G', -1, true, nil
	}
	code := format[0]
	if code >= 'a' && code <= 'z' {
		code -= 32
	}
	if code < 'A' || code > 'Z' {
		return 0, 0, false, nil
	}
	if len(format) == 1 {
		return code, -1, true, nil
	}
	for _, c := range format[1:] {
		if c < '0' || c > '9' {
			return 0, 0, false, nil
		}
	}
	precision, err := strconv.Atoi(format[1:])
	if err != nil || precision > 999999999 {
		return 0, 0, false, fmt.Errorf("numeric precision out of range")
	}
	return code, precision, true, nil
}
func customIntegral(value *big.Rat, format string) (string, error) {
	sections := []string{}
	start := 0
	var quote byte
	escaped := false
	for i := 0; i < len(format); i++ {
		c := format[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == ';' {
			sections = append(sections, format[start:i])
			start = i + 1
		}
	}
	if quote != 0 || escaped {
		return "", fmt.Errorf("unclosed numeric literal")
	}
	sections = append(sections, format[start:])
	if len(sections) > 3 {
		return "", fmt.Errorf("too many numeric sections")
	}
	selected := sections[0]
	negative := value.Sign() < 0
	if negative && len(sections) > 1 && sections[1] != "" {
		selected = sections[1]
		negative = false
	}
	if value.Sign() == 0 && len(sections) > 2 && sections[2] != "" {
		selected = sections[2]
	}
	type token struct {
		literal        byte
		zero, optional bool
	}
	tokens := []token{}
	quote = 0
	escaped = false
	zeros, places := 0, 0
	for i := 0; i < len(selected); i++ {
		c := selected[i]
		if escaped {
			tokens = append(tokens, token{literal: c})
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				tokens = append(tokens, token{literal: c})
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		switch c {
		case '0':
			tokens = append(tokens, token{zero: true})
			zeros++
			places++
		case '#':
			tokens = append(tokens, token{optional: true})
			places++
		case '.', ',', '%':
			return "", fmt.Errorf("custom fractional/scaled numeric format is not yet supported")
		default:
			tokens = append(tokens, token{literal: c})
		}
	}
	if places == 0 {
		var out strings.Builder
		for _, t := range tokens {
			out.WriteByte(t.literal)
		}
		return out.String(), nil
	}
	absolute := new(big.Rat).Abs(value)
	digits := fixedDecimal(absolute, 0)
	if digits == "0" && zeros == 0 {
		digits = ""
	}
	if len(digits) < zeros {
		digits = strings.Repeat("0", zeros-len(digits)) + digits
	}
	rendered := make([]string, len(tokens))
	at := len(digits)
	first := -1
	for i := len(tokens) - 1; i >= 0; i-- {
		t := tokens[i]
		if !t.zero && !t.optional {
			rendered[i] = string(t.literal)
			continue
		}
		first = i
		if at > 0 {
			at--
			rendered[i] = digits[at : at+1]
		} else if t.zero {
			rendered[i] = "0"
		}
	}
	if at > 0 {
		rendered[first] = digits[:at] + rendered[first]
	}
	result := strings.Join(rendered, "")
	if negative && result != "" {
		result = "-" + result
	}
	return result, nil
}

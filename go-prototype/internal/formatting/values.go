package formatting

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
)

type StringFormatter struct{}

func (StringFormatter) Priority() int { return 2 }
func (StringFormatter) TryFormat(value any, format string) (string, bool, error) {
	s, ok := value.(string)
	if !ok {
		return "", false, nil
	}
	if strings.TrimSpace(s) == "" {
		return "", true, nil
	}
	switch format {
	case "u":
		return strings.ToUpper(s), true, nil
	case "l":
		return strings.ToLower(s), true, nil
	case "s":
		rs := []rune(strings.ToLower(s))
		rs[0] = unicode.ToUpper(rs[0])
		return string(rs), true, nil
	case "t":
		rs := []rune(strings.ToLower(s))
		start := true
		for i, c := range rs {
			if unicode.IsLetter(c) {
				if start {
					rs[i] = unicode.ToUpper(c)
				}
				start = false
			} else if c != '\'' {
				start = !unicode.IsNumber(c)
			}
		}
		return string(rs), true, nil
	case "c":
		// The pin iterates UTF-16 char values; non-BMP surrogate units separate words.
		out := []rune{}
		start := true
		for _, unit := range utf16.Encode([]rune(s)) {
			c := rune(unit)
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
				start = true
				continue
			}
			if start {
				c = unicode.ToUpper(c)
			} else {
				c = unicode.ToLower(c)
			}
			out = append(out, c)
			start = false
		}
		return string(out), true, nil
	default:
		return "", false, nil
	}
}

type NumericFormatter struct{}

func (NumericFormatter) Priority() int { return 1 }
func (NumericFormatter) TryFormat(value any, format string) (string, bool, error) {
	s, ok := value.(string)
	if !ok {
		return "", false, nil
	}
	s = strings.TrimSpace(s)
	integer, integerError := strconv.ParseInt(s, 10, 32)
	if format == "" {
		if integerError == nil {
			return strconv.FormatInt(integer, 10), true, nil
		}
		return "", false, fmt.Errorf("empty numeric format")
	}
	parsedFloat, floatError := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
	if integerError != nil && floatError != nil {
		return "", false, nil
	}
	digits := true
	for _, c := range format {
		if c < '0' || c > '9' {
			digits = false
			break
		}
	}
	if digits && integerError == nil {
		return customInteger(integer, format), true, nil
	}
	code := format[0]
	upper := byte(unicode.ToUpper(rune(code)))
	precision := -1
	if len(format) > 1 {
		n, err := strconv.Atoi(format[1:])
		if err != nil || n < 0 || n > 999999999 {
			return "", false, fmt.Errorf("unsupported numeric format %q", format)
		}
		precision = n
	}
	if strings.ContainsRune("BDX", rune(upper)) && integerError == nil {
		base := 10
		if upper == 'B' {
			base = 2
		}
		if upper == 'X' {
			base = 16
		}
		neg := integer < 0
		var result string
		if neg && base != 10 {
			result = strconv.FormatUint(uint64(uint32(integer)), base)
		} else {
			n := integer
			if neg {
				n = -n
			}
			result = strconv.FormatInt(n, base)
		}
		if code == 'X' {
			result = strings.ToUpper(result)
		}
		if precision > len(result) {
			result = strings.Repeat("0", precision-len(result)) + result
		}
		if neg && base == 10 {
			result = "-" + result
		}
		return result, true, nil
	}
	n := parsedFloat
	if floatError != nil {
		return "", false, nil
	}
	if !strings.ContainsRune("FEGNCP", rune(upper)) {
		return "", false, fmt.Errorf("unsupported numeric format %q", format)
	}
	if math.IsInf(n, 0) || math.IsNaN(n) {
		return strconv.FormatFloat(n, 'g', -1, 64), true, nil
	}
	if upper == 'G' {
		p := precision
		if p == 0 {
			p = -1
		}
		return strconv.FormatFloat(n, code, p, 64), true, nil
	}
	if precision < 0 {
		precision = 2
		if upper == 'E' {
			precision = 6
		}
	}
	if upper == 'E' {
		result := strconv.FormatFloat(n, code, precision, 64)
		pos := strings.LastIndexAny(result, "eE")
		exponent := result[pos+1:]
		for len(exponent) < 4 {
			exponent = exponent[:1] + "0" + exponent[1:]
		}
		return result[:pos+1] + exponent, true, nil
	}
	if upper == 'P' {
		n *= 100
	}
	result := strconv.FormatFloat(n, 'f', precision, 64)
	if upper == 'N' || upper == 'C' || upper == 'P' {
		result = groupThousands(result)
	}
	if upper == 'C' {
		if strings.HasPrefix(result, "-") {
			result = "(¤" + result[1:] + ")"
		} else {
			result = "¤" + result
		}
	}
	if upper == 'P' {
		result += " %"
	}
	return result, true, nil
}
func customInteger(n int64, format string) string {
	neg := n < 0
	if neg {
		n = -n
	}
	digits := strconv.FormatInt(n, 10)
	zeros := strings.Count(format, "0")
	if zeros == 0 {
		return format
	}
	if len(digits) < zeros {
		digits = strings.Repeat("0", zeros-len(digits)) + digits
	}
	var out strings.Builder
	at := 0
	first := true
	for _, c := range format {
		if c == '0' {
			if first {
				take := len(digits) - zeros + 1
				out.WriteString(digits[:take])
				at = take
				first = false
			} else {
				out.WriteByte(digits[at])
				at++
			}
		} else {
			out.WriteRune(c)
		}
	}
	result := out.String()
	if neg {
		result = "-" + result
	}
	return result
}
func groupThousands(s string) string {
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign = "-"
		s = s[1:]
	}
	whole, fraction, has := strings.Cut(s, ".")
	out := []string{}
	for len(whole) > 3 {
		out = append([]string{whole[len(whole)-3:]}, out...)
		whole = whole[:len(whole)-3]
	}
	out = append([]string{whole}, out...)
	result := sign + strings.Join(out, ",")
	if has {
		result += "." + fraction
	}
	return result
}

package formatting

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DateValue preserves whether a parsed date was explicitly UTC. An unzoned
// DateTime has no K suffix, even if its calendar fields are stored using UTC.
type DateValue struct {
	Time time.Time
	UTC  bool
}
type DateFormatter struct{}

func (DateFormatter) Priority() int { return 2 }
func (DateFormatter) TryFormat(value any, format string) (string, bool, error) {
	var date DateValue
	switch v := value.(type) {
	case DateValue:
		date = v
	case time.Time:
		date = DateValue{v, v.Location() == time.UTC}
	case string:
		var ok bool
		date, ok = ParseDate(v)
		if !ok {
			return "", false, nil
		}
	default:
		return "", false, nil
	}
	result, err := formatDate(date.Time, format, date.UTC)
	return result, err == nil, err
}

// ParseDate accepts invariant ISO and common invariant date/time representations.
// Culture-specific DateTime parsing remains a separate compatibility surface.
func ParseDate(value string) (DateValue, bool) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02", "01/02/2006 15:04:05", "01/02/2006", "January 2, 2006", time.RFC1123, time.RFC1123Z} {
		if date, err := time.Parse(layout, value); err == nil {
			return DateValue{date, strings.HasSuffix(value, "Z") || strings.HasSuffix(value, "GMT")}, true
		}
	}
	return DateValue{}, false
}
func formatDate(date time.Time, format string, utc bool) (string, error) {
	if format == "" {
		format = "G"
	}
	if len(format) == 1 {
		switch format {
		case "d":
			format = "MM/dd/yyyy"
		case "D":
			format = "dddd, dd MMMM yyyy"
		case "f":
			format = "dddd, dd MMMM yyyy HH:mm"
		case "F", "U":
			if format == "U" {
				date = date.UTC()
				utc = true
			}
			format = "dddd, dd MMMM yyyy HH:mm:ss"
		case "g":
			format = "MM/dd/yyyy HH:mm"
		case "G":
			format = "MM/dd/yyyy HH:mm:ss"
		case "m", "M":
			format = "MMMM dd"
		case "o", "O":
			format = "yyyy-MM-ddTHH:mm:ss.fffffffK"
		case "r", "R":
			format = "ddd, dd MMM yyyy HH:mm:ss 'GMT'"
		case "s":
			format = "yyyy-MM-ddTHH:mm:ss"
		case "t":
			format = "HH:mm"
		case "T":
			format = "HH:mm:ss"
		case "u":
			format = "yyyy-MM-dd HH:mm:ss'Z'"
		case "y", "Y":
			format = "yyyy MMMM"
		default:
			return "", fmt.Errorf("unknown date format %q", format)
		}
	}
	var out strings.Builder
	for i := 0; i < len(format); {
		c := format[i]
		if c == '\'' || c == '"' {
			quote := c
			i++
			closed := false
			for i < len(format) {
				if format[i] == quote {
					i++
					closed = true
					break
				}
				if format[i] == '\\' {
					i++
					if i == len(format) {
						break
					}
				}
				out.WriteByte(format[i])
				i++
			}
			if !closed {
				return "", fmt.Errorf("unterminated date literal")
			}
			continue
		}
		if c == '\\' {
			i++
			if i == len(format) {
				return "", fmt.Errorf("incomplete date escape")
			}
			out.WriteByte(format[i])
			i++
			continue
		}
		forced := false
		if c == '%' {
			i++
			if i == len(format) || format[i] == '%' {
				return "", fmt.Errorf("invalid date format escape")
			}
			c = format[i]
			forced = true
		}
		n := 1
		if !forced {
			for i+n < len(format) && format[i+n] == c {
				n++
			}
		}
		i += n
		num := func(value, width int) { out.WriteString(fmt.Sprintf("%0*d", width, value)) }
		switch c {
		case 'y':
			y := date.Year()
			if n <= 2 {
				y %= 100
			}
			num(y, n)
		case 'M':
			if n == 3 {
				out.WriteString(date.Format("Jan"))
			} else if n >= 4 {
				out.WriteString(date.Month().String())
			} else {
				num(int(date.Month()), n)
			}
		case 'd':
			if n == 3 {
				out.WriteString(date.Format("Mon"))
			} else if n >= 4 {
				out.WriteString(date.Weekday().String())
			} else {
				num(date.Day(), n)
			}
		case 'H':
			num(date.Hour(), min(n, 2))
		case 'h':
			hour := date.Hour() % 12
			if hour == 0 {
				hour = 12
			}
			num(hour, min(n, 2))
		case 'm':
			num(date.Minute(), min(n, 2))
		case 's':
			num(date.Second(), min(n, 2))
		case 'f', 'F':
			if n > 7 {
				return "", fmt.Errorf("date fractions support at most seven digits")
			}
			fraction := fmt.Sprintf("%09d", date.Nanosecond())[:n]
			if c == 'F' {
				fraction = strings.TrimRight(fraction, "0")
				if fraction == "" && strings.HasSuffix(out.String(), ".") {
					prior := strings.TrimSuffix(out.String(), ".")
					out.Reset()
					out.WriteString(prior)
				}
			}
			out.WriteString(fraction)
		case 't':
			label := "AM"
			if date.Hour() >= 12 {
				label = "PM"
			}
			if n == 1 {
				label = label[:1]
			}
			out.WriteString(label)
		case 'z':
			_, offset := date.Zone()
			sign := "+"
			if offset < 0 {
				sign = "-"
				offset = -offset
			}
			out.WriteString(sign)
			hour := offset / 3600
			if n == 1 {
				out.WriteString(strconv.Itoa(hour))
			} else {
				num(hour, 2)
			}
			if n >= 3 {
				out.WriteByte(':')
				num(offset/60%60, 2)
			}
		case 'K':
			if utc {
				out.WriteByte('Z')
			} else if date.Location() != time.UTC {
				_, offset := date.Zone()
				sign := "+"
				if offset < 0 {
					sign = "-"
					offset = -offset
				}
				fmt.Fprintf(&out, "%s%02d:%02d", sign, offset/3600, offset/60%60)
			}
		case 'g':
			out.WriteString("A.D.")
		default:
			for range n {
				out.WriteByte(c)
			}
		}
	}
	return out.String(), nil
}

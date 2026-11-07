package formatting

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func typedCaseValue(t *testing.T, kind string, value any) any {
	t.Helper()
	switch kind {
	case "null":
		return nil
	case "string", "float":
		return value
	case "integer":
		return int32(value.(float64))
	case "decimal":
		return Decimal(value.(string))
	case "date":
		d, ok := ParseDate(value.(string))
		if !ok {
			t.Fatal("invalid pinned date fixture", value)
		}
		return d
	default:
		t.Fatal("unknown fixture kind", kind)
		return nil
	}
}
func TestCSharpDateAndFormattableFormatters(t *testing.T) {
	for _, spec := range []struct {
		file           string
		methods, cases int
	}{{"csharp-formattable-formatter.json", 5, 12}, {"csharp-date-formatter.json", 3, 4}} {
		runCases(t, originalCases(t, spec.file, spec.methods, spec.cases), func(t *testing.T, tc originalCase) {
			var got string
			var ok bool
			var err error
			switch tc.Operation {
			case "priority-formattable":
				if (FormattableFormatter{}).Priority() != tc.Priority {
					t.Fatal("priority differs")
				}
				return
			case "priority-date":
				if (DateFormatter{}).Priority() != tc.Priority {
					t.Fatal("priority differs")
				}
				return
			case "formattable":
				got, ok, err = FormattableFormatter{}.TryFormat(typedCaseValue(t, tc.Kind, tc.Value), tc.Format)
			case "date":
				got, ok, err = DateFormatter{}.TryFormat(typedCaseValue(t, tc.Kind, tc.Value), tc.Format)
			default:
				t.Fatal("unknown operation")
			}
			if err != nil || ok != tc.Success || got != tc.Expected {
				t.Fatalf("want %q handled=%t got %q handled=%t (%v)", tc.Expected, tc.Success, got, ok, err)
			}
		})
	}
}

type testFormatter struct {
	priority int
	marker   string
}

func (f testFormatter) Priority() int { return f.priority }
func (f testFormatter) TryFormat(value any, format string) (string, bool, error) {
	if s, ok := value.(string); ok && format == "custom" {
		return f.marker + s, true, nil
	}
	return "", false, nil
}
func TestCSharpValueFormatterDispatch(t *testing.T) {
	runCases(t, originalCases(t, "csharp-value-formatter.json", 9, 9), func(t *testing.T, tc originalCase) {
		f := NewValueFormatter()
		for i, step := range tc.Stages {
			switch step.Op {
			case "register":
				if err := f.Register(testFormatter{step.Priority, step.Marker}); err != nil {
					t.Fatal(err)
				}
			case "remove":
				if step.Name == "numeric" {
					f.Remove(NumericFormatter{})
				} else if step.Name == "formattable" {
					f.Remove(FormattableFormatter{})
				} else {
					t.Fatal("unknown removal")
				}
			case "format":
				got, ok, err := f.TryFormat(typedCaseValue(t, step.Kind, step.Value), step.Format)
				if err != nil || got != step.Expected || step.Success != nil && ok != *step.Success {
					t.Fatalf("stage %d want %q handled=%v got %q handled=%t (%v)", i, step.Expected, step.Success, got, ok, err)
				}
			default:
				t.Fatal("unknown stage")
			}
		}
	})
}
func TestFormatterRegistryAndNumericBoundaries(t *testing.T) {
	f := NewValueFormatter()
	if f.Priority() != 0 {
		t.Fatal("combiner priority")
	}
	if err := f.Register(nil); err == nil {
		t.Fatal("nil formatter accepted")
	}
	var null *testFormatter
	if err := f.Register(null); err == nil {
		t.Fatal("typed nil formatter accepted")
	}
	for _, item := range []testFormatter{{99, "LOW:"}, {0, "FIRST:"}, {0, "SECOND:"}} {
		if err := f.Register(item); err != nil {
			t.Fatal(err)
		}
	}
	if got, ok, err := f.TryFormat("x", "custom"); err != nil || !ok || got != "FIRST:x" {
		t.Fatal("stable priority order", got, ok, err)
	}
	f.Remove(testFormatter{})
	if got, ok, err := f.TryFormat("x", "custom"); err != nil || ok || got != "" {
		t.Fatal("remove must delete every instance of type", got, ok, err)
	}
	for _, v := range []any{nil, (*testFormatter)(nil)} {
		if got, ok, err := f.TryFormat(v, "custom"); err != nil || ok || got != "" {
			t.Fatal("null value must be unhandled")
		}
	}
	for _, tc := range []struct {
		value        any
		format, want string
	}{
		{int64(math.MinInt64), "d", "-9223372036854775808"}, {int64(math.MinInt64), "0000", "-9223372036854775808"},
		{int64(-1), "x", "ffffffffffffffff"}, {int32(-1), "x", "ffffffff"}, {uint64(math.MaxUint64), "D", "18446744073709551615"},
		{Decimal("1.225"), "F2", "1.22"}, {Decimal("1.235"), "F2", "1.24"}, {Decimal("12345678901234567890.125"), "F2", "12345678901234567890.12"},
		{int32(-12), "0000;(0000);'zero'", "(0012)"}, {int32(0), "0000;(0000);'zero'", "zero"},
		{int32(0), "#", ""}, {int32(12), `'value='0000`, "value=0012"}, {Decimal("0.125"), "P1", "12.5 %"},
	} {
		got, ok, err := FormattableFormatter{}.TryFormat(tc.value, tc.format)
		if err != nil || !ok || got != tc.want {
			t.Fatalf("%v %s: want %q got %q %t (%v)", tc.value, tc.format, tc.want, got, ok, err)
		}
	}
	for _, tc := range []struct {
		value  any
		format string
	}{{float64(42), "d"}, {int32(1), "Q"}, {int32(1), "'unclosed"}} {
		got, ok, err := FormattableFormatter{}.TryFormat(tc.value, tc.format)
		if err != nil || ok || got != fmt.Sprintf("Format '%s' is not supported in FormattableFormatter", tc.format) {
			t.Fatal("formattable failure result", got, ok, err)
		}
	}
	for _, tc := range []struct{ value, want string }{{"NaN", "NaN"}, {"Infinity", "Infinity"}, {"-Infinity", "-Infinity"}} {
		got, ok, err := NumericFormatter{}.TryFormat(tc.value, "G")
		if err != nil || !ok || got != tc.want {
			t.Fatal("nonfinite formatting", got, ok, err)
		}
	}
}
func TestDateFormattingAndTemplateDispatch(t *testing.T) {
	d := time.Date(2024, 2, 29, 15, 4, 5, 120000000, time.UTC)
	for _, tc := range []struct{ format, want string }{{"yyyy-MM-dd HH:mm:ss.fffffff K", "2024-02-29 15:04:05.1200000 Z"}, {"ddd, dd MMM yyyy", "Thu, 29 Feb 2024"}, {"hh:mm tt", "03:04 PM"}, {"%d", "29"}, {"'date:' yyyy\\/MM\\/dd", "date: 2024/02/29"}, {"测试 yyyy", "测试 2024"}, {"O", "2024-02-29T15:04:05.1200000Z"}, {"ss.FFF", "05.12"}} {
		got, ok, err := DateFormatter{}.TryFormat(d, tc.format)
		if err != nil || !ok || got != tc.want {
			t.Fatalf("%q want %q got %q %t (%v)", tc.format, tc.want, got, ok, err)
		}
	}
	if got, ok, err := (DateFormatter{}).TryFormat(d.Truncate(time.Second), "ss.FFF"); err != nil || !ok || got != "05" {
		t.Fatal("optional fractional separator", got, err)
	}
	for _, format := range []string{"Q", "%", "%%", "yyyy'bad", "ffffffff"} {
		if _, _, err := (DateFormatter{}).TryFormat(d, format); err == nil {
			t.Fatal("bad date format accepted", format)
		}
	}
	for _, v := range []any{nil, 42, "not a date"} {
		got, ok, err := DateFormatter{}.TryFormat(v, "yyyy")
		if err != nil || ok || got != "" {
			t.Fatal("unsupported date input accepted")
		}
	}
	if got, ok, err := (DateFormatter{}).TryFormat("2024-02-29", "yyyy-MM-ddK"); err != nil || !ok || got != "2024-02-29" {
		t.Fatal("unspecified date acquired UTC marker", got, err)
	}
	zone := time.FixedZone("offset", 5*3600+30*60)
	date := time.Date(2024, 2, 29, 12, 0, 0, 0, zone)
	if got, _, err := (DateFormatter{}).TryFormat(date, "zzz"); err != nil || got != "+05:30" {
		t.Fatal("offset formatting", got, err)
	}
	got, err := FormatWith("{Date:yyyy-MM-dd}-{Value:u}", map[string]any{"Date": "2024-02-29", "Value": "test"}, nil)
	if err != nil || got != "2024-02-29-TEST" {
		t.Fatal("date formatter not connected to template", got, err)
	}
	if got, err := FormatWith("{Date:yyyy-MM-dd}", map[string]any{"Date": d}, nil); err != nil || got != "2024-02-29" {
		t.Fatal("typed date must survive member conversion", got, err)
	}
	if got, err := Decimal("1.00").Format("G"); err != nil || got != "1.00" {
		t.Fatal("decimal scale lost", got, err)
	}
	if _, err := Decimal("1/3").Format("F2"); err == nil {
		t.Fatal("nondecimal syntax accepted")
	}
}

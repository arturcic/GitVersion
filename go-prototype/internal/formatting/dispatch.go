package formatting

import (
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Formatter interface {
	Priority() int
	TryFormat(any, string) (string, bool, error)
}

// Formattable is the Go counterpart of an object with a custom format operation.
type Formattable interface{ Format(string) (string, error) }

type FormattableFormatter struct{}

func (FormattableFormatter) Priority() int { return 2 }
func (FormattableFormatter) TryFormat(value any, format string) (string, bool, error) {
	if nullValue(value) || strings.TrimSpace(format) == "" {
		return "", false, nil
	}
	var result string
	var err error
	switch v := value.(type) {
	case Formattable:
		result, err = v.Format(format)
	case time.Time:
		result, err = formatDate(v, format, v.Location() == time.UTC)
	case DateValue:
		result, err = formatDate(v.Time, format, v.UTC)
	default:
		rv := reflect.ValueOf(value)
		var text string
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			text = strconv.FormatInt(rv.Int(), 10)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			text = strconv.FormatUint(rv.Uint(), 10)
		case reflect.Float32, reflect.Float64:
			text = strconv.FormatFloat(rv.Float(), 'g', -1, rv.Type().Bits())
		default:
			return "", false, nil
		}
		var ok bool
		code, _, standard, parseError := standardNumberFormat(format)
		if parseError != nil {
			err = parseError
		} else if !standard {
			number, valid := new(big.Rat).SetString(text)
			if !valid {
				return "", false, nil
			}
			result, err = customIntegral(number, format)
			ok = err == nil
		} else if strings.ContainsRune("BDX", rune(code)) {
			if rv.Kind() == reflect.Float32 || rv.Kind() == reflect.Float64 {
				err = fmt.Errorf("integer format on floating point value")
			} else {
				result, err = typedIntegerFormat(rv, format)
				ok = err == nil
			}
		} else {
			result, ok, err = NumericFormatter{}.TryFormat(text, format)
		}
		if err == nil && !ok {
			return "", false, nil
		}
	}
	if err != nil {
		return fmt.Sprintf("Format '%s' is not supported in FormattableFormatter", format), false, nil
	}
	return result, true, nil
}

// ValueFormatter takes snapshots of its registry so concurrent formatting is
// independent of subsequent registration/removal. Equal priorities remain stable.
type ValueFormatter struct {
	mu         sync.RWMutex
	formatters []Formatter
}

func NewValueFormatter() *ValueFormatter {
	return &ValueFormatter{formatters: []Formatter{StringFormatter{}, FormattableFormatter{}, NumericFormatter{}, DateFormatter{}}}
}
func (*ValueFormatter) Priority() int { return 0 }
func (f *ValueFormatter) Register(formatter Formatter) error {
	if formatter == nil {
		return fmt.Errorf("formatter cannot be null")
	}
	v := reflect.ValueOf(formatter)
	if v.Kind() == reflect.Pointer && v.IsNil() {
		return fmt.Errorf("formatter cannot be null")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.formatters = append(f.formatters, formatter)
	return nil
}
func (f *ValueFormatter) Remove(formatter Formatter) {
	typ := reflect.TypeOf(formatter)
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Formatter, 0, len(f.formatters))
	for _, item := range f.formatters {
		if reflect.TypeOf(item) != typ {
			out = append(out, item)
		}
	}
	f.formatters = out
}
func (f *ValueFormatter) TryFormat(value any, format string) (string, bool, error) {
	if nullValue(value) {
		return "", false, nil
	}
	f.mu.RLock()
	items := append([]Formatter(nil), f.formatters...)
	f.mu.RUnlock()
	sort.SliceStable(items, func(i, j int) bool { return items[i].Priority() < items[j].Priority() })
	result := ""
	for _, item := range items {
		v, ok, err := item.TryFormat(value, format)
		result = v
		if err != nil {
			return "", false, err
		}
		if ok {
			return v, true, nil
		}
	}
	return result, false, nil
}

var defaultValueFormatter = NewValueFormatter()

func typedIntegerFormat(value reflect.Value, format string) (string, error) {
	code, precision, _, err := standardNumberFormat(format)
	if err != nil {
		return "", err
	}
	signed := value.Kind() >= reflect.Int && value.Kind() <= reflect.Int64
	negative := signed && value.Int() < 0
	var digits string
	if code == 'D' {
		if signed {
			digits = strconv.FormatInt(value.Int(), 10)
		} else {
			digits = strconv.FormatUint(value.Uint(), 10)
		}
		digits = strings.TrimPrefix(digits, "-")
	} else {
		var number uint64
		if signed {
			number = uint64(value.Int())
			if value.Type().Bits() < 64 {
				number &= (uint64(1) << value.Type().Bits()) - 1
			}
		} else {
			number = value.Uint()
		}
		base := 16
		if code == 'B' {
			base = 2
		}
		digits = strconv.FormatUint(number, base)
		if format[0] == 'X' {
			digits = strings.ToUpper(digits)
		}
	}
	if precision > len(digits) {
		digits = strings.Repeat("0", precision-len(digits)) + digits
	}
	if negative && code == 'D' {
		digits = "-" + digits
	}
	return digits, nil
}

func nullValue(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return v.IsNil()
	}
	return false
}

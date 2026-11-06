// Package parity implements strict, typed JSON comparison without lossy coercion.
package parity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"unicode/utf8"
)

// Object rejects duplicate keys at every depth, trailing values, and non-object roots.
// Numbers retain their exact JSON representation, including integers beyond 2^53.
func Object(data []byte) (map[string]any, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("invalid UTF-8 JSON")
	}
	if err := unicodeScalars(data); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	v, err := value(d)
	if err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON: %v", err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("JSON root must be an object")
	}
	if len(m) == 0 {
		return nil, fmt.Errorf("version object must not be empty")
	}
	return m, nil
}

// encoding/json replaces lone UTF-16 surrogate escapes with U+FFFD. Reject them
// so an invalid candidate string cannot compare equal to a real replacement rune.
func unicodeScalars(data []byte) error {
	inString := false
	for i := 0; i < len(data); i++ {
		if data[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || data[i] != '\\' {
			continue
		}
		i++
		if i >= len(data) || data[i] != 'u' {
			continue
		}
		if i+4 >= len(data) {
			return fmt.Errorf("incomplete Unicode escape")
		}
		u, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
		if err != nil {
			return fmt.Errorf("invalid Unicode escape")
		}
		i += 4
		if u >= 0xd800 && u <= 0xdbff {
			if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
				return fmt.Errorf("unpaired high surrogate")
			}
			low, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return fmt.Errorf("invalid surrogate pair")
			}
			i += 6
		} else if u >= 0xdc00 && u <= 0xdfff {
			return fmt.Errorf("unpaired low surrogate")
		}
	}
	return nil
}

func value(d *json.Decoder) (any, error) {
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch t {
	case json.Delim('{'):
		m := map[string]any{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return nil, err
			}
			key, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("invalid object key")
			}
			if _, ok := m[key]; ok {
				return nil, fmt.Errorf("duplicate key %q", key)
			}
			v, err := value(d)
			if err != nil {
				return nil, err
			}
			m[key] = v
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, fmt.Errorf("unclosed object: %v", err)
		}
		return m, nil
	case json.Delim('['):
		a := []any{}
		for d.More() {
			v, err := value(d)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, fmt.Errorf("unclosed array: %v", err)
		}
		return a, nil
	default:
		if _, ok := t.(json.Delim); ok {
			return nil, fmt.Errorf("unexpected delimiter")
		}
		return t, nil
	}
}

type Difference struct {
	Field    string `json:"field"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
	Reason   string `json:"reason"`
}

// Compare ignores key ordering and whitespace only. Missing is distinct from null.
func Compare(expected, actual []byte) ([]Difference, error) {
	e, err := Object(expected)
	if err != nil {
		return nil, fmt.Errorf("expected: %w", err)
	}
	a, err := Object(actual)
	if err != nil {
		return nil, fmt.Errorf("actual: %w", err)
	}
	out := []Difference{}
	keys := map[string]bool{}
	for k := range e {
		keys[k] = true
	}
	for k := range a {
		keys[k] = true
	}
	names := []string{}
	for k := range keys {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		ev, ep := e[k]
		av, ap := a[k]
		reason := "value or type differs"
		if !ep {
			reason = "unexpected key"
		} else if !ap {
			reason = "missing key"
		} else if reflect.DeepEqual(ev, av) {
			continue
		}
		out = append(out, Difference{k, ev, av, reason})
	}
	return out, nil
}

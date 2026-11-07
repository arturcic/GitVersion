package formatting

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// EnvironmentLookup is injected so formatting never reads ambient process state.
type EnvironmentLookup func(string) (string, bool)

var expressionPattern = regexp.MustCompile(`\{([^{}]+)\}`)

// FormatWith evaluates property/environment placeholders and ordered fallbacks.
// Maps use exact keys, as the C# dictionary overload does. Structs expose public
// Go fields and recursively resolve nested fields, adapting C# object properties.
// Formatter primitives currently support invariant string and numeric values;
// culture-specific, date and registered custom formatters are separate ports.
func FormatWith(template string, source any, environment EnvironmentLookup) (string, error) {
	if source == nil {
		return "", fmt.Errorf("source cannot be null")
	}
	v := reflect.ValueOf(source)
	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Map) && v.IsNil() {
		return "", fmt.Errorf("source cannot be null")
	}
	matches := expressionPattern.FindAllStringSubmatchIndex(template, -1)
	var out strings.Builder
	last := 0
	for _, m := range matches {
		out.WriteString(template[last:m[0]])
		value, err := evaluate(template[m[2]:m[3]], source, environment)
		if err != nil {
			return "", err
		}
		out.WriteString(value)
		last = m[1]
	}
	out.WriteString(template[last:])
	return out.String(), nil
}
func evaluate(expression string, source any, environment EnvironmentLookup) (string, error) {
	tokens, err := ParseTokens(expression)
	if err != nil {
		return "", err
	}
	var lastError error
	s := InputSanitizer{}
	for _, token := range tokens {
		if token.Type == Literal {
			return token.Name, nil
		}
		value, present, err := resolveToken(token, source, environment)
		if err != nil {
			lastError = err
			continue
		}
		if !present {
			continue
		}
		if token.Format != nil && *token.Format != "" {
			format, err := s.SanitizeFormat(*token.Format)
			if err != nil {
				lastError = err
				continue
			}
			formatted, ok, err := defaultValueFormatter.TryFormat(value, format)
			if err != nil {
				lastError = err
				continue
			}
			if ok {
				return formatted, nil
			}
		}
		return value, nil
	}
	if lastError != nil {
		return "", lastError
	}
	return "", nil
}
func resolveToken(token Token, source any, environment EnvironmentLookup) (string, bool, error) {
	s := InputSanitizer{}
	if token.Type == Environment {
		if _, err := s.SanitizeEnvVarName(&token.Name); err != nil {
			return "", false, err
		}
		if environment != nil {
			if v, ok := environment(token.Name); ok {
				return v, true, nil
			}
		}
		return "", false, fmt.Errorf("Environment variable %s not found and no fallback provided", token.Name)
	}
	if _, err := s.SanitizeMemberName(&token.Name); err != nil {
		return "", false, err
	}
	value, found := resolveMember(reflect.ValueOf(source), token.Name)
	if !found {
		return "", false, fmt.Errorf("'%s' is not a valid placeholder", token.Name)
	}
	return memberText(value)
}
func dereference(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}
func resolveMember(source reflect.Value, name string) (reflect.Value, bool) {
	root := dereference(source)
	if !root.IsValid() {
		return reflect.Value{}, true
	}
	if root.Kind() == reflect.Map && root.Type().Key().Kind() == reflect.String {
		v := root.MapIndex(reflect.ValueOf(name).Convert(root.Type().Key()))
		return v, v.IsValid()
	}
	current := root
	for _, part := range strings.Split(name, ".") {
		current = dereference(current)
		if !current.IsValid() {
			return reflect.Value{}, true
		}
		if current.Kind() != reflect.Struct {
			return recursiveMember(root, part, map[reflect.Type]bool{})
		}
		field, ok := current.Type().FieldByName(part)
		if !ok || !field.IsExported() {
			return recursiveMember(root, part, map[reflect.Type]bool{})
		}
		current = safeField(current, field.Index)
	}
	return current, true
}
func recursiveMember(v reflect.Value, name string, seen map[reflect.Type]bool) (reflect.Value, bool) {
	v = dereference(v)
	if !v.IsValid() || v.Kind() != reflect.Struct || seen[v.Type()] {
		return reflect.Value{}, false
	}
	seen[v.Type()] = true
	if field, ok := v.Type().FieldByName(name); ok && field.IsExported() {
		return safeField(v, field.Index), true
	}
	for i := 0; i < v.NumField(); i++ {
		if v.Type().Field(i).IsExported() {
			if value, ok := recursiveMember(v.Field(i), name, seen); ok {
				return value, true
			}
		}
	}
	return reflect.Value{}, false
}
func memberText(value reflect.Value) (string, bool, error) {
	value = dereference(value)
	if !value.IsValid() {
		return "", false, nil
	}
	if value.CanInterface() {
		switch v := value.Interface().(type) {
		case time.Time:
			text, err := formatDate(v, "G", v.Location() == time.UTC)
			return text, true, err
		case DateValue:
			text, err := formatDate(v.Time, "G", v.UTC)
			return text, true, err
		}
		if v, ok := value.Interface().(fmt.Stringer); ok {
			return v.String(), true, nil
		}
	}
	switch value.Kind() {
	case reflect.String:
		return value.String(), true, nil
	case reflect.Bool:
		if value.Bool() {
			return "True", true, nil
		}
		return "False", true, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10), true, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(value.Uint(), 10), true, nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'g', -1, value.Type().Bits()), true, nil
	}
	return "", false, fmt.Errorf("unsupported placeholder value type %s", value.Type())
}

// Promoted fields can cross a nil embedded pointer. Return a nullable value
// instead of letting reflection panic while evaluating a fallback expression.
func safeField(value reflect.Value, index []int) reflect.Value {
	for _, i := range index {
		value = dereference(value)
		if !value.IsValid() {
			return reflect.Value{}
		}
		value = value.Field(i)
	}
	return value
}

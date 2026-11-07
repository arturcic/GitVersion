package output

import (
	"bytes"
	"encoding/json"
	"slices"
)

var variableWireOrder = func() []string {
	names := slices.Clone(VariableNames)
	start := slices.Index(names, "CommitDate")
	names[start], names[start+1], names[start+2] = names[start+1], names[start+2], names[start]
	return names
}()

// MarshalVariables produces the pinned field order for JSON version output.
func MarshalVariables(values map[string]any) ([]byte, error) {
	var result bytes.Buffer
	result.WriteString("{\n")
	for index, name := range variableWireOrder {
		if index > 0 {
			result.WriteString(",\n")
		}
		value, err := json.Marshal(values[name])
		if err != nil {
			return nil, err
		}
		result.WriteString(`  "` + name + `": `)
		result.Write(value)
	}
	result.WriteString("\n}")
	return result.Bytes(), nil
}

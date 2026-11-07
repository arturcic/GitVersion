package config

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationOverrides converts public CLI scalar paths to the internal flat
// override document. Value validation happens after decoding in the resolver.
func ValidationOverrides(values []string, version string) ([]byte, error) {
	if len(values) == 0 {
		return nil, nil
	}
	root := documentMap()
	for _, entry := range values {
		key, value, found := strings.Cut(entry, "=")
		path := ""
		if found {
			path = key
		}
		segments, ok := overridePath(path, version)
		if !ok {
			return nil, validation("OVERRIDE", path, "Invalid --override-config. Use a supported public property path and key=value syntax.")
		}
		if len(value) >= 2 && (value[0] == '\'' || value[0] == '"') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		current := root
		for _, segment := range segments[:len(segments)-1] {
			next := nodeField(current, segment)
			if next == nil {
				next = documentMap()
				putDocumentField(current, segment, next)
			}
			current = next
		}
		name := segments[len(segments)-1]
		if previous := nodeField(current, name); previous != nil {
			*previous = *stringNode(value)
		} else {
			putDocumentField(current, name, stringNode(value))
		}
	}
	return yaml.Marshal(root)
}

func overridePath(path, version string) ([]string, bool) {
	segments := strings.Split(path, ".")
	if len(segments) == 1 && strings.EqualFold(segments[0], "workflow") {
		return []string{"workflow"}, true
	}
	section := ""
	if version == "v7" {
		section = strings.ToLower(segments[0])
		if section != "calculation" && section != "output" {
			return nil, false
		}
		segments = segments[1:]
	}
	var rule fieldRule
	var output bool
	switch len(segments) {
	case 1:
		segments[0] = strings.ToLower(segments[0])
		name := segments[0]
		if name == "workflow" {
			return nil, false
		}
		rule, output = rootField(name), containsProperty(outputProperties, name)
	case 3:
		segments[0], segments[2] = strings.ToLower(segments[0]), strings.ToLower(segments[2])
		if segments[0] != "branches" || segments[1] == "" {
			return nil, false
		}
		rule, output = branchField(segments[2]), containsProperty(outputBranchProperties, segments[2])
	default:
		return nil, false
	}
	if section != "" && (section == "output") != output {
		return nil, false
	}
	switch rule.kind {
	case "string", "enum", "int", "bool", "strategies":
		return segments, true
	}
	return nil, false
}

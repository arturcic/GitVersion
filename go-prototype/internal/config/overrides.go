package config

import (
	"fmt"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/commandline"

	"gopkg.in/yaml.v3"
)

// ValidationOverrides converts public CLI scalar paths to the internal flat
// override document. Value validation happens after decoding in the resolver.
func ValidationOverrides(values []string, version string) ([]byte, error) {
	return parseOverrides(values, version, "--override-config", true)
}

func ParseCLIOverrides(values []string, version, option string) ([]byte, error) {
	return parseOverrides(values, version, option, false)
}

func parseOverrides(values []string, version, option string, safe bool) ([]byte, error) {
	if len(values) == 0 {
		return nil, nil
	}
	root := documentMap()
	for _, entry := range values {
		key, _, found := strings.Cut(entry, "=")
		if !found {
			key = ""
		}
		parts := commandline.SplitUnquoted(entry, '=')
		segments, valid := overridePath(key, version)
		if len(parts) != 2 || !valid {
			if safe {
				return nil, validation("OVERRIDE", key, "Invalid --override-config. Use a supported public property path and key=value syntax.")
			}
			reason := overrideDiagnostic(key, version)
			if len(parts) != 2 {
				reason = "Ensure it is in format 'key=value'."
			}
			return nil, fmt.Errorf("Could not parse %s option: %s. %s", option, entry, reason)
		}
		value := commandline.UnquoteText(parts[1])
		if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
			value = value[1 : len(value)-1]
		}
		if !safe && version == "v7" && segments[0] != "workflow" {
			section := "calculation"
			if (len(segments) == 1 && containsProperty(outputProperties, segments[0])) ||
				(len(segments) == 3 && containsProperty(outputBranchProperties, segments[2])) {
				section = "output"
			}
			segments = append([]string{section}, segments...)
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

func overrideDiagnostic(path, version string) string {
	segments := strings.Split(path, ".")
	branchIndex := 0
	first := strings.ToLower(segments[0])
	if first == "calculation" || first == "output" {
		branchIndex = 1
	}
	if len(segments) > branchIndex+3 && strings.EqualFold(segments[branchIndex], "branches") &&
		branchField(strings.ToLower(segments[len(segments)-1])).kind != "" {
		branch := strings.Join(segments[branchIndex+1:len(segments)-1], ".")
		return fmt.Sprintf("Branch name '%s' contains '.', which cannot be addressed by an override configuration path. Use a configuration file instead.", branch)
	}
	normalized := strings.ToLower(path)
	var replacement string
	if len(segments) == 2 && branchIndex == 1 && strings.EqualFold(segments[1], "workflow") {
		replacement = "workflow"
	} else if candidate, ok := overridePath(strings.Join(segments[branchIndex:], "."), "v6"); ok {
		switch {
		case version == "v6" && branchIndex == 1:
			replacement = strings.Join(candidate, ".")
		case version == "v7":
			section := "calculation"
			if (len(candidate) == 1 && containsProperty(outputProperties, candidate[0])) ||
				(len(candidate) == 3 && containsProperty(outputBranchProperties, candidate[2])) {
				section = "output"
			}
			replacement = section + "." + strings.Join(candidate, ".")
		}
	}
	if replacement != "" {
		return fmt.Sprintf("Key '%s' is not valid in the selected configuration structure. Use '%s' instead. Run 'gitversion config migrate' to convert a legacy configuration.", normalized, replacement)
	}
	return fmt.Sprintf("Unsupported key '%s'.", normalized)
}

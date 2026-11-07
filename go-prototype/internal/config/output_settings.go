package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type OutputSettings struct {
	AssemblyScheme, AssemblyFileScheme string
	AssemblyFormat, AssemblyFileFormat string
	InformationalFormat, CustomFormat  string
	CommitDateFormat                   *string
}

var outputSettingNames = []string{
	"assembly-versioning-scheme", "assembly-file-versioning-scheme",
	"assembly-versioning-format", "assembly-file-versioning-format",
	"assembly-informational-format", "custom-version-format", "commit-date-format",
}

func parseOutputSettings(fields map[string]*yaml.Node, out *OutputSettings) error {
	for _, name := range outputSettingNames {
		node := fields[name]
		if node == nil || node.Tag == "!!null" {
			continue
		}
		value, err := scalar(node, "output."+name)
		if err != nil {
			return err
		}
		switch name {
		case "assembly-versioning-scheme", "assembly-file-versioning-scheme":
			switch value {
			case "None", "Major", "MajorMinor", "MajorMinorPatch", "MajorMinorPatchTag":
			default:
				return fmt.Errorf("unsupported output.%s %q", name, value)
			}
			if name == "assembly-versioning-scheme" {
				out.AssemblyScheme = value
			} else {
				out.AssemblyFileScheme = value
			}
		case "assembly-versioning-format":
			out.AssemblyFormat = value
		case "assembly-file-versioning-format":
			out.AssemblyFileFormat = value
		case "assembly-informational-format":
			out.InformationalFormat = value
		case "custom-version-format":
			out.CustomFormat = value
		case "commit-date-format":
			out.CommitDateFormat = &value
		}
	}
	return nil
}

func (c Config) OutputFor(kind string) OutputSettings {
	output := c.Output
	if format := c.Options(kind).CustomVersionFormat; format != nil {
		output.CustomFormat = *format
	}
	return output
}

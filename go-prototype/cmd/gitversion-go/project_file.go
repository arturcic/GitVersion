package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func updateProjectFiles(directory string, paths []string, values map[string]any, diagnostics io.Writer) error {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(paths, func(path string) bool { return strings.TrimSpace(path) != "" }) {
		paths = nil
		const maxDepth = 255
		err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				relative, err := filepath.Rel(directory, path)
				if err != nil {
					return err
				}
				if strings.Count(relative, string(filepath.Separator)) >= maxDepth {
					return filepath.SkipDir
				}
				return nil
			}
			switch filepath.Ext(path) {
			case ".csproj", ".fsproj", ".vbproj", ".sqlproj":
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	var changes []fileChange
	seen := map[string]bool{}
	for _, name := range paths {
		path := resolveMigrationPath(directory, name)
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) || (err == nil && info.IsDir()) {
			fmt.Fprintf(diagnostics, "Specified file %s was not found and will not be updated.\n", path)
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("project file is not a regular file: %s", path)
		}
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		if seen[target] {
			continue
		}
		seen[target] = true
		raw, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		content := assemblyInfoText(raw)
		project, err := parseProjectXML(content)
		if err != nil {
			return fmt.Errorf("Unable to parse file as xml: %s: %w", path, err)
		}
		groups, warning, err := updatableProjectGroups(project)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if warning != "" {
			fmt.Fprintf(diagnostics, "%s\nUnable to update file: %s\n", warning, path)
			continue
		}
		for _, item := range []struct{ element, variable string }{
			{"AssemblyVersion", "AssemblySemVer"},
			{"FileVersion", "AssemblySemFileVer"},
			{"InformationalVersion", "InformationalVersion"},
			{"Version", "SemVer"},
		} {
			value, _ := values[item.variable].(string)
			if strings.TrimSpace(value) != "" {
				setProjectVersionElement(groups, item.element, value)
			}
		}
		updated := project.render()
		if updated != content {
			changes = append(changes, fileChange{target, updated, raw, info})
		}
	}
	return applyFileChanges(changes)
}

func updatableProjectGroups(project *projectXML) ([]*projectXML, string, error) {
	const sdkHelp = "please ensure the project sdk starts with 'Microsoft.NET.Sdk' or 'Microsoft.Build.Sql'"
	if !project.matches("Project") {
		return nil, "Invalid project file specified, root element must be <Project>.", nil
	}
	sdk := slices.IndexFunc(project.attrs, func(a xml.Attr) bool { return a.Name == (xml.Name{Local: "Sdk"}) })
	if sdk < 0 {
		return nil, "Project file does not specify a Sdk attribute, " + sdkHelp, nil
	}
	value := project.attrs[sdk].Value
	if !strings.HasPrefix(value, "Microsoft.NET.Sdk") && !strings.HasPrefix(value, "Microsoft.Build.Sql") {
		return nil, fmt.Sprintf("Specified project file Sdk (%s) is not supported, %s", value, sdkHelp), nil
	}

	var groups []*projectXML
	project.collectPropertyGroups(&groups)
	if len(groups) == 0 {
		return nil, "Unable to locate any <PropertyGroup> elements in specified project file. Are you sure it is in a correct format?", nil
	}
	var generate *projectXML
	for _, group := range groups {
		if last := group.lastElement("GenerateAssemblyInfo"); last != nil {
			generate = last
		}
	}
	if generate != nil {
		value := strings.ToLower(strings.TrimSpace(generate.value()))
		switch value {
		case "false", "0":
			return nil, "Project file specifies <GenerateAssemblyInfo>false</GenerateAssemblyInfo>: versions set in this project file will not affect the output artifacts.", nil
		case "true", "1":
		default:
			return nil, "", fmt.Errorf("invalid GenerateAssemblyInfo boolean %q", value)
		}
	}
	return groups, "", nil
}

func setProjectVersionElement(groups []*projectXML, name, value string) {
	for i := len(groups) - 1; i >= 0; i-- {
		if existing := groups[i].lastElement(name); existing != nil {
			existing.content = []projectXMLContent{{token: xml.CharData(value)}}
			return
		}
	}
	groups[0].content = append(groups[0].content, projectXMLContent{element: &projectXML{
		name: xml.Name{Local: name}, content: []projectXMLContent{{token: xml.CharData(value)}},
	}})
}

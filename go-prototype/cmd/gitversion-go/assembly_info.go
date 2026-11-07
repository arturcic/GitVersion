package main

import (
	"bytes"
	"embed"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/GitTools/GitVersion/go-prototype/internal/patterns"
)

//go:embed assemblyinfo/VersionAssemblyInfo.*
var assemblyInfoTemplates embed.FS

var assemblyInfoFormats = map[string]struct {
	prefix, suffix, attributePattern string
}{
	".cs": {"[assembly: ", "]", "Output.CsharpAssemblyAttributeRegex"},
	".fs": {"[<assembly: ", ">]", "Output.FsharpAssemblyAttributeRegex"},
	".vb": {"<Assembly: ", ">", "Output.VisualBasicAssemblyAttributeRegex"},
}

func updateAssemblyInfo(directory string, paths []string, ensure bool, values map[string]any, diagnostics io.Writer) error {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(paths, func(path string) bool { return strings.TrimSpace(path) != "" }) {
		paths = nil
		err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if _, supported := assemblyInfoFormats[strings.ToLower(filepath.Ext(path))]; supported &&
				!entry.IsDir() && strings.HasPrefix(entry.Name(), "AssemblyInfo.") {
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
		var original []byte
		content := ""
		extension := filepath.Ext(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			if !ensure {
				continue
			}
			data, readErr := assemblyInfoTemplates.ReadFile("assemblyinfo/VersionAssemblyInfo" + strings.ToLower(extension))
			if errors.Is(readErr, fs.ErrNotExist) {
				fmt.Fprintf(diagnostics, "No version assembly info template available to create source file '%s'\n", path)
				continue
			}
			if readErr != nil {
				return readErr
			}
			content = string(data)
		case err != nil:
			return err
		case info.IsDir():
			continue
		case !info.Mode().IsRegular():
			return fmt.Errorf("assembly info file is not a regular file: %s", path)
		default:
			path, err = filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			original, err = os.ReadFile(path)
			if err != nil {
				return err
			}
			content = assemblyInfoText(original)
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		updated, err := replaceAssemblyInfo(content, extension, values)
		if err != nil {
			return err
		}
		if info == nil || updated != content {
			changes = append(changes, fileChange{path, updated, original, info})
		}
	}

	return applyFileChanges(changes)
}

// .NET ReadAllText recognizes Unicode BOMs and writes changed text as UTF-8.
func assemblyInfoText(data []byte) string {
	var order binary.ByteOrder
	var width int
	switch {
	case bytes.HasPrefix(data, []byte{0xff, 0xfe, 0, 0}):
		order, width = binary.LittleEndian, 4
	case bytes.HasPrefix(data, []byte{0, 0, 0xfe, 0xff}):
		order, width = binary.BigEndian, 4
	case bytes.HasPrefix(data, []byte{0xff, 0xfe}):
		order, width = binary.LittleEndian, 2
	case bytes.HasPrefix(data, []byte{0xfe, 0xff}):
		order, width = binary.BigEndian, 2
	default:
		return strings.TrimPrefix(string([]rune(string(data))), "\ufeff")
	}

	data = data[width:]
	count := len(data) / width
	var decoded []rune
	if width == 2 {
		units := make([]uint16, count)
		for i := range units {
			units[i] = order.Uint16(data[i*width:])
		}
		decoded = utf16.Decode(units)
	} else {
		decoded = make([]rune, count)
		for i := range decoded {
			codePoint := rune(order.Uint32(data[i*width:]))
			if !utf8.ValidRune(codePoint) {
				codePoint = utf8.RuneError
			}
			decoded[i] = codePoint
		}
	}
	if len(data)%width != 0 {
		decoded = append(decoded, utf8.RuneError)
	}
	return string(decoded)
}

func createAssemblyInfo(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return err
	}
	_, err = file.WriteString(content)
	if err = errors.Join(err, file.Close()); err != nil {
		return errors.Join(err, os.Remove(path))
	}
	return nil
}

func replaceAssemblyInfo(content, extension string, values map[string]any) (string, error) {
	const newline = "\r\n"
	language, supported := assemblyInfoFormats[strings.ToLower(extension)]
	var attributes *patterns.Pattern
	if supported && extension == strings.ToLower(extension) {
		var err error
		attributes, err = patterns.Builtin(language.attributePattern)
		if err != nil {
			return "", err
		}
	}
	appended := false
	for _, item := range []struct{ attribute, variable string }{
		{"AssemblyVersion", "AssemblySemVer"},
		{"AssemblyFileVersion", "AssemblySemFileVer"},
		{"AssemblyInformationalVersion", "InformationalVersion"},
	} {
		value, _ := values[item.variable].(string)
		if strings.TrimSpace(value) == "" {
			continue
		}
		expression, err := patterns.Compile(item.attribute+`(Attribute)?\s*\(.*\)\s*`, patterns.IgnoreCase)
		if err != nil {
			return "", err
		}
		replacement := item.attribute + `("` + value + `")`
		match, err := expression.Find(content)
		if err != nil {
			return "", err
		}
		if match != nil {
			content, err = expression.Replace(content, replacement, -1, -1)
			if err != nil {
				return "", err
			}
			continue
		}
		formatted := language.prefix + replacement + language.suffix
		if attributes != nil {
			matches, err := attributes.FindAll(content)
			if err != nil {
				return "", err
			}
			if len(matches) != 0 {
				last := matches[len(matches)-1].Value
				insert := last
				if !strings.HasSuffix(insert, newline) {
					insert += newline
				}
				content = strings.ReplaceAll(content, last, insert+formatted+newline)
				continue
			}
		}
		if supported {
			content += newline + formatted
		}
		appended = true
	}
	if appended {
		content += newline
	}
	return content, nil
}

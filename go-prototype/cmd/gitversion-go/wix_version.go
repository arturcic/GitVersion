package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/GitTools/GitVersion/go-prototype/internal/buildagent"
	"github.com/GitTools/GitVersion/go-prototype/internal/output"
)

const wixVersionFileName = "GitVersion_WixVersion.wxi"

func updateWixVersionFile(directory string, values map[string]any) error {
	var document strings.Builder
	document.WriteString("\ufeff<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
	document.WriteString("<Include xmlns=\"http://schemas.microsoft.com/wix/2006/wi\">\n")
	newlines := strings.NewReplacer("\r\n", "\n", "\r", "\n")
	for _, name := range slices.Sorted(slices.Values(output.VariableNames)) {
		value := newlines.Replace(buildagent.ValueText(values[name]))
		if strings.Contains(value, "?>") || strings.ContainsFunc(value, invalidXMLRune) {
			return fmt.Errorf("invalid WiX definition value for %s", name)
		}
		fmt.Fprintf(&document, "  <?define %s=\"%s\"?>\n", name, value)
	}
	document.WriteString("</Include>")
	if err := os.MkdirAll(directory, 0777); err != nil {
		return err
	}
	path := filepath.Join(directory, wixVersionFileName)
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return writeFileAtomically(path, nativeText([]byte(document.String())), info, true)
}

func invalidXMLRune(r rune) bool {
	return (r < ' ' && r != '\t' && r != '\n' && r != '\r') || r == '\ufffe' || r == '\uffff'
}

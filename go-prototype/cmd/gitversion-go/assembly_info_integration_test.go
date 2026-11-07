//go:build platform

package main

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

func TestPlatformAssemblyInfoParity(t *testing.T) {
	var cases []fileUpdaterCase
	for _, ext := range []string{".cs", ".fs", ".vb"} {
		language := assemblyInfoFormats[ext]
		existing := language.prefix + `AssemblyFileVersion("0.0.0.0")` + language.suffix + "\r\n"
		cases = append(cases,
			fileUpdaterCase{name: "discover" + ext, path: "AssemblyInfo" + ext, original: existing},
			fileUpdaterCase{name: "explicit" + ext, path: "VersionInfo" + ext, original: existing, explicit: true},
			fileUpdaterCase{name: "ensure" + ext, path: "GeneratedInfo" + ext, explicit: true, ensure: true},
		)
	}
	utf16File := []byte{0xff, 0xfe}
	for _, unit := range utf16.Encode([]rune("[assembly: AssemblyFileVersion(\"0.0.0.0\")] // ✓\r\n")) {
		utf16File = binary.LittleEndian.AppendUint16(utf16File, unit)
	}
	cases = append(cases,
		fileUpdaterCase{name: "utf16", path: "AssemblyInfo.cs", original: string(utf16File)},
		fileUpdaterCase{name: "utf8-bom", path: "AssemblyInfo.cs", original: "\ufeff[assembly: AssemblyFileVersion(\"0.0.0.0\")]\r\n"},
	)
	testFileUpdaterParity(t, "assembly-info", "--update-assembly-info", cases)
}

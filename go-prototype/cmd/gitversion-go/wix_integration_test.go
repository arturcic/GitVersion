//go:build platform

package main

import (
	"strings"
	"testing"
)

func TestPlatformWixParity(t *testing.T) {
	testFileUpdaterParity(t, "wix-output", "--update-wix-version-file", []fileUpdaterCase{
		{name: "create-at-tag", path: wixVersionFileName},
		{name: "replace-after-tag", path: wixVersionFileName, original: strings.Repeat("x", 1024*1024), afterTag: true},
	})
}

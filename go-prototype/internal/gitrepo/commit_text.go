package gitrepo

import (
	"strings"
	"unicode/utf8"
)

// Git commit text defaults to UTF-8 with a Latin-1 fallback. An explicit
// Latin-1 header takes precedence even when its bytes happen to be valid UTF-8.
func decodeCommitText(value, encoding string) string {
	latin1 := false
	switch strings.ToLower(encoding) {
	case "iso-8859-1", "iso_8859-1", "latin1", "latin-1", "l1":
		latin1 = true
	}
	if !latin1 && utf8.ValidString(value) {
		return value
	}
	var decoded strings.Builder
	decoded.Grow(len(value))
	for i := 0; i < len(value); i++ {
		decoded.WriteRune(rune(value[i]))
	}
	return decoded.String()
}

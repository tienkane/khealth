// Package text sanitises untrusted strings (check details, command output)
// before they are printed, so hostile content can't corrupt the terminal.
package text

import (
	"strings"
	"unicode"
)

// Sanitize strips control characters (C0, DEL, C1) and Unicode format/bidi
// characters, and trims surrounding whitespace.
func Sanitize(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r < 0x20, r >= 0x7f && r <= 0x9f:
			return -1
		case unicode.Is(unicode.Cf, r):
			return -1
		default:
			return r
		}
	}, s)
	return strings.TrimSpace(s)
}

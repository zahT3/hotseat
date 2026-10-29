// Package shellq quotes strings for POSIX shells.
package shellq

import (
	"regexp"
	"strings"
)

var plain = regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,-]+$`)

// Quote returns s as one shell word: unchanged when it holds only safe
// characters, otherwise single-quoted with ' written as '\”.
func Quote(s string) string {
	if s != "" && plain.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

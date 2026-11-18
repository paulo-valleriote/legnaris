package generation

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func SanitizeConcept(s string) string {
	return strings.TrimSpace(stripConceptRunes(norm.NFKC.String(s)))
}

func stripConceptRunes(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n':
			b.WriteRune(r)
		case r == '\r':
			b.WriteRune('\n')
		case r == '\t':
			b.WriteRune(' ')
		case unicode.In(r, unicode.Cf, unicode.Co, unicode.Cs, unicode.Cc):
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

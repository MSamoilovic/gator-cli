package text

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

var (
	blockTagRe  = regexp.MustCompile(`(?i)</?(br|p|div|li|tr|h[1-6])[^>]*>`)
	anyTagRe    = regexp.MustCompile(`<[^>]*>`)
	blankLineRe = regexp.MustCompile(`\n{3,}`)
)

func StripHTML(s string) string {
	s = blockTagRe.ReplaceAllString(s, "\n")
	s = anyTagRe.ReplaceAllString(s, "")
	s = Printable(html.UnescapeString(s))
	s = blankLineRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func OneLine(s string) string {
	return strings.Join(strings.Fields(Printable(s)), " ")
}

func Printable(s string) string {
	return strings.Map(func(r rune) rune {
		if hidden(r) {
			return -1
		}
		return r
	}, s)
}

func hidden(r rune) bool {
	switch {
	case r == '\n', r == '\t':
		return false
	case r < 0x20, r == 0x7f:
		return true
	case r >= 0x80 && r <= 0x9f:
		return true
	case r >= 0x202a && r <= 0x202e:
		return true
	case r >= 0x2066 && r <= 0x2069:
		return true
	}
	return false
}

func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}

	s = Printable(s)
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}

	cut := runes[:max]
	if i := lastSpace(cut); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(string(cut), " \t\n") + "…"
}

func lastSpace(runes []rune) int {
	for i := len(runes) - 1; i >= 0; i-- {
		if unicode.IsSpace(runes[i]) {
			return i
		}
	}
	return -1
}

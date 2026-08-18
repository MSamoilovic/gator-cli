// Package text sredjuje tekst posta za prikaz. Feedovi salju HTML, a i TUI i
// CLI ispis treba da vide isti ociscen tekst — zato ovo ne zivi u nijednom od
// njih.
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

// StripHTML izbaci markup i dekodira entitete. Blok tagovi postaju novi red da
// pasusi ne bi bili slepljeni, ostali se samo brisu.
func StripHTML(s string) string {
	s = blockTagRe.ReplaceAllString(s, "\n")
	s = anyTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = blankLineRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// Truncate skrati na najvise max znakova i doda "…". Rez ide na granici reci,
// osim ako je prva rec duza od celog ogranicenja. Meri se u znakovima, ne u
// bajtovima — inace bi rez umeo da raspolovi ćirilicu ili emoji.
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}

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

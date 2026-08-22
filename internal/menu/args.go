package menu

import (
	"strings"
	"unicode"
)

// SplitArgs deli otkucanu liniju na argumente, postujuci navodnike, da bi
// `addfeed "Hacker News" https://news.ycombinator.com/rss` proslo kao dva
// argumenta a ne kao tri. Prazna linija daje nil, ne listu sa praznim
// argumentom — handleri granaju na len(cmd.Args).
func SplitArgs(line string) []string {
	var (
		args   []string
		cur    strings.Builder
		quote  rune
		quoted bool // tekuci argument je postojao makar kao ""
	)

	flush := func() {
		if cur.Len() > 0 || quoted {
			args = append(args, cur.String())
			cur.Reset()
			quoted = false
		}
	}

	for _, r := range line {
		switch {
		case quote != 0:
			// Unutar navodnika sve je tekst, pa i razmak; zatvara ga samo
			// isti navodnik kojim je otvoren.
			if r == quote {
				quote = 0
				continue
			}
			cur.WriteRune(r)

		case r == '"' || r == '\'':
			quote = r
			quoted = true

		case unicode.IsSpace(r):
			flush()

		default:
			cur.WriteRune(r)
		}
	}
	flush()

	return args
}

package menu

import (
	"strings"
	"unicode"
)

func SplitArgs(line string) []string {
	var (
		args   []string
		cur    strings.Builder
		quote  rune
		quoted bool
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

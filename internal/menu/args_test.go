package menu

import "testing"

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{"prazna linija", "", nil},
		{"samo razmaci", "   ", nil},
		{"jedan argument", "boot.dev", []string{"boot.dev"}},
		{"dva argumenta", "tech sport", []string{"tech", "sport"}},
		{"visak razmaka", "  a   b  ", []string{"a", "b"}},
		{"navodnici drze razmak", `"Hacker News" https://hn/rss`, []string{"Hacker News", "https://hn/rss"}},
		{"jednostruki navodnici", `'Ars Technica' url`, []string{"Ars Technica", "url"}},
		{"navodnici usred reci", `--feed="boot dev"`, []string{"--feed=boot dev"}},
		{"prazan argument u navodnicima", `url ""`, []string{"url", ""}},
		{"apostrof unutar navodnika", `"it's here"`, []string{"it's here"}},
		{"nezatvoreni navodnik uzima ostatak", `"boot dev`, []string{"boot dev"}},
		{"folder sa razmakom", `https://feed "Daily reads"`, []string{"https://feed", "Daily reads"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitArgs(tt.line)
			if len(got) != len(tt.want) {
				t.Fatalf("SplitArgs(%q) = %q, want %q", tt.line, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SplitArgs(%q)[%d] = %q, want %q", tt.line, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSplitArgsEmptyLineIsNil(t *testing.T) {
	// Handleri granaju na len(cmd.Args), pa prazna linija ne sme da im
	// podmetne jedan prazan argument.
	if got := SplitArgs("  "); got != nil {
		t.Errorf("SplitArgs(spaces) = %q, want nil", got)
	}
}

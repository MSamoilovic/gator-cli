package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func onServer(t *testing.T, server string, width int) model {
	t.Helper()
	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc}, server),
		tea.WindowSizeMsg{Width: width, Height: 24})
	m, _ = step(t, m, postsLoadedMsg{posts: densePosts(4)})
	return m
}

func TestServerHostDropsTheSchemeAndThenThePort(t *testing.T) {
	cases := map[string]string{
		"https://reader.example.com":      "reader.example.com",
		"http://localhost:8099":           "localhost:8099",
		"https://reader.example.com:8443": "reader.example.com:8443",
		"localhost:8099":                  "localhost:8099",
		"":                                "",
	}
	for server, want := range cases {
		if got := serverHost(server); got != want {
			t.Errorf("serverHost(%q) = %q, want %q", server, got, want)
		}
	}

	if got, want := hostWithoutPort("localhost:8099"), "localhost"; got != want {
		t.Errorf("hostWithoutPort = %q, want %q", got, want)
	}
	if got, want := hostWithoutPort("reader.example.com"), "reader.example.com"; got != want {
		t.Errorf("hostWithoutPort left a portless host alone as %q, want %q", got, want)
	}
}

func TestALocalSessionOffersOnlyTheUserTag(t *testing.T) {
	m := onServer(t, "", 80)

	if got, want := m.footerTags(), 1; len(got) != want {
		t.Fatalf("footerTags() = %v, want exactly the user tag", got)
	}
	if got, want := m.footerTags()[0], "@marko"; got != want {
		t.Errorf("tag = %q, want %q", got, want)
	}
	if strings.Contains(m.footer(), remoteMark) {
		t.Error("a local session claims to be talking to a server")
	}
}

func TestAWideTerminalNamesTheServerInTheFooter(t *testing.T) {
	m := onServer(t, "http://localhost:8099", 120)

	footer := m.footer()
	if !strings.Contains(footer, "localhost:8099") {
		t.Errorf("footer = %q, want the server named at 120 columns — remote mode was invisible "+
			"until something went wrong", footer)
	}
	if !strings.Contains(footer, "@marko") {
		t.Error("naming the server cost the user tag")
	}
}

func TestANarrowTerminalKeepsTheUserAndMarksTheSessionRemote(t *testing.T) {
	m := onServer(t, "http://localhost:8099", 80)

	footer := m.footer()
	if !strings.Contains(footer, "@marko") {
		t.Fatalf("footer = %q, want the user tag kept at 80 columns", footer)
	}
	if strings.Contains(footer, "localhost") {
		t.Errorf("footer = %q — the full server name does not fit 80 columns next to the help line", footer)
	}
	if !strings.Contains(footer, remoteMark) {
		t.Errorf("footer = %q, want %q as the narrow stand-in for the server name", footer, remoteMark)
	}
}

func TestTheFooterNeverSpillsPastTheTerminal(t *testing.T) {
	for _, server := range []string{"", "http://localhost:8099", "https://reader.example.com:8443"} {
		for _, width := range []int{60, 80, 100, 120} {
			m := onServer(t, server, width)
			if got := lipgloss.Width(m.footer()); got > width {
				t.Errorf("footer for server %q at %d columns is %d wide: %q", server, width, got, m.footer())
			}
		}
	}
}

func TestEveryFooterTagVariantIsNarrowerThanTheOneBefore(t *testing.T) {
	m := onServer(t, "https://reader.example.com:8443", 120)

	tags := m.footerTags()
	if len(tags) < 3 {
		t.Fatalf("footerTags() = %v, want a sequence to fall back through", tags)
	}
	for i := 1; i < len(tags); i++ {
		if lipgloss.Width(tags[i]) >= lipgloss.Width(tags[i-1]) {
			t.Errorf("tag %d (%q) is not narrower than tag %d (%q), so the fallback never helps",
				i, tags[i], i-1, tags[i-1])
		}
	}
	if got, want := tags[len(tags)-1], "@marko"; got != want {
		t.Errorf("last resort tag = %q, want the bare %q", got, want)
	}
}

func TestAnAnonymousSessionGetsNoTagAtAll(t *testing.T) {
	m := onServer(t, "http://localhost:8099", 120)
	m.userName = ""

	if got := m.footerTags(); got != nil {
		t.Errorf("footerTags() = %v, want none without a user name", got)
	}
	if got := m.userTagged("help"); got != "help" {
		t.Errorf("userTagged = %q, want the line untouched", got)
	}
}

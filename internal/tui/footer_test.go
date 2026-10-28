package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/store"

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

type fakeSyncer struct{ state store.SyncState }

func (f fakeSyncer) State() store.SyncState { return f.state }

func (f fakeSyncer) Sync(context.Context) (store.SyncResult, error) {
	return store.SyncResult{}, nil
}

func syncing(t *testing.T, width int, state store.SyncState) model {
	t.Helper()
	m := onServer(t, "http://localhost:8099", width)
	m.sync = fakeSyncer{state: state}
	return m
}

func TestAWideTerminalSaysTheSessionIsOfflineAndHowOldTheCacheIs(t *testing.T) {
	m := syncing(t, 120, store.SyncState{LastSync: time.Date(2026, 10, 28, 14, 32, 0, 0, time.Local)})

	footer := m.footer()
	if !strings.Contains(footer, "offline") {
		t.Errorf("footer = %q, want it to say the session is offline", footer)
	}
	if !strings.Contains(footer, "sync 14:32") {
		t.Errorf("footer = %q, want the time the cache was last filled — offline it is the only thing that dates what you see", footer)
	}
	if strings.Contains(footer, "localhost") {
		t.Error("the footer names the server while nothing can reach it")
	}
}

func TestAtEightyColumnsOfflineShrinksToTheWarningMark(t *testing.T) {
	m := syncing(t, 80, store.SyncState{})

	footer := m.footer()
	if strings.Contains(footer, "offline") {
		t.Errorf("footer = %q — %q is 16 columns and the post list leaves 13, so the word cannot be there",
			footer, "@marko · offline")
	}
	if !strings.Contains(footer, brokenMark) {
		t.Errorf("footer = %q, want %q as the narrow stand-in — dropping to the bare user tag would hide being offline", footer, brokenMark)
	}
	if !strings.Contains(footer, "@marko") {
		t.Error("the mark cost the user tag")
	}
}

func TestQueuedWritesAreCountedInTheFooterAtEightyColumns(t *testing.T) {
	m := syncing(t, 80, store.SyncState{Pending: 3})

	if got, want := m.footer(), "@marko · 3 "+remoteMark; !strings.Contains(got, want) {
		t.Errorf("footer = %q, want %q — 12 columns fits the 13 the post list leaves", got, want)
	}
}

func TestLostWritesOutrankBeingOfflineInTheFooter(t *testing.T) {
	m := syncing(t, 120, store.SyncState{Pending: 3, Dropped: 2})

	footer := m.footer()
	if !strings.Contains(footer, "2 lost") {
		t.Errorf("footer = %q, want the overflowed changes named — a silent loss is worse than an admitted one", footer)
	}

	narrow := syncing(t, 80, store.SyncState{Pending: 3, Dropped: 2})
	if got, want := narrow.footer(), "@marko · 2 "+brokenMark; !strings.Contains(got, want) {
		t.Errorf("footer = %q, want %q: %q is 15 columns and the post list leaves 13, so the count "+
			"keeps the loud mark instead of the word", got, want, "@marko · 2 lost")
	}
	if strings.Contains(narrow.footer(), remoteMark) {
		t.Error("the waiting count took the slot a lost write had earned")
	}
}

func TestAnOnlineSessionWithWritesWaitingNamesTheServerAndTheCount(t *testing.T) {
	m := syncing(t, 120, store.SyncState{Online: true, Pending: 2})

	footer := m.footer()
	for _, want := range []string{"localhost:8099", "2 " + remoteMark} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer = %q, want %q", footer, want)
		}
	}
	if strings.Contains(footer, brokenMark) {
		t.Error("a reachable server is marked as broken")
	}
}

func TestEveryTagLadderNarrowsAtEveryRungInEveryState(t *testing.T) {
	states := map[string]store.SyncState{
		"online":                {Online: true},
		"online, 2 waiting":     {Online: true, Pending: 2},
		"online, 2 lost":        {Online: true, Dropped: 2},
		"offline":               {},
		"offline, synced once":  {LastSync: time.Now()},
		"offline, 3 waiting":    {Pending: 3},
		"offline, 3 and 2 lost": {Pending: 3, Dropped: 2},
	}

	for name, state := range states {
		m := syncing(t, 120, state)
		tags := m.footerTags()
		for i := 1; i < len(tags); i++ {
			if lipgloss.Width(tags[i]) >= lipgloss.Width(tags[i-1]) {
				t.Errorf("%s: tag %d (%q) is not narrower than tag %d (%q), so the fallback never helps",
					name, i, tags[i], i-1, tags[i-1])
			}
		}
		if got, want := tags[len(tags)-1], "@marko"; got != want {
			t.Errorf("%s: last resort tag = %q, want the bare %q", name, got, want)
		}
	}
}

func TestEveryTagVariantLeavesTheNarrowestFooterIntact(t *testing.T) {
	states := []store.SyncState{
		{Online: true}, {Online: true, Pending: 2}, {Online: true, Dropped: 2},
		{}, {LastSync: time.Now()}, {Pending: 3}, {Pending: 3, Dropped: 2},
	}

	for _, state := range states {
		for _, width := range []int{60, 80, 100, 120} {
			m := syncing(t, width, state)
			m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
			for _, line := range strings.Split(m.View(), "\n") {
				if got := lipgloss.Width(line); got > width {
					t.Fatalf("state %+v at %d columns renders %d: %q", state, width, got, line)
				}
			}
		}
	}
}

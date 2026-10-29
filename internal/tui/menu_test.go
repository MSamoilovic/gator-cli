package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/menu"
	"github.com/MSamoilovic/gator-cli/internal/store"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func readerCommands() []menu.Item {
	return []menu.Item{
		{Group: "feeds", Name: "feeds", Summary: "List every feed in the database"},
		{Group: "feeds", Name: "stats", Args: "[flags]", Summary: "Which feeds you actually read"},
		{Group: "feeds", Name: "export", Args: "[file]", Summary: "Write your subscriptions out as OPML"},
		{Group: "aggregation", Name: "agg", Args: "<duration>", Summary: "Fetch every feed in a loop"},
		{Group: "account", Name: "logout", Summary: "Disconnect from a server"},
	}
}

func withMenu(t *testing.T, width int) model {
	t.Helper()
	m := ready(t, testPost("Prvi"))
	m.commands = readerCommands()
	m, _ = step(t, m, tea.WindowSizeMsg{Width: width, Height: 24})
	return m
}

func TestTheMenuOpensOnM(t *testing.T) {
	m, _ := step(t, withMenu(t, 80), press("m"))

	if m.screen != screenMenu {
		t.Fatalf("screen = %v after m, want the menu", m.screen)
	}
	view := m.View()
	for _, want := range []string{"agg", "Fetch every feed in a loop", "aggregation"} {
		if !strings.Contains(view, want) {
			t.Errorf("menu view = %q, want it to contain %q", view, want)
		}
	}
}

func TestTheMenuSaysWhatItIsForAndWhoIsSignedIn(t *testing.T) {
	m, _ := step(t, withMenu(t, 80), press("m"))

	view := m.View()
	if !strings.Contains(view, "@marko") {
		t.Error("the menu does not say who is signed in, and the reader's footer is gone on this screen")
	}
	if !strings.Contains(view, "cannot do itself") {
		t.Errorf("menu view = %q, want it to explain why search and bookmark are missing", view)
	}
}

func TestPickingACommandLeavesTheReaderCarryingTheChoice(t *testing.T) {
	m, _ := step(t, withMenu(t, 80), press("m"))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if !m.chose {
		t.Fatal("enter on a command did not choose it")
	}
	if got, want := m.choice.Name, "feeds"; got != want {
		t.Errorf("chose %q, want %q", got, want)
	}
}

func TestACommandThatTakesArgumentsAsksBeforeLeaving(t *testing.T) {
	m, _ := step(t, withMenu(t, 80), press("m"))
	for range 3 {
		m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.chose {
		t.Fatal("agg was chosen before the duration it requires was typed")
	}
	if !strings.Contains(m.View(), "gator agg") {
		t.Errorf("view = %q, want the argument prompt", m.View())
	}

	for _, r := range "15m" {
		m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if !m.chose {
		t.Fatal("agg was not chosen after its duration was typed")
	}
	if got, want := strings.Join(m.choice.Args, " "), "15m"; got != want {
		t.Errorf("args = %q, want %q", got, want)
	}
}

func TestEscLeavesTheMenuAndKeepsReading(t *testing.T) {
	m, _ := step(t, withMenu(t, 80), press("m"))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.screen != screenList {
		t.Fatalf("screen = %v after esc, want the post list", m.screen)
	}
	if m.chose {
		t.Error("esc chose a command on the way out")
	}
	if !strings.Contains(m.View(), "Prvi") {
		t.Error("the post list did not come back")
	}
}

func TestAReaderWithNoCommandsToOfferSaysSoInsteadOfOpeningAnEmptyMenu(t *testing.T) {
	m := ready(t, testPost("Prvi"))
	m, _ = step(t, m, press("m"))

	if m.screen == screenMenu {
		t.Fatal("m opened a menu with nothing in it")
	}
	if m.status == "" {
		t.Error("m did nothing at all and said nothing about it")
	}
}

func TestTheMenuScreenNeverSpillsPastTheTerminal(t *testing.T) {
	for _, width := range []int{60, 80, 100, 120} {
		m, _ := step(t, withMenu(t, width), press("m"))
		for _, line := range strings.Split(m.View(), "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Errorf("at %d columns the menu renders a line of %d: %q", width, got, line)
			}
		}
	}
}

func TestTheMenuKnowsItIsOfflineBecauseThatChangesWhatIsWorthRunning(t *testing.T) {
	m := onServer(t, "http://localhost:8099", 120)
	m.commands = readerCommands()
	m.sync = fakeSyncer{state: store.SyncState{LastSync: time.Now()}}
	m, _ = step(t, m, press("m"))

	if !strings.Contains(m.View(), "offline") {
		t.Errorf("menu view = %q, want it to say the session is offline", m.View())
	}
}

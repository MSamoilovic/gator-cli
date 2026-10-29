package menu

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var testConfig = Config{
	Title:    "gator",
	Greeting: "Hello, Tsunami — what would you like to do?",
	Items: []Item{
		{Group: "account", Name: "users", Summary: "List all users"},
		{Group: "account", Name: "login", Args: "<username>", Summary: "Log in"},
		{Group: "reading", Name: "browse", Args: "[flags]", Summary: "Read posts"},
	},
}

func newTestModel(t *testing.T) Model {
	t.Helper()
	m, _ := New(testConfig).Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m
}

func press(t *testing.T, m Model, msgs ...tea.Msg) Model {
	t.Helper()
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	return m
}

func hit(k tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: k} }

func typed(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestSelectingCommandWithoutArgs(t *testing.T) {
	m := press(t, newTestModel(t), hit(tea.KeyEnter))

	if !m.chosen {
		t.Fatal("enter on a no-arg command did not choose it")
	}
	if m.choice.Name != "users" {
		t.Errorf("chose %q, want users", m.choice.Name)
	}
	if m.choice.Args != nil {
		t.Errorf("args = %q, want nil", m.choice.Args)
	}
	if m.asking {
		t.Error("a command that takes no args should not open the prompt")
	}
}

func TestCommandWithArgsOpensPrompt(t *testing.T) {
	m := press(t, newTestModel(t), hit(tea.KeyDown), hit(tea.KeyEnter))

	if m.chosen {
		t.Fatal("chose the command before its args were given")
	}
	if !m.asking {
		t.Fatal("enter on login did not open the argument prompt")
	}
	if m.pending.Name != "login" {
		t.Errorf("pending = %q, want login", m.pending.Name)
	}
	if !strings.Contains(m.prompt.Prompt, "login") {
		t.Errorf("prompt = %q, want it to name the command", m.prompt.Prompt)
	}
}

func TestTypedArgsReachTheChoice(t *testing.T) {
	m := press(t, newTestModel(t),
		hit(tea.KeyDown), hit(tea.KeyEnter),
		typed("mako"), hit(tea.KeyEnter),
	)

	if !m.chosen {
		t.Fatal("enter after typing args did not choose the command")
	}
	if m.choice.Name != "login" {
		t.Errorf("chose %q, want login", m.choice.Name)
	}
	if len(m.choice.Args) != 1 || m.choice.Args[0] != "mako" {
		t.Errorf("args = %q, want [mako]", m.choice.Args)
	}
}

func TestRequiredArgsCannotBeSkipped(t *testing.T) {
	m := press(t, newTestModel(t),
		hit(tea.KeyDown), hit(tea.KeyEnter),
		hit(tea.KeyEnter),
	)

	if m.chosen {
		t.Fatal("login was chosen without the username it requires")
	}
	if !m.asking {
		t.Error("prompt closed instead of waiting for the argument")
	}
	if !strings.Contains(m.status, "<username>") {
		t.Errorf("status = %q, want it to say what is missing", m.status)
	}
}

func TestOptionalArgsMayBeSkipped(t *testing.T) {
	m := press(t, newTestModel(t),
		hit(tea.KeyDown), hit(tea.KeyDown), hit(tea.KeyEnter),
		hit(tea.KeyEnter),
	)

	if !m.chosen {
		t.Fatal("browse with no flags was not accepted")
	}
	if m.choice.Name != "browse" || m.choice.Args != nil {
		t.Errorf("choice = %+v, want browse with nil args", m.choice)
	}
}

func TestEscLeavesThePromptWithoutChoosing(t *testing.T) {
	m := press(t, newTestModel(t),
		hit(tea.KeyDown), hit(tea.KeyEnter),
		typed("mako"), hit(tea.KeyEsc),
	)

	if m.chosen {
		t.Fatal("esc chose the command anyway")
	}
	if m.asking {
		t.Error("esc did not return to the list")
	}

	if m = press(t, m, hit(tea.KeyEnter)); !m.asking {
		t.Error("the list stopped responding after esc")
	}
}

func TestPromptStartsEmptyOnSecondVisit(t *testing.T) {
	m := press(t, newTestModel(t),
		hit(tea.KeyDown), hit(tea.KeyEnter),
		typed("mako"), hit(tea.KeyEsc),
		hit(tea.KeyEnter),
	)

	if got := m.prompt.Value(); got != "" {
		t.Errorf("prompt kept %q from the previous visit", got)
	}
}

func TestQuittingChoosesNothing(t *testing.T) {
	if m := press(t, newTestModel(t), typed("q")); m.chosen {
		t.Error("q chose a command instead of quitting")
	}
}

func TestFilteringSwallowsQuitKeys(t *testing.T) {
	m := press(t, newTestModel(t), typed("/"), typed("q"))

	if m.chosen {
		t.Fatal("typing in the filter chose a command")
	}
	if got := m.list.FilterInput.Value(); got != "q" {
		t.Errorf("filter = %q, want q to have been typed into it", got)
	}
}

func TestItemNeedsArgs(t *testing.T) {
	tests := []struct {
		args string
		want bool
	}{
		{"", false},
		{"<username>", true},
		{"[flags]", false},
		{"[name] <url>", true},
		{"[file]", false},
		{"<url> <folder>", true},
	}

	for _, tt := range tests {
		if got := (Item{Args: tt.args}).needsArgs(); got != tt.want {
			t.Errorf("Item{Args: %q}.needsArgs() = %v, want %v", tt.args, got, tt.want)
		}
	}
}

func TestItemRowIsTheNameThenTheSummary(t *testing.T) {
	got := Item{Name: "login", Args: "<username>", Summary: "Log in"}.Title()
	if strings.Contains(got, "<username>") {
		t.Errorf("Title() = %q — the prompt asks for the arguments, the row does not show them", got)
	}
	if want := "login        Log in"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
	if got := (Item{Name: "users"}).Title(); got != "users" {
		t.Errorf("Title() = %q, want no padding when there is nothing to pad for", got)
	}
}

func TestEverySummaryStartsInTheSameColumn(t *testing.T) {
	first := -1
	for _, name := range []string{"tui", "unbookmark", "agg", "categorize"} {
		title := Item{Name: name, Summary: "x"}.Title()
		at := strings.Index(title, "x")
		if first == -1 {
			first = at
			continue
		}
		if at != first {
			t.Errorf("%q puts its summary at column %d, want %d", name, at, first)
		}
	}
}

func TestFilterValueIsTheRowItself(t *testing.T) {
	it := Item{Name: "import", Group: "feeds", Summary: "Follow everything in an OPML file"}
	if it.FilterValue() != it.Title() {
		t.Errorf("FilterValue() = %q, Title() = %q — list highlights matches of the first inside "+
			"the second, so they must be the same text", it.FilterValue(), it.Title())
	}
	for _, want := range []string{"import", "OPML"} {
		if !strings.Contains(it.FilterValue(), want) {
			t.Errorf("FilterValue() = %q, want it to contain %q", it.FilterValue(), want)
		}
	}
}

func TestViewShowsGreeting(t *testing.T) {
	if !strings.Contains(newTestModel(t).View(), "Hello, Tsunami") {
		t.Error("the greeting is not on screen")
	}
}

func TestViewWithoutGreetingLeavesNoBlankGap(t *testing.T) {
	cfg := testConfig
	cfg.Greeting = ""
	m, _ := New(cfg).Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	if m.greetingHeight() != 0 {
		t.Errorf("greetingHeight() = %d with no greeting, want 0", m.greetingHeight())
	}
	if strings.HasPrefix(m.View(), "\n") {
		t.Error("view starts with a blank line where the greeting would have been")
	}
}

func TestViewShowsPromptOnlyWhileAsking(t *testing.T) {
	m := newTestModel(t)
	if strings.Contains(m.View(), "gator login") {
		t.Error("the argument prompt is visible before a command was picked")
	}

	if m = press(t, m, hit(tea.KeyDown), hit(tea.KeyEnter)); !strings.Contains(m.View(), "gator login") {
		t.Error("the argument prompt is not visible while asking")
	}
}

func realConfig() Config {
	groups := []struct {
		name  string
		count int
	}{{"reading", 7}, {"feeds", 10}, {"aggregation", 5}, {"account", 5}, {"setup", 2}, {"other", 2}}

	var items []Item
	for _, g := range groups {
		for i := range g.count {
			items = append(items, Item{
				Group:   g.name,
				Name:    fmt.Sprintf("%s%d", g.name[:3], i),
				Summary: fmt.Sprintf("summary number %d of the lot", i),
			})
		}
	}
	return Config{Title: "gator", Greeting: "Hello, marko — what would you like to do?", Items: items}
}

func TestEachGroupIsAnnouncedExactlyOnce(t *testing.T) {
	m, _ := New(realConfig()).Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	headers := 0
	for _, row := range m.list.Items() {
		if _, ok := row.(header); ok {
			headers++
		}
	}
	if want := 6; headers != want {
		t.Errorf("%d group rows, want %d — one per group", headers, want)
	}
	if got := len(m.list.Items()); got != 31+6 {
		t.Errorf("%d rows, want 31 commands plus 6 group rows", got)
	}
}

func TestTheCursorNeverRestsOnAGroupRow(t *testing.T) {
	m, _ := New(realConfig()).Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	if _, ok := m.list.SelectedItem().(header); ok {
		t.Fatal("the menu opens with a group row selected, so ⏎ would do nothing")
	}

	for i := range m.list.Items() {
		m = press(t, m, hit(tea.KeyDown))
		if _, ok := m.list.SelectedItem().(header); ok {
			t.Fatalf("%d presses of ↓ landed on group row %q", i+1, m.list.SelectedItem().(header))
		}
	}
	for i := range m.list.Items() {
		m = press(t, m, hit(tea.KeyUp))
		if _, ok := m.list.SelectedItem().(header); ok {
			t.Fatalf("%d presses of ↑ landed on group row %q", i+1, m.list.SelectedItem().(header))
		}
	}
}

func TestEveryCommandIsStillReachableWithTheArrowsAlone(t *testing.T) {
	m, _ := New(realConfig()).Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	seen := map[string]bool{}
	for range m.list.Items() {
		if it, ok := m.list.SelectedItem().(Item); ok {
			seen[it.Name] = true
		}
		m = press(t, m, hit(tea.KeyDown))
	}
	if got, want := len(seen), 31; got != want {
		t.Errorf("↓ alone reached %d of %d commands", got, want)
	}
}

func TestFilteringHidesTheGroupRows(t *testing.T) {
	var (
		targets  []string
		isHeader = map[int]bool{}
	)
	for i, row := range rows(realConfig().Items) {
		targets = append(targets, row.FilterValue())
		if _, ok := row.(header); ok {
			isHeader[i] = true
		}
	}

	ranks := list.DefaultFilter("agg3", targets)
	if len(ranks) == 0 {
		t.Fatal("agg3 matched nothing, so the assertion below proves nothing")
	}
	for _, rank := range ranks {
		if isHeader[rank.Index] {
			t.Errorf("group row %q ranked as a match, so it would sit in a filtered list",
				targets[rank.Index])
		}
	}
}

func TestADenseRowShowsMostOfTheMenuAtEightyColumns(t *testing.T) {
	m, _ := New(realConfig()).Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	if got, want := m.list.Paginator.PerPage, 14; got < want {
		t.Errorf("%d rows fit an 80x24 terminal, want at least %d — the two-line delegate showed "+
			"5 of 31 commands across 7 pages", got, want)
	}

	view := m.View()
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 80 {
			t.Errorf("line is %d columns wide: %q", lipgloss.Width(line), line)
		}
	}
	if !strings.Contains(view, "reading") {
		t.Error("the view names no group — no command is called that, so only a group row can say it")
	}
}

func TestNothingTheMenuDrawsOutgrowsTheTerminal(t *testing.T) {
	cfg := realConfig()
	cfg.Hint = "↑↓ move · ⏎ run it and leave the reader · / filter · esc back"
	cfg.Greeting = "@marko · offline · sync 14:32 — a greeting long enough to be a problem"
	cfg.Items = append(cfg.Items, Item{Group: "other", Name: "categorize", Args: "<url> <folder>",
		Summary: "Move a feed into a folder"})

	for _, width := range []int{40, 60, 80, 120} {
		m, _ := New(cfg).Update(tea.WindowSizeMsg{Width: width, Height: 24})

		m = press(t, m, hit(tea.KeyEnd), hit(tea.KeyEnter))
		if !m.asking {
			t.Fatalf("at %d columns the argument prompt did not open, so it is not being measured", width)
		}

		for _, line := range strings.Split(m.View(), "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Errorf("at %d columns the menu draws a line of %d: %q", width, got, line)
			}
		}
	}
}

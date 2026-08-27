package menu

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func newTestModel(t *testing.T) model {
	t.Helper()
	next, _ := newModel(testConfig).Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return next.(model)
}

func press(t *testing.T, m model, msgs ...tea.Msg) model {
	t.Helper()
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(model)
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

func TestItemTitleShowsArgs(t *testing.T) {
	if got := (Item{Name: "login", Args: "<username>"}).Title(); got != "login <username>" {
		t.Errorf("Title() = %q", got)
	}
	if got := (Item{Name: "users"}).Title(); got != "users" {
		t.Errorf("Title() = %q, want no trailing space", got)
	}
}

func TestFilterValueCoversGroupAndSummary(t *testing.T) {
	got := Item{Name: "import", Group: "feeds", Summary: "Follow everything in an OPML file"}.FilterValue()
	for _, want := range []string{"import", "feeds", "OPML"} {
		if !strings.Contains(got, want) {
			t.Errorf("FilterValue() = %q, want it to contain %q", got, want)
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
	next, _ := newModel(cfg).Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := next.(model)

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

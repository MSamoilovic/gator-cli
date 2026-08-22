package cli

import (
	"strings"
	"testing"

	"gator-cli/internal/database"
)

func TestEveryCommandHasExactlyOneHandler(t *testing.T) {
	for _, e := range allCommands() {
		switch {
		case e.run == nil && e.runAuth == nil:
			t.Errorf("command %q has no handler", e.name)
		case e.run != nil && e.runAuth != nil:
			t.Errorf("command %q sets both run and runAuth", e.name)
		}
	}
}

func TestGuestCommandsDoNotRequireLogin(t *testing.T) {
	// Nudjenje komande izlogovanom korisniku koja ipak trazi prijavu je
	// obecanje koje middleware odmah pokvari.
	for _, e := range allCommands() {
		if e.guest && e.needsLogin() {
			t.Errorf("command %q is offered to guests but requires login", e.name)
		}
	}
}

func TestEveryCommandIsDescribed(t *testing.T) {
	for _, e := range allCommands() {
		if e.summary == "" {
			t.Errorf("command %q has no summary", e.name)
		}
		if e.group == "" {
			t.Errorf("command %q has no group", e.name)
		}
	}
}

func TestCommandNamesAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, e := range allCommands() {
		if seen[e.name] {
			t.Errorf("duplicate command %q", e.name)
		}
		seen[e.name] = true
	}
}

func TestGroupsAreContiguous(t *testing.T) {
	// printUsage stampa zaglavlje kad se grupa promeni, pa razbijena grupa
	// znaci isto zaglavlje dva puta.
	var last string
	seen := make(map[string]bool)
	for _, e := range allCommands() {
		if e.group == last {
			continue
		}
		if seen[e.group] {
			t.Errorf("group %q appears in two separate runs", e.group)
		}
		seen[e.group] = true
		last = e.group
	}
}

func TestDefaultCommandsRegistersEveryEntry(t *testing.T) {
	cmds := defaultCommands()

	for _, e := range allCommands() {
		if _, ok := cmds.registeredCommands[e.name]; !ok {
			t.Errorf("command %q is in the table but was not registered", e.name)
		}
	}
	if got, want := len(cmds.registeredCommands), len(allCommands()); got != want {
		t.Errorf("registered %d commands, table has %d", got, want)
	}
}

func TestHiddenCommandsStayCallable(t *testing.T) {
	// reset se ne nudi, ali mora da se otkuca.
	if _, ok := defaultCommands().registeredCommands["reset"]; !ok {
		t.Error("reset is hidden from the menu but also unreachable")
	}
}

func TestGuestIsOfferedOnlyWhatWorks(t *testing.T) {
	names := offeredNames(false)

	for _, want := range []string{"register", "login", "help"} {
		if !names[want] {
			t.Errorf("a logged-out user is not offered %q", want)
		}
	}
	for _, notWant := range []string{"tui", "browse", "addfeed", "bookmarks"} {
		if names[notWant] {
			t.Errorf("a logged-out user is offered %q, which requires login", notWant)
		}
	}
}

func TestLoggedInUserIsOfferedEverythingVisible(t *testing.T) {
	names := offeredNames(true)

	for _, e := range allCommands() {
		if e.hidden {
			continue
		}
		if !names[e.name] {
			t.Errorf("a logged-in user is not offered %q", e.name)
		}
	}
}

func TestHiddenCommandsAreNeverOffered(t *testing.T) {
	for _, loggedIn := range []bool{false, true} {
		if offeredNames(loggedIn)["reset"] {
			t.Errorf("reset is offered in the menu (loggedIn=%v)", loggedIn)
		}
	}
}

func TestReadingComesFirstForALoggedInUser(t *testing.T) {
	// Meni se otvara zbog citanja, pa tui ne sme da bude na dnu liste.
	items := offered(true)
	if len(items) == 0 {
		t.Fatal("nothing offered")
	}
	if items[0].Name != "tui" {
		t.Errorf("first offer is %q, want tui", items[0].Name)
	}
}

func TestGreeting(t *testing.T) {
	got := greeting(database.User{Name: "Tsunami"}, true)
	if !strings.Contains(got, "Tsunami") {
		t.Errorf("greeting = %q, want it to name the user", got)
	}

	got = greeting(database.User{}, false)
	if strings.Contains(got, "Hello") {
		t.Errorf("greeting = %q, want it not to greet a user who is not logged in", got)
	}
	for _, want := range []string{"register", "log in"} {
		if !strings.Contains(got, want) {
			t.Errorf("greeting = %q, want it to mention %q", got, want)
		}
	}
}

func TestPrintUsageListsEveryVisibleCommand(t *testing.T) {
	var sb strings.Builder
	printUsage(&sb)
	out := sb.String()

	if !strings.Contains(out, "usage: gator <command> [args...]") {
		t.Error("usage line is missing")
	}
	for _, e := range allCommands() {
		if e.hidden {
			continue
		}
		if !strings.Contains(out, e.name) {
			t.Errorf("usage output does not mention %q", e.name)
		}
		if !strings.Contains(out, e.summary) {
			t.Errorf("usage output does not describe %q", e.name)
		}
	}
}

func TestPrintUsageIsPlainText(t *testing.T) {
	// Ide i na stderr kad izlaz nije terminal, pa ne sme da nosi ANSI kodove.
	var sb strings.Builder
	printUsage(&sb)

	if strings.Contains(sb.String(), "\x1b[") {
		t.Error("usage output contains ANSI escape codes")
	}
}

func offeredNames(loggedIn bool) map[string]bool {
	names := make(map[string]bool)
	for _, it := range offered(loggedIn) {
		names[it.Name] = true
	}
	return names
}

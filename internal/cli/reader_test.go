package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/config"
	"github.com/MSamoilovic/gator-cli/internal/menu"
)

func TestTheReaderMenuLeavesOutWhatTheReaderAlreadyDoes(t *testing.T) {
	items := readerMenu(&state{Cfg: &config.Config{}})

	offered := map[string]bool{}
	for _, it := range items {
		offered[it.Name] = true
	}

	for _, name := range []string{"tui", "browse", "search", "bookmarks", "addfeed", "discover", "unfollow"} {
		if offered[name] {
			t.Errorf("the reader's menu offers %q, which would leave the reader to do what the "+
				"reader does with one key", name)
		}
	}
	for _, name := range []string{"agg", "stats", "export", "prune", "serve", "sync", "help"} {
		if !offered[name] && name != "sync" {
			t.Errorf("the reader's menu is missing %q, which needs the shell", name)
		}
	}
	if offered["sync"] {
		t.Error("sync is remoteOnly and this install is local, so the menu must not offer it")
	}
}

func TestEveryCommandIsEitherInTheReaderOrInItsMenu(t *testing.T) {
	inReader := 0
	for _, e := range allCommands() {
		if e.inReader {
			inReader++
		}
	}
	if want := 12; inReader != want {
		t.Errorf("%d commands are marked as covered by the reader, want %d — a new mark hides a "+
			"command from the menu, so it has to be deliberate", inReader, want)
	}

	items := readerMenu(&state{Cfg: &config.Config{}})
	if got, want := len(items), 19; got != want {
		t.Errorf("the reader's menu offers %d commands, want %d", got, want)
	}
}

func TestAPickedCommandComesBackAsTheNextCommandToRun(t *testing.T) {
	err := readerChoice(menu.Choice{Name: "agg", Args: []string{"15m"}}, true, nil)

	var next runNext
	if !errors.As(err, &next) {
		t.Fatalf("readerChoice returned %v, want the picked command to travel up as runNext", err)
	}
	if next.cmd.Name != "agg" || len(next.cmd.Args) != 1 || next.cmd.Args[0] != "15m" {
		t.Errorf("next = %+v, want agg 15m", next.cmd)
	}
}

func TestLeavingTheReaderWithoutPickingRunsNothing(t *testing.T) {
	if err := readerChoice(menu.Choice{}, false, nil); err != nil {
		t.Errorf("readerChoice = %v, want nil when the reader was simply quit", err)
	}

	boom := errors.New("the program crashed")
	if err := readerChoice(menu.Choice{Name: "agg"}, true, boom); !errors.Is(err, boom) {
		t.Errorf("readerChoice = %v, want the real error rather than a command to run", err)
	}
}

func TestTheCommandPickedInsideTheReaderRunsAfterItCloses(t *testing.T) {
	cmds := commands{registeredCommands: map[string]handlerFunc{}}

	var order []string
	cmds.register("tui", func(context.Context, *state, command) error {
		order = append(order, "tui")
		return runNext{cmd: command{Name: "agg", Args: []string{"15m"}}}
	})
	cmds.register("agg", func(_ context.Context, _ *state, cmd command) error {
		order = append(order, "agg "+cmd.Args[0])
		return nil
	})

	if err := chain(context.Background(), cmds, &state{}, command{Name: "tui"}); err != nil {
		t.Fatalf("chain: %v", err)
	}
	if len(order) != 2 || order[0] != "tui" || order[1] != "agg 15m" {
		t.Errorf("ran %v, want the reader first and then what it picked", order)
	}
}

func TestAFailureInThePickedCommandIsStillAFailure(t *testing.T) {
	cmds := commands{registeredCommands: map[string]handlerFunc{}}
	boom := errors.New("no such duration")

	cmds.register("tui", func(context.Context, *state, command) error {
		return runNext{cmd: command{Name: "agg"}}
	})
	cmds.register("agg", func(context.Context, *state, command) error { return boom })

	err := chain(context.Background(), cmds, &state{}, command{Name: "tui"})
	if !errors.Is(err, boom) {
		t.Errorf("chain = %v, want the picked command's error", err)
	}
}

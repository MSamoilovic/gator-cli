package tui

import (
	"context"
	"fmt"
	"os"

	"github.com/MSamoilovic/gator-cli/internal/menu"
	"github.com/MSamoilovic/gator-cli/internal/store"

	tea "github.com/charmbracelet/bubbletea"
)

func Run(ctx context.Context, st store.Store, user store.User, server string, cmds []menu.Item) (menu.Choice, bool, error) {
	m := newModel(ctx, st, user, loadState(), server)
	m.commands = cmds
	return run(m)
}

func RunCatalog(ctx context.Context, st store.Store, user store.User, server string, cmds []menu.Item) (menu.Choice, bool, error) {
	m := newModel(ctx, st, user, loadState(), server)
	m.commands = cmds
	m.openOnLoad = true
	return run(m)
}

func run(m model) (menu.Choice, bool, error) {
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return menu.Choice{}, false, err
	}

	last, ok := final.(model)
	if !ok {
		return menu.Choice{}, false, nil
	}
	if err := last.snapshot().save(); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not save TUI state:", err)
	}
	return last.choice, last.chose, nil
}

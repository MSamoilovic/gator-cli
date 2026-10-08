package tui

import (
	"context"
	"fmt"
	"os"

	"github.com/MSamoilovic/gator-cli/internal/store"

	tea "github.com/charmbracelet/bubbletea"
)

func Run(ctx context.Context, st store.Store, user store.User) error {
	return run(newModel(ctx, st, user, loadState()))
}

func RunCatalog(ctx context.Context, st store.Store, user store.User) error {
	m := newModel(ctx, st, user, loadState())
	m.openOnLoad = true
	return run(m)
}

func run(m model) error {
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}

	if m, ok := final.(model); ok {
		if err := m.snapshot().save(); err != nil {
			fmt.Fprintln(os.Stderr, "warning: could not save TUI state:", err)
		}
	}
	return nil
}

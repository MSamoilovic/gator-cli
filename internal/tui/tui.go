package tui

import (
	"context"
	"fmt"
	"os"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
)

func Run(ctx context.Context, q *database.Queries, user database.User) error {
	return run(newModel(ctx, q, user, loadState()))
}

// RunCatalog otvara TUI odmah na biracu kategorija, za `gator discover` na
// interaktivnom terminalu. Sve ostalo je isto, ukljucujuci pamcenje stanja.
func RunCatalog(ctx context.Context, q *database.Queries, user database.User) error {
	m := newModel(ctx, q, user, loadState())
	m.openOnLoad = true
	return run(m)
}

func run(m model) error {
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}

	// Neuspelo pamcenje stanja ne sme da obori komandu koja je inace prosla.
	if m, ok := final.(model); ok {
		if err := m.snapshot().save(); err != nil {
			fmt.Fprintln(os.Stderr, "warning: could not save TUI state:", err)
		}
	}
	return nil
}

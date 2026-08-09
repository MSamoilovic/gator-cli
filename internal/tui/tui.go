package tui

import (
	"context"
	"fmt"
	"os"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
)

func Run(ctx context.Context, q *database.Queries, user database.User) error {
	final, err := tea.NewProgram(newModel(ctx, q, user, loadState()), tea.WithAltScreen()).Run()
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

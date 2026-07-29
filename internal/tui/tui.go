package tui

import (
	"context"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func Run(ctx context.Context, q *database.Queries, userID uuid.UUID) error {
	_, err := tea.NewProgram(newModel(ctx, q, userID), tea.WithAltScreen()).Run()
	return err
}

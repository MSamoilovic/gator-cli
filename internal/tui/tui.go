package tui

import (
	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)


type model struct {
	queries *database.Queries
	userID uuid.UUID
}

func (m model) Init() tea.Cmd {return nil}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}


func (m model) View() string {
	return "Gator TUI -  press q to quit\n"
}

func Run(q *database.Queries, userID uuid.UUID) error {
    m := model{queries: q, userID: userID}
    _, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
    return err
}
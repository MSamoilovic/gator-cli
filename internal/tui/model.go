package tui

import (
	"context"

	"gator-cli/internal/database"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

type model struct {
	ctx     context.Context
	queries *database.Queries
	userID  uuid.UUID

	list    list.Model
	loading bool
	err     error
}

func newModel(ctx context.Context, q *database.Queries, userID uuid.UUID) model {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Posts"
	l.SetStatusBarItemName("post", "posts")

	return model{
		ctx:     ctx,
		queries: q,
		userID:  userID,
		list:    l,
		loading: true,
	}
}

func (m model) Init() tea.Cmd {
	return loadPosts(m.ctx, m.queries, m.userID)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil

	case postsLoadedMsg:
		m.loading = false
		items := make([]list.Item, len(msg.posts))
		for i, p := range msg.posts {
			items[i] = postItem{post: p}
		}
		return m, m.list.SetItems(items)

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	switch {
	case m.err != nil:
		return "Error: " + m.err.Error() + "\n\nPress q to quit.\n"
	case m.loading:
		return "\n  Loading posts...\n"
	default:
		return m.list.View()
	}
}

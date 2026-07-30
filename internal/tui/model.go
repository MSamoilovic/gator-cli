package tui

import (
	"context"

	"gator-cli/internal/database"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

type model struct {
	ctx     context.Context
	queries *database.Queries
	userID  uuid.UUID

	list     list.Model
	viewport viewport.Model
	selected database.Post

	showDetail bool
	loading    bool
	err        error
}

func newModel(ctx context.Context, q *database.Queries, userID uuid.UUID) model {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Posts"
	l.SetStatusBarItemName("post", "posts")

	return model{
		ctx:      ctx,
		queries:  q,
		userID:   userID,
		list:     l,
		viewport: viewport.New(0, 0),
		loading:  true,
	}
}

func (m model) Init() tea.Cmd {
	return loadPosts(m.ctx, m.queries, m.userID)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		m.viewport.Width = msg.Width
		m.viewport.Height = max(msg.Height-detailChromeHeight, 1)
		if m.showDetail {
			m.viewport.SetContent(renderDetailBody(m.selected, m.viewport.Width))
		}
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
		if m.showDetail {
			return m.updateDetail(msg)
		}
		return m.updateList(msg)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.showDetail = false
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.list.FilterState() != list.Filtering {
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			item, ok := m.list.SelectedItem().(postItem)
			if !ok {
				return m, nil
			}
			m.selected = item.post
			m.showDetail = true
			m.viewport.SetContent(renderDetailBody(m.selected, m.viewport.Width))
			m.viewport.GotoTop()
			return m, nil
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
	case m.showDetail:
		return m.detailView()
	default:
		return m.list.View()
	}
}

func (m model) detailView() string {
	return renderDetailHeader(m.selected, m.viewport.Width) +
		m.viewport.View() +
		"\n\n↑/↓ scroll · esc back · q quit"
}

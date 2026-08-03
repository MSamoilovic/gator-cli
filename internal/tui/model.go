package tui

import (
	"context"
	"strings"

	"gator-cli/internal/database"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

const (
	feedsKeysHint  = "↑/↓ move · ⏎ select feed · tab posts · q quit"
	listKeysHint   = "↑/↓ move · ⏎ read · o browser · b bookmark · tab feeds · / filter · q quit"
	detailKeysHint = "↑/↓ scroll · o browser · b bookmark · esc back · q quit"

	feedPanelWidth   = 26
	minWidthForFeeds = 60
	allFeedsLabel    = "All feeds"
)

type focusArea int

const (
	focusPosts focusArea = iota
	focusFeeds
)

type model struct {
	ctx     context.Context
	queries *database.Queries
	userID  uuid.UUID

	list     list.Model
	feedList list.Model
	viewport viewport.Model
	selected database.Post

	bookmarks map[uuid.UUID]bool
	feedID    uuid.UUID

	focus       focusArea
	width       int
	height      int
	feedWidth   int
	status      string
	statusToken int

	showDetail bool
	loading    bool
	err        error
}

func newModel(ctx context.Context, q *database.Queries, userID uuid.UUID) model {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Posts"
	l.SetStatusBarItemName("post", "posts")
	l.SetShowHelp(false)

	feedDelegate := list.NewDefaultDelegate()
	feedDelegate.ShowDescription = false
	feedDelegate.SetSpacing(0)

	fl := list.New([]list.Item{feedItem{id: uuid.Nil, name: allFeedsLabel}}, feedDelegate, 0, 0)
	fl.Title = "Feeds"
	fl.SetStatusBarItemName("feed", "feeds")
	fl.SetShowStatusBar(false)
	fl.SetFilteringEnabled(false)
	fl.SetShowHelp(false)

	m := model{
		ctx:       ctx,
		queries:   q,
		userID:    userID,
		list:      l,
		feedList:  fl,
		viewport:  viewport.New(0, 0),
		bookmarks: make(map[uuid.UUID]bool),
		loading:   true,
	}
	m.applyFocus()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		loadPosts(m.ctx, m.queries, m.userID, m.feedID),
		loadBookmarks(m.ctx, m.queries, m.userID),
		loadFeeds(m.ctx, m.queries, m.userID),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil

	case postsLoadedMsg:
		m.loading = false
		items := make([]list.Item, len(msg.posts))
		for i, p := range msg.posts {
			items[i] = postItem{post: p, bookmarks: m.bookmarks}
		}
		return m, m.list.SetItems(items)

	case feedsLoadedMsg:
		items := make([]list.Item, 0, len(msg.feeds)+1)
		items = append(items, feedItem{id: uuid.Nil, name: allFeedsLabel})
		for _, f := range msg.feeds {
			items = append(items, feedItem{id: f.FeedID, name: f.FeedName})
		}
		return m, m.feedList.SetItems(items)

	case bookmarksLoadedMsg:
		for _, id := range msg.postIDs {
			m.bookmarks[id] = true
		}
		return m, nil

	case bookmarkToggledMsg:
		if msg.bookmarked {
			m.bookmarks[msg.postID] = true
			return m.withStatus("Bookmarked ★")
		}
		delete(m.bookmarks, msg.postID)
		return m.withStatus("Bookmark removed")

	case openedMsg:
		return m.withStatus("Opened " + msg.url)

	case statusExpiredMsg:
		if msg.token == m.statusToken {
			m.status = ""
		}
		return m, nil

	case errMsg:
		if m.loading {
			m.loading = false
			m.err = msg.err
			return m, nil
		}
		return m.withStatus("Error: " + msg.err.Error())

	case tea.KeyMsg:
		switch {
		case m.showDetail:
			return m.updateDetail(msg)
		case m.focus == focusFeeds:
			return m.updateFeeds(msg)
		default:
			return m.updateList(msg)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *model) resize(w, h int) {
	m.width, m.height = w, h

	m.feedWidth = feedPanelWidth
	if w < minWidthForFeeds {
		m.feedWidth = 0
		m.focus = focusPosts
	}

	postsWidth := w - m.feedWidth
	if m.feedWidth > 0 {
		postsWidth--
	}

	panelHeight := max(h-1, 1)
	m.feedList.SetSize(m.feedWidth, panelHeight)
	m.list.SetSize(max(postsWidth, 1), panelHeight)

	m.viewport.Width = w
	m.viewport.Height = max(h-detailChromeHeight, 1)
	if m.showDetail {
		m.viewport.SetContent(renderDetailBody(m.selected, m.viewport.Width))
	}
	m.applyFocus()
}

func (m *model) applyFocus() {
	m.list.Styles.Title = panelTitleStyle(m.focus == focusPosts)
	m.feedList.Styles.Title = panelTitleStyle(m.focus == focusFeeds)
}

func (m model) withStatus(text string) (model, tea.Cmd) {
	m.statusToken++
	m.status = text
	return m, expireStatus(m.statusToken)
}

func (m model) currentPost() (database.Post, bool) {
	if m.showDetail {
		return m.selected, true
	}
	item, ok := m.list.SelectedItem().(postItem)
	if !ok {
		return database.Post{}, false
	}
	return item.post, true
}

func (m model) postAction(key string) (tea.Model, tea.Cmd, bool) {
	post, ok := m.currentPost()
	if !ok {
		return m, nil, key == "o" || key == "b"
	}

	switch key {
	case "o":
		return m, openInBrowser(post.Url), true
	case "b":
		if m.bookmarks[post.ID] {
			return m, removeBookmark(m.ctx, m.queries, m.userID, post.ID), true
		}
		return m, addBookmark(m.ctx, m.queries, m.userID, post.ID), true
	}
	return m, nil, false
}

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.showDetail = false
		return m, nil
	}

	if next, cmd, handled := m.postAction(msg.String()); handled {
		return next, cmd
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m model) updateFeeds(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab", "shift+tab", "esc":
		m.focus = focusPosts
		m.applyFocus()
		return m, nil
	case "enter":
		item, ok := m.feedList.SelectedItem().(feedItem)
		if !ok {
			return m, nil
		}
		m.feedID = item.id
		m.focus = focusPosts
		m.applyFocus()
		m.list.Title = item.name
		if item.id == uuid.Nil {
			m.list.Title = "Posts"
		}
		m.list.ResetSelected()
		return m, loadPosts(m.ctx, m.queries, m.userID, m.feedID)
	}

	var cmd tea.Cmd
	m.feedList, cmd = m.feedList.Update(msg)
	return m, cmd
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.list.FilterState() != list.Filtering {
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "shift+tab":
			if m.feedWidth > 0 {
				m.focus = focusFeeds
				m.applyFocus()
			}
			return m, nil
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

		if next, cmd, handled := m.postAction(msg.String()); handled {
			return next, cmd
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
		return m.panelsView()
	}
}

func (m model) panelsView() string {
	hint := listKeysHint
	if m.focus == focusFeeds {
		hint = feedsKeysHint
	}

	if m.feedWidth == 0 {
		return m.list.View() + "\n" + m.statusLine(listKeysHint)
	}

	panels := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.feedList.View(),
		verticalRule(max(m.height-1, 1)),
		m.list.View(),
	)
	return panels + "\n" + m.statusLine(hint)
}

func verticalRule(height int) string {
	return strings.TrimSuffix(strings.Repeat("│\n", height), "\n")
}

func (m model) detailView() string {
	return renderDetailHeader(m.selected, m.viewport.Width) +
		m.viewport.View() +
		"\n\n" + m.statusLine(detailKeysHint)
}

func (m model) statusLine(hint string) string {
	text := hint
	if m.status != "" {
		text = m.status
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(text)
}

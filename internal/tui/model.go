package tui

import (
	"context"
	"strconv"
	"strings"

	"gator-cli/internal/database"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

const (
	feedPanelWidth   = 26
	minWidthForFeeds = 60
	allFeedsLabel    = "All feeds"
	postsTitle       = "Posts"

	// Koliko stavki pre dna liste okida ucitavanje sledece strane.
	loadMoreThreshold = 5
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

	keys     keyMap
	help     help.Model
	list     list.Model
	feedList list.Model
	viewport viewport.Model
	spinner  spinner.Model
	search   textinput.Model
	selected database.Post

	bookmarks map[uuid.UUID]bool
	feedID    uuid.UUID
	feedName  string
	query     string

	offset      int32
	hasMore     bool
	loadingMore bool

	feedCount   int
	feedsLoaded bool

	focus       focusArea
	width       int
	height      int
	feedWidth   int
	status      string
	statusToken int

	searching  bool
	showDetail bool
	loading    bool
	err        error
}

func newModel(ctx context.Context, q *database.Queries, userID uuid.UUID) model {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = postsTitle
	l.SetStatusBarItemName("post", "posts")
	l.SetShowHelp(false)

	feedDelegate := list.NewDefaultDelegate()
	feedDelegate.ShowDescription = false
	feedDelegate.SetSpacing(0)

	fl := list.New([]list.Item{feedItem{id: uuid.Nil, name: allFeedsLabel}}, feedDelegate, 0, 0)
	fl.Title = "Feeds"
	fl.SetShowStatusBar(false)
	fl.SetFilteringEnabled(false)
	fl.SetShowHelp(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	ti := textinput.New()
	ti.Prompt = "search: "
	ti.CharLimit = 200

	m := model{
		ctx:       ctx,
		queries:   q,
		userID:    userID,
		keys:      defaultKeyMap(),
		help:      help.New(),
		list:      l,
		feedList:  fl,
		viewport:  viewport.New(0, 0),
		spinner:   sp,
		search:    ti,
		bookmarks: make(map[uuid.UUID]bool),
		loading:   true,
		hasMore:   true,
	}
	m.applyFocus()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		loadPosts(m.ctx, m.queries, m.userID, m.feedID, 0),
		loadBookmarks(m.ctx, m.queries, m.userID),
		loadFeeds(m.ctx, m.queries, m.userID),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case postsLoadedMsg:
		return m.postsLoaded(msg)

	case feedsLoadedMsg:
		m.feedsLoaded = true
		m.feedCount = len(msg.feeds)
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
		m.loadingMore = false
		if m.loading {
			m.loading = false
			m.err = msg.err
			return m, nil
		}
		return m.withStatus("Error: " + msg.err.Error())

	case tea.KeyMsg:
		switch {
		case m.searching:
			return m.updateSearch(msg)
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

func (m model) postsLoaded(msg postsLoadedMsg) (tea.Model, tea.Cmd) {
	// Zastarela strana: stigla je posle promene feeda ili pretrage.
	if msg.paged && msg.offset != m.offset {
		return m, nil
	}

	m.loading = false
	m.loadingMore = false
	m.hasMore = msg.paged && len(msg.posts) == pageSize

	items := make([]list.Item, len(msg.posts))
	for i, p := range msg.posts {
		items[i] = postItem{post: p, bookmarks: m.bookmarks}
	}

	if msg.paged && msg.offset > 0 {
		return m, m.list.SetItems(append(m.list.Items(), items...))
	}

	m.offset = 0
	m.list.ResetSelected()
	return m, m.list.SetItems(items)
}

// startLoad pokrece cistu (prvu) stranu za trenutni feed i resetuje paginaciju.
func (m *model) startLoad() tea.Cmd {
	m.offset = 0
	m.hasMore = true
	m.loadingMore = false
	return loadPosts(m.ctx, m.queries, m.userID, m.feedID, 0)
}

func (m *model) maybeLoadMore() tea.Cmd {
	switch {
	case !m.hasMore, m.loadingMore, m.query != "":
		return nil
	case m.list.FilterState() == list.Filtering:
		return nil
	case m.list.Index() < len(m.list.Items())-loadMoreThreshold:
		return nil
	}

	m.loadingMore = true
	m.offset += pageSize
	return loadPosts(m.ctx, m.queries, m.userID, m.feedID, m.offset)
}

func (m *model) resize(w, h int) {
	m.width, m.height = w, h
	m.help.Width = w

	m.feedWidth = feedPanelWidth
	if w < minWidthForFeeds {
		m.feedWidth = 0
		m.focus = focusPosts
	}

	postsWidth := w - m.feedWidth
	if m.feedWidth > 0 {
		postsWidth--
	}

	panelHeight := max(h-m.footerHeight(), 1)
	m.feedList.SetSize(m.feedWidth, panelHeight)
	m.list.SetSize(max(postsWidth, 1), panelHeight)
	m.search.Width = max(w-len(m.search.Prompt)-1, 1)

	m.viewport.Width = w
	m.viewport.Height = max(h-detailChromeHeight-m.footerHeight()+1, 1)
	if m.showDetail {
		m.viewport.SetContent(renderDetailBody(m.selected, m.viewport.Width))
	}
	m.applyFocus()
}

func (m *model) applyFocus() {
	m.list.Styles.Title = panelTitleStyle(m.focus == focusPosts)
	m.feedList.Styles.Title = panelTitleStyle(m.focus == focusFeeds)
}

func (m *model) setPostsTitle() {
	switch {
	case m.query != "":
		m.list.Title = "Search: " + m.query
	case m.feedName != "":
		m.list.Title = m.feedName
	default:
		m.list.Title = postsTitle
	}
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

func (m model) postAction(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.Open):
		post, ok := m.currentPost()
		if !ok {
			return m, nil, true
		}
		return m, openInBrowser(post.Url), true

	case key.Matches(msg, m.keys.Bookmark):
		post, ok := m.currentPost()
		if !ok {
			return m, nil, true
		}
		if m.bookmarks[post.ID] {
			return m, removeBookmark(m.ctx, m.queries, m.userID, post.ID), true
		}
		return m, addBookmark(m.ctx, m.queries, m.userID, post.ID), true
	}
	return m, nil, false
}

func (m model) toggleHelp() (tea.Model, tea.Cmd) {
	m.help.ShowAll = !m.help.ShowAll
	m.resize(m.width, m.height)
	return m, nil
}

func (m model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.searching = false
		m.search.Blur()
		return m, nil

	case tea.KeyEnter:
		query := strings.TrimSpace(m.search.Value())
		m.searching = false
		m.search.Blur()
		if query == "" {
			return m, nil
		}
		m.query = query
		m.setPostsTitle()
		return m, searchPosts(m.ctx, m.queries, m.userID, query)

	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	return m, cmd
}

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyCtrlC:
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		return m.toggleHelp()
	case key.Matches(msg, m.keys.Back), msg.String() == "q":
		m.showDetail = false
		return m, nil
	}

	if next, cmd, handled := m.postAction(msg); handled {
		return next, cmd
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m model) updateFeeds(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		return m.toggleHelp()

	case key.Matches(msg, m.keys.Tab), key.Matches(msg, m.keys.Back):
		m.focus = focusPosts
		m.applyFocus()
		return m, nil

	case key.Matches(msg, m.keys.Select):
		item, ok := m.feedList.SelectedItem().(feedItem)
		if !ok {
			return m, nil
		}
		m.feedID = item.id
		m.feedName = ""
		if item.id != uuid.Nil {
			m.feedName = item.name
		}
		m.query = ""
		m.setPostsTitle()
		m.focus = focusPosts
		m.applyFocus()
		return m, m.startLoad()
	}

	var cmd tea.Cmd
	m.feedList, cmd = m.feedList.Update(msg)
	return m, cmd
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.list.FilterState() != list.Filtering {
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Help):
			return m.toggleHelp()

		case key.Matches(msg, m.keys.Search):
			m.searching = true
			m.search.SetValue("")
			return m, m.search.Focus()

		case key.Matches(msg, m.keys.Back):
			if m.query == "" {
				return m, nil
			}
			m.query = ""
			m.setPostsTitle()
			return m, m.startLoad()

		case key.Matches(msg, m.keys.Tab):
			if m.feedWidth > 0 {
				m.focus = focusFeeds
				m.applyFocus()
			}
			return m, nil

		case key.Matches(msg, m.keys.Read):
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

		if next, cmd, handled := m.postAction(msg); handled {
			return next, cmd
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, tea.Batch(cmd, m.maybeLoadMore())
}

func (m model) View() string {
	switch {
	case m.err != nil:
		return "Error: " + m.err.Error() + "\n\nPress q to quit.\n"
	case m.loading:
		return "\n  " + m.spinner.View() + " Loading posts...\n"
	case m.showDetail:
		return m.detailView()
	default:
		return m.panelsView()
	}
}

func (m model) panelsView() string {
	if m.feedWidth == 0 {
		return m.postsPanel() + "\n" + m.footer()
	}

	panels := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.feedList.View(),
		verticalRule(max(m.height-m.footerHeight(), 1)),
		m.postsPanel(),
	)
	return panels + "\n" + m.footer()
}

// postsPanel je lista, ili poruka o praznom stanju koja objasnjava zasto je prazna.
func (m model) postsPanel() string {
	if len(m.list.Items()) > 0 {
		return m.list.View()
	}

	body := emptyStateStyle.Render(m.emptyStateText())
	panel := m.list.Styles.Title.Render(m.list.Title) + "\n\n" + body

	return lipgloss.NewStyle().
		Width(m.list.Width()).
		Height(max(m.height-m.footerHeight(), 1)).
		MaxWidth(m.list.Width()).
		Render(panel)
}

func (m model) emptyStateText() string {
	switch {
	case m.query != "":
		return "No posts match " + strconv.Quote(m.query) + ".\nPress esc to go back."
	case m.feedsLoaded && m.feedCount == 0:
		return "You are not following any feeds.\nAdd one with: gator addfeed <name> <url>"
	case m.feedName != "":
		return "No posts stored for " + m.feedName + " yet.\nFetch some with: gator agg 15m"
	default:
		return "No posts yet.\nFetch some with: gator agg 15m"
	}
}

func verticalRule(height int) string {
	return strings.TrimSuffix(strings.Repeat("│\n", height), "\n")
}

func (m model) detailView() string {
	return renderDetailHeader(m.selected, m.viewport.Width) +
		m.viewport.View() +
		"\n\n" + m.footer()
}

func (m model) currentBindings() []key.Binding {
	switch {
	case m.showDetail:
		return m.keys.detailHelp()
	case m.focus == focusFeeds:
		return m.keys.feedsHelp()
	default:
		return m.keys.listHelp(m.feedWidth > 0, m.query != "")
	}
}

// footerHeight zavisi samo od m.help.ShowAll, pa je racunica layouta stabilna
// bez obzira na to da li se trenutno prikazuje status ili search input.
func (m model) footerHeight() int {
	if !m.help.ShowAll {
		return 1
	}
	return lipgloss.Height(m.help.FullHelpView(m.keys.fullHelp()))
}

// footer je uvek tacno footerHeight redova: search input dok se kuca, pa
// status poruka, inace help. MaxWidth je obavezan i za help — bubbles
// ShortHelpView preseca red samo ako i tri tacke staju u sirinu.
func (m model) footer() string {
	var line string
	switch {
	case m.searching:
		line = m.search.View()
	case m.status != "":
		line = m.status
	case m.help.ShowAll:
		line = m.help.FullHelpView(m.keys.fullHelp())
	default:
		line = m.help.ShortHelpView(m.currentBindings())
	}

	return lipgloss.NewStyle().
		MaxWidth(m.width).
		Height(m.footerHeight()).
		MaxHeight(m.footerHeight()).
		Render(line)
}

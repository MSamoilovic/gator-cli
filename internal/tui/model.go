package tui

import (
	"context"
	"fmt"
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
	bookmarksTitle   = "Bookmarks"

	sortDesc = "desc"
	sortAsc  = "asc"

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

	sortDir     string
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

	searching     bool
	fetching      bool
	showBookmarks bool
	showDetail    bool
	loading       bool
	err           error
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
		sortDir:   sortDesc,
		loading:   true,
		hasMore:   true,
	}
	m.applyFocus()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		loadPosts(m.ctx, m.queries, m.userID, m.feedID, 0, m.sortDir),
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
		next, cmd := m.withStatus("Bookmark removed")
		// U pogledu bookmark-a odjavljeni post vise ne pripada listi.
		if next.showBookmarks {
			return next, tea.Batch(cmd, loadBookmarkedPosts(next.ctx, next.queries, next.userID))
		}
		return next, cmd

	case scrapedMsg:
		m.fetching = false
		next, cmd := m.withStatus(scrapeSummary(msg))
		// Nove postove tek treba ucitati iz baze.
		return next, tea.Batch(cmd, next.startLoad())

	case openedMsg:
		return m.withStatus("Opened " + msg.url)

	case statusExpiredMsg:
		if msg.token == m.statusToken {
			m.status = ""
		}
		return m, nil

	case errMsg:
		m.loadingMore = false
		m.fetching = false
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

// startLoad pokrece cistu (prvu) stranu i resetuje paginaciju.
func (m *model) startLoad() tea.Cmd {
	m.offset = 0
	m.hasMore = true
	m.loadingMore = false
	if m.showBookmarks {
		return loadBookmarkedPosts(m.ctx, m.queries, m.userID)
	}
	return loadPosts(m.ctx, m.queries, m.userID, m.feedID, 0, m.sortDir)
}

func (m *model) maybeLoadMore() tea.Cmd {
	switch {
	case !m.hasMore, m.loadingMore, m.query != "", m.showBookmarks:
		return nil
	case m.list.FilterState() == list.Filtering:
		return nil
	case m.list.Index() < len(m.list.Items())-loadMoreThreshold:
		return nil
	}

	m.loadingMore = true
	m.offset += pageSize
	return loadPosts(m.ctx, m.queries, m.userID, m.feedID, m.offset, m.sortDir)
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
	case m.showBookmarks:
		m.list.Title = bookmarksTitle
	case m.query != "":
		m.list.Title = "Search: " + m.query
	case m.feedName != "":
		m.list.Title = m.feedName
	default:
		m.list.Title = postsTitle
	}
}

func scrapeSummary(msg scrapedMsg) string {
	switch {
	case msg.feeds == 0:
		return "No feeds were due for a fetch"
	case msg.failed > 0:
		return fmt.Sprintf("Fetched %d new posts · %d feed(s) failed", msg.saved, msg.failed)
	default:
		return fmt.Sprintf("Fetched %d new posts from %d feeds", msg.saved, msg.feeds)
	}
}

// canGoBack je tacno kad je lista u nekom izvedenom pogledu koji esc napusta.
func (m model) canGoBack() bool {
	return m.query != "" || m.showBookmarks
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
		m.showBookmarks = false
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
			if !m.canGoBack() {
				return m, nil
			}
			m.query = ""
			m.showBookmarks = false
			m.setPostsTitle()
			return m, m.startLoad()

		case key.Matches(msg, m.keys.Reload):
			next, cmd := m.withStatus("Reloading…")
			return next, tea.Batch(cmd, next.startLoad())

		case key.Matches(msg, m.keys.Fetch):
			if m.fetching {
				return m, nil
			}
			m.fetching = true
			next, cmd := m.withStatus("Fetching feeds…")
			return next, tea.Batch(cmd, scrapeFeeds(next.ctx, next.queries))

		case key.Matches(msg, m.keys.Saved):
			m.showBookmarks = !m.showBookmarks
			m.query = ""
			m.setPostsTitle()
			return m, m.startLoad()

		case key.Matches(msg, m.keys.Sort):
			if m.showBookmarks || m.query != "" {
				return m.withStatus("Sorting applies to feed posts only")
			}
			label := "oldest first"
			if m.sortDir == sortAsc {
				m.sortDir, label = sortDesc, "newest first"
			} else {
				m.sortDir = sortAsc
			}
			next, cmd := m.withStatus("Sorted " + label)
			return next, tea.Batch(cmd, next.startLoad())

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
	case m.showBookmarks:
		return "No bookmarks yet.\nPress b on a post to save it."
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
		return m.keys.listHelp(m.feedWidth > 0, m.canGoBack())
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

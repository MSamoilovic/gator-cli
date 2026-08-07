package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

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

// inputMode odredjuje sta se kuca u prompt polju; jedno polje sluzi i pretrazi
// i dodavanju feeda, pa dva stanja ne mogu istovremeno biti aktivna.
type inputMode int

const (
	inputNone inputMode = iota
	inputSearch
	inputAddFeed
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
	prompt   textinput.Model
	selected database.Post

	bookmarks map[uuid.UUID]bool
	reads     map[uuid.UUID]bool
	unread    map[uuid.UUID]int
	feedID    uuid.UUID
	feedName  string
	query     string

	sortDir     string
	since       time.Duration
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

	input         inputMode
	confirming    bool
	fetching      bool
	unreadOnly    bool
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

	fl := list.New(nil, feedDelegate, 0, 0)
	fl.Title = "Feeds"
	fl.SetShowStatusBar(false)
	fl.SetFilteringEnabled(false)
	fl.SetShowHelp(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	ti := textinput.New()
	ti.CharLimit = 500

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
		prompt:    ti,
		bookmarks: make(map[uuid.UUID]bool),
		reads:     make(map[uuid.UUID]bool),
		unread:    make(map[uuid.UUID]int),
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
		loadPosts(m.ctx, m.queries, m.userID, m.filter(), 0),
		loadBookmarks(m.ctx, m.queries, m.userID),
		loadReads(m.ctx, m.queries, m.userID),
		loadFeeds(m.ctx, m.queries, m.userID),
		loadUnreadCounts(m.ctx, m.queries, m.userID),
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
		return m, m.feedList.SetItems(m.feedItems(msg.feeds))

	case readsLoadedMsg:
		for _, id := range msg.postIDs {
			m.reads[id] = true
		}
		return m, nil

	case unreadCountsMsg:
		clear(m.unread)
		for _, c := range msg.counts {
			m.unread[c.FeedID] = int(c.Unread)
		}
		return m, nil

	case readToggledMsg:
		m.applyRead(msg.postID, msg.feedID, msg.read)
		return m, nil

	case allReadMsg:
		if msg.count == 0 {
			return m.withStatus("Nothing to mark")
		}
		next, cmd := m.withStatus(fmt.Sprintf("Marked %d posts read", msg.count))
		if next.unreadOnly {
			return next, tea.Batch(cmd, next.startLoad())
		}
		return next, cmd

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

	case feedAddedMsg:
		label := "Now following " + msg.name
		if msg.created {
			label = "Added " + msg.name
		}
		next, cmd := m.withStatus(label)
		return next, tea.Batch(cmd, next.reloadFeeds(), next.startLoad())

	case feedUnfollowMsg:
		// Ako je odjavljen bas filtrirani feed, lista se vraca na sve.
		if m.feedName == msg.name {
			m.feedID = uuid.Nil
			m.feedName = ""
			m.setPostsTitle()
			m.feedList.ResetSelected()
		}
		next, cmd := m.withStatus("Unfollowed " + msg.name)
		return next, tea.Batch(cmd, next.reloadFeeds(), next.startLoad())

	case scrapedMsg:
		m.fetching = false
		next, cmd := m.withStatus(scrapeSummary(msg))
		// Nove postove tek treba ucitati iz baze.
		return next, tea.Batch(cmd, next.startLoad())

	case openedMsg:
		return m.withStatus("Opened " + msg.url)

	case copiedMsg:
		return m.withStatus("Copied " + msg.url)

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
		case m.confirming:
			return m.updateConfirm(msg)
		case m.input != inputNone:
			return m.updateInput(msg)
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
		items[i] = postItem{post: p, bookmarks: m.bookmarks, reads: m.reads}
	}

	if msg.paged && msg.offset > 0 {
		return m, m.list.SetItems(append(m.list.Items(), items...))
	}

	m.offset = 0
	m.list.ResetSelected()
	return m, m.list.SetItems(items)
}

func (m model) feedItems(feeds []database.GetFeedFollowsForUserRow) []list.Item {
	items := make([]list.Item, 0, len(feeds)+1)
	items = append(items, feedItem{id: uuid.Nil, name: allFeedsLabel, unread: m.unread})
	for _, f := range feeds {
		items = append(items, feedItem{id: f.FeedID, name: f.FeedName, unread: m.unread})
	}
	return items
}

// applyRead odrzava brojac nepročitanih lokalno umesto novog upita po tasteru.
func (m *model) applyRead(postID, feedID uuid.UUID, read bool) {
	if m.reads[postID] == read {
		return
	}
	if read {
		m.reads[postID] = true
		if m.unread[feedID] > 0 {
			m.unread[feedID]--
		}
		return
	}
	delete(m.reads, postID)
	m.unread[feedID]++
}

// filter skuplja trenutna ogranicenja liste u jedan objekat.
func (m model) filter() postFilter {
	f := postFilter{
		feedID:     m.feedID,
		sortDir:    m.sortDir,
		unreadOnly: m.unreadOnly,
	}
	if m.since > 0 {
		f.since = time.Now().Add(-m.since)
	}
	return f
}

// reloadFeeds osvezava levi panel i brojace nepročitanih posle promene pretplata.
func (m model) reloadFeeds() tea.Cmd {
	return tea.Batch(
		loadFeeds(m.ctx, m.queries, m.userID),
		loadUnreadCounts(m.ctx, m.queries, m.userID),
	)
}

// startLoad pokrece cistu (prvu) stranu i resetuje paginaciju.
func (m *model) startLoad() tea.Cmd {
	m.offset = 0
	m.hasMore = true
	m.loadingMore = false
	if m.showBookmarks {
		return loadBookmarkedPosts(m.ctx, m.queries, m.userID)
	}
	return loadPosts(m.ctx, m.queries, m.userID, m.filter(), 0)
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
	return loadPosts(m.ctx, m.queries, m.userID, m.filter(), m.offset)
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
	m.prompt.Width = max(w-len(m.prompt.Prompt)-1, 1)

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
		return
	case m.query != "":
		m.list.Title = "Search: " + m.query
		return
	case m.feedName != "":
		m.list.Title = m.feedName
	default:
		m.list.Title = postsTitle
	}
	if label := sinceLabel(m.since); label != "" {
		m.list.Title += " · " + label
	}
}

// sinceRanges je ciklus kroz koji prolazi taster "t"; 0 znaci bez ogranicenja.
var sinceRanges = []time.Duration{0, 24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour}

func sinceLabel(d time.Duration) string {
	switch d {
	case 24 * time.Hour:
		return "24h"
	case 7 * 24 * time.Hour:
		return "7d"
	case 30 * 24 * time.Hour:
		return "30d"
	default:
		return ""
	}
}

func nextSince(d time.Duration) time.Duration {
	for i, r := range sinceRanges {
		if r == d {
			return sinceRanges[(i+1)%len(sinceRanges)]
		}
	}
	return 0
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

// openPost puni detalj i oznacava post procitanim; vec procitan post ne pravi
// novi upis u bazu.
func (m *model) openPost(post database.Post) tea.Cmd {
	m.selected = post
	m.viewport.SetContent(renderDetailBody(post, m.viewport.Width))
	m.viewport.GotoTop()

	if m.reads[post.ID] {
		return nil
	}
	m.applyRead(post.ID, post.FeedID, true)
	return setPostRead(m.ctx, m.queries, m.userID, post, true)
}

// stepPost pomera selekciju u listi za delta i otvara taj post u detalju.
func (m *model) stepPost(delta int) tea.Cmd {
	next := m.list.Index() + delta
	if next < 0 || next >= len(m.list.Items()) {
		return nil
	}
	m.list.Select(next)

	item, ok := m.list.SelectedItem().(postItem)
	if !ok {
		return nil
	}
	return m.openPost(item.post)
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

func (m *model) startInput(mode inputMode, prompt string) tea.Cmd {
	m.input = mode
	m.prompt.Prompt = prompt
	m.prompt.SetValue("")
	m.prompt.Width = max(m.width-len(prompt)-1, 1)
	return m.prompt.Focus()
}

func (m *model) cancelInput() {
	m.input = inputNone
	m.prompt.Blur()
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.cancelInput()
		return m, nil

	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyEnter:
		mode := m.input
		value := strings.TrimSpace(m.prompt.Value())
		m.cancelInput()
		if value == "" {
			return m, nil
		}

		switch mode {
		case inputSearch:
			m.query = value
			m.showBookmarks = false
			m.setPostsTitle()
			return m, searchPosts(m.ctx, m.queries, m.userID, value)

		case inputAddFeed:
			next, cmd := m.withStatus("Adding " + value + "…")
			return next, tea.Batch(cmd, addFeed(next.ctx, next.queries, next.userID, value))
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m model) confirmText() string {
	item, ok := m.feedList.SelectedItem().(feedItem)
	if !ok {
		return ""
	}
	return "Unfollow " + item.name + "? (y/n)"
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.confirming = false

	if msg.String() != "y" {
		return m.withStatus("Cancelled")
	}

	item, ok := m.feedList.SelectedItem().(feedItem)
	if !ok || item.id == uuid.Nil {
		return m, nil
	}
	return m, unfollowFeed(m.ctx, m.queries, m.userID, item.id, item.name)
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

	switch {
	case key.Matches(msg, m.keys.Copy):
		return m, copyToClipboard(m.selected.Url)

	case key.Matches(msg, m.keys.Next):
		return m, m.stepPost(1)

	case key.Matches(msg, m.keys.Prev):
		return m, m.stepPost(-1)
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

	case key.Matches(msg, m.keys.AddFeed):
		return m, m.startInput(inputAddFeed, "feed url: ")

	case key.Matches(msg, m.keys.Unfollow):
		item, ok := m.feedList.SelectedItem().(feedItem)
		if !ok {
			return m, nil
		}
		if item.id == uuid.Nil {
			return m.withStatus("Pick a feed to unfollow")
		}
		m.confirming = true
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
			return m, m.startInput(inputSearch, "search: ")

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
			m.showDetail = true
			return m, m.openPost(item.post)

		case key.Matches(msg, m.keys.Copy):
			post, ok := m.currentPost()
			if !ok {
				return m, nil
			}
			return m, copyToClipboard(post.Url)

		case key.Matches(msg, m.keys.Since):
			if m.showBookmarks || m.query != "" {
				return m.withStatus("Time range applies to feed posts only")
			}
			m.since = nextSince(m.since)
			m.setPostsTitle()
			label := "Showing all time"
			if l := sinceLabel(m.since); l != "" {
				label = "Showing the last " + l
			}
			next, cmd := m.withStatus(label)
			return next, tea.Batch(cmd, next.startLoad())

		case key.Matches(msg, m.keys.Unread):
			post, ok := m.currentPost()
			if !ok {
				return m, nil
			}
			read := !m.reads[post.ID]
			m.applyRead(post.ID, post.FeedID, read)
			label := "Marked unread"
			if read {
				label = "Marked read"
			}
			next, cmd := m.withStatus(label)
			return next, tea.Batch(cmd, setPostRead(next.ctx, next.queries, next.userID, post, read))

		case key.Matches(msg, m.keys.AllRead):
			ids := make([]uuid.UUID, 0, len(m.list.Items()))
			for _, it := range m.list.Items() {
				pi, ok := it.(postItem)
				if !ok || m.reads[pi.post.ID] {
					continue
				}
				ids = append(ids, pi.post.ID)
				m.applyRead(pi.post.ID, pi.post.FeedID, true)
			}
			return m, markAllRead(m.ctx, m.queries, m.userID, ids)

		case key.Matches(msg, m.keys.OnlyNew):
			if m.showBookmarks || m.query != "" {
				return m.withStatus("Unread filter applies to feed posts only")
			}
			m.unreadOnly = !m.unreadOnly
			label := "Showing all posts"
			if m.unreadOnly {
				label = "Showing unread only"
			}
			next, cmd := m.withStatus(label)
			return next, tea.Batch(cmd, next.startLoad())
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
	case m.confirming:
		line = m.confirmText()
	case m.input != inputNone:
		line = m.prompt.View()
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

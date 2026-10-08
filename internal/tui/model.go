package tui

import (
	"context"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/store"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

const (
	feedPanelWidth   = 26
	panelGutter      = 1
	minWidthForFeeds = 60
	allFeedsLabel    = "All feeds"
	postsTitle       = "Posts"
	bookmarksTitle   = "Bookmarks"
	readTitle        = "Read"

	sortDesc = "desc"
	sortAsc  = "asc"

	loadMoreThreshold = 5
)

type focusArea int

const (
	focusPosts focusArea = iota
	focusFeeds
)

type inputMode int

const (
	inputNone inputMode = iota
	inputSearch
	inputAddFeed
)

type screenMode int

const (
	screenList screenMode = iota
	screenDetail
	screenCatalog
)

type listSource int

const (
	sourceFeeds listSource = iota
	sourceBookmarks
	sourceRead
	sourceSearch
)

func (s listSource) derived() bool { return s != sourceFeeds }

type listFilter struct {
	feedID     uuid.UUID
	feedName   string
	query      string
	sortDir    string
	since      time.Duration
	unreadOnly bool
}

func (f listFilter) params() postFilter {
	p := postFilter{
		feedID:     f.feedID,
		sortDir:    f.sortDir,
		unreadOnly: f.unreadOnly,
	}
	if f.since > 0 {
		p.since = time.Now().Add(-f.since)
	}
	return p
}

type model struct {
	ctx      context.Context
	store    store.Store
	userName string

	keys        keyMap
	help        help.Model
	list        list.Model
	feedList    list.Model
	catalogList list.Model
	viewport    viewport.Model
	spinner     spinner.Model
	prompt      textinput.Model
	selected    database.Post

	bookmarks map[uuid.UUID]bool
	reads     map[uuid.UUID]bool
	unread    map[uuid.UUID]int
	picked    map[string]bool
	expanded  map[string]bool
	feeds     []database.GetFeedFollowsForUserRow

	source listSource
	filter listFilter

	offset      int32
	hasMore     bool
	loadingMore bool

	feedCount   int
	feedsLoaded bool

	screen      screenMode
	focus       focusArea
	width       int
	height      int
	feedWidth   int
	postsWidth  int
	status      string
	statusToken int

	input        inputMode
	openOnLoad   bool
	confirming   bool
	fetching     bool
	fetchingText bool
	loading      bool
	err          error
}

func newModel(ctx context.Context, st store.Store, user store.User, saved uiState) model {
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

	catalogDelegate := list.NewDefaultDelegate()
	catalogDelegate.ShowDescription = false
	catalogDelegate.SetSpacing(0)

	cl := list.New(nil, catalogDelegate, 0, 0)
	cl.Title = catalogTitle
	cl.SetShowStatusBar(false)
	cl.SetFilteringEnabled(false)
	cl.SetShowHelp(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	ti := textinput.New()
	ti.CharLimit = 500

	m := model{
		ctx:         ctx,
		store:       st,
		userName:    user.Name,
		keys:        defaultKeyMap(),
		help:        help.New(),
		list:        l,
		feedList:    fl,
		catalogList: cl,
		viewport:    viewport.New(0, 0),
		spinner:     sp,
		prompt:      ti,
		bookmarks:   make(map[uuid.UUID]bool),
		reads:       make(map[uuid.UUID]bool),
		unread:      make(map[uuid.UUID]int),
		picked:      make(map[string]bool),
		expanded:    expandedSet(saved.Expanded),
		filter: listFilter{
			feedID:     saved.feedUUID(),
			feedName:   saved.FeedName,
			sortDir:    saved.SortDir,
			since:      saved.since(),
			unreadOnly: saved.UnreadOnly,
		},
		loading: true,
		hasMore: true,
	}
	m.setPostsTitle()
	m.applyFocus()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		loadPosts(m.ctx, m.store, m.filter.params(), 0),
		loadBookmarks(m.ctx, m.store),
		loadReads(m.ctx, m.store),
		loadFeeds(m.ctx, m.store),
		loadUnreadCounts(m.ctx, m.store),
	)
}

func (m model) inDerivedView() bool { return m.source.derived() }

func (m model) withStatus(text string) (model, tea.Cmd) {
	m.statusToken++
	m.status = text
	return m, expireStatus(m.statusToken)
}

func (m model) currentPost() (database.Post, bool) {
	if m.screen == screenDetail {
		return m.selected, true
	}
	item, ok := m.list.SelectedItem().(postItem)
	if !ok {
		return database.Post{}, false
	}
	return item.post, true
}

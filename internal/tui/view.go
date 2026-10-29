package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	switch {
	case m.err != nil:
		return m.fatalView()
	case m.loading:
		return "\n  " + m.spinner.View() + " Loading posts...\n"
	case m.screen == screenCatalog:
		return m.catalogView()
	case m.screen == screenPalette:
		return m.paletteView()
	case m.screen == screenMenu:
		return m.menuView()
	case m.screen == screenDetail:
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
		m.feedPanel(),
		verticalRule(m.panelHeight()),
		m.postsPanel(),
	)
	return panels + "\n" + m.footer()
}

func (m model) panelHeight() int {
	return max(m.height-m.footerHeight(), 1)
}

func (m model) postsGutter() int {
	if m.feedWidth == 0 {
		return 0
	}
	return panelGutter
}

func (m model) feedPanel() string {
	return panelBox(m.feedList.View(), m.feedWidth, m.panelHeight(), 0)
}

func (m model) postsPanel() string {
	body := m.list.View()
	if len(m.list.Items()) == 0 {
		body = lipgloss.JoinVertical(
			lipgloss.Left,
			m.list.Styles.TitleBar.Render(m.list.Styles.Title.Render(m.list.Title)),
			emptyStateStyle.Render(m.emptyStateText()),
		)
	}
	return panelBox(body, m.postsWidth, m.panelHeight(), m.postsGutter())
}

func panelBox(content string, width, height, padLeft int) string {
	return lipgloss.NewStyle().
		PaddingLeft(padLeft).
		Width(width).
		MaxWidth(width).
		Height(height).
		MaxHeight(height).
		Render(content)
}

func (m model) emptyStateText() string {
	switch {
	case m.source == sourceBookmarks:
		return "No bookmarks yet.\nPress b on a post to save it."
	case m.source == sourceSearch:
		return "No posts match " + strconv.Quote(m.filter.query) + ".\nPress esc to go back."
	case m.feedsLoaded && m.feedCount == 0:
		return "You are not following any feeds.\nPress tab, then c to pick from the catalog."
	case m.filter.feedName != "":
		return "No posts stored for " + m.filter.feedName + " yet.\nFetch some with: gator agg 15m"
	default:
		return "No posts yet.\nFetch some with: gator agg 15m"
	}
}

func verticalRule(height int) string {
	return strings.TrimSuffix(strings.Repeat("│\n", height), "\n")
}

func (m model) detailHeader() string {
	return renderDetailHeader(m.selected, m.feedNames[m.selected.FeedID], m.viewport.Width, m.viewport.ScrollPercent())
}

func (m *model) fitDetail() {
	rows := strings.Count(m.detailHeader(), "\n")
	m.viewport.Height = max(m.height-rows-1-m.footerHeight(), 1)
}

func (m model) detailView() string {
	return m.detailHeader() +
		m.viewport.View() +
		"\n\n" + m.footer()
}

func (m model) currentBindings() []key.Binding {
	switch {
	case m.screen == screenCatalog:
		return m.keys.catalogHelp()
	case m.screen == screenPalette:
		return m.keys.paletteHelp()
	case m.screen == screenDetail:
		return m.keys.detailHelp()
	case m.focus == focusFeeds:
		return m.keys.feedsHelp()
	default:
		return m.keys.listHelp(m.feedWidth > 0, m.inDerivedView())
	}
}

func (m model) footerHeight() int {
	if !m.help.ShowAll {
		return 1
	}
	return lipgloss.Height(m.help.FullHelpView(m.keys.fullHelp(m.remote())))
}

func (m model) footer() string {
	var line string
	switch {
	case m.confirming:
		line = m.confirmText()
	case m.input != inputNone:
		line = m.prompt.View()
	case m.status != "":
		line = m.userTagged(m.status)
	case m.help.ShowAll:
		line = m.help.FullHelpView(m.keys.fullHelp(m.remote()))
	case m.fetching:
		line = m.userTagged(m.fetchLine())
	default:
		line = m.userTagged(m.help.ShortHelpView(m.currentBindings()))
	}

	return lipgloss.NewStyle().
		MaxWidth(m.width).
		Height(m.footerHeight()).
		MaxHeight(m.footerHeight()).
		Render(line)
}

func (m model) fetchLine() string {
	line := m.spinner.View() + "Fetching feeds"
	if m.fetchTotal == 0 {
		return line + "…"
	}
	return line + " " + strconv.Itoa(m.fetchDone) + "/" + strconv.Itoa(m.fetchTotal)
}

func (m model) userTagged(line string) string {
	for _, tag := range m.footerTags() {
		rendered := userTagStyle.Render(tag)
		gap := m.width - lipgloss.Width(line) - lipgloss.Width(rendered)
		if gap >= 1 {
			return line + strings.Repeat(" ", gap) + rendered
		}
	}
	return line
}

func (m model) footerTags() []string {
	if m.userName == "" {
		return nil
	}

	user := "@" + m.userName
	if !m.remote() {
		return []string{user}
	}

	state := m.syncState()
	lost, waiting := "", ""
	if state.Dropped > 0 {
		lost = strconv.Itoa(state.Dropped) + " lost"
	}
	if state.Pending > 0 {
		waiting = strconv.Itoa(state.Pending) + " " + remoteMark
	}

	var tags []string
	head := user + " · offline"
	if state.Online {
		head = user + " · " + serverHost(m.server)
	}
	switch {
	case lost != "":
		tags = append(tags, head+" · "+lost)
	case waiting != "":
		tags = append(tags, head+" · "+waiting)
	case !state.Online && !state.LastSync.IsZero():
		tags = append(tags, head+" · sync "+state.LastSync.Format("15:04"))
	}
	tags = append(tags, head)

	if host := serverHost(m.server); state.Online && hostWithoutPort(host) != host {
		tags = append(tags, user+" · "+hostWithoutPort(host))
	}
	switch {
	case lost != "":
		tags = append(tags, user+" · "+lost, user+" · "+strconv.Itoa(state.Dropped)+" "+brokenMark)
	case waiting != "":
		tags = append(tags, user+" · "+waiting)
	}

	mark := remoteMark
	if !state.Online || lost != "" {
		mark = brokenMark
	}
	return append(tags, user+" "+mark, user)
}

func (m *model) resize(w, h int) {
	m.width, m.height = w, h
	m.help.Width = w

	m.feedWidth = feedPanelWidth
	if w < minWidthForFeeds {
		m.feedWidth = 0
		m.focus = focusPosts
	}

	m.postsWidth = w - m.feedWidth
	if m.feedWidth > 0 {
		m.postsWidth--
	}

	feedListWidth, postsListWidth := m.feedWidth, m.postsWidth
	if m.feedWidth > 0 {
		feedListWidth -= panelGutter
		postsListWidth -= panelGutter
	}

	panelHeight := m.panelHeight()
	m.feedList.SetSize(max(feedListWidth, 1), panelHeight)
	m.catalogList.SetSize(w, panelHeight)
	m.paletteList.SetSize(w, panelHeight)
	m.list.SetSize(max(postsListWidth, 1), panelHeight)
	m.prompt.Width = max(w-len(m.prompt.Prompt)-1, 1)

	if m.screen == screenMenu {
		m.menu.SetSize(w, h)
	}

	m.viewport.Width = w
	m.fitDetail()
	if m.screen == screenDetail {
		m.viewport.SetContent(renderDetailBody(m.selected, m.viewport.Width))
	}
	m.applyFocus()
}

func (m *model) applyFocus() {
	m.list.Styles.Title = panelTitleStyle(m.focus == focusPosts)
	m.feedList.Styles.Title = panelTitleStyle(m.focus == focusFeeds)
}

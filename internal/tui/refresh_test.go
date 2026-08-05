package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestReloadReloadsCurrentView(t *testing.T) {
	m := loaded(t, fullPage("a"))
	m.offset = pageSize * 2

	m, cmd := step(t, m, press("r"))

	if cmd == nil {
		t.Fatal("r did not reload")
	}
	if m.offset != 0 {
		t.Errorf("offset = %d, want paging reset on reload", m.offset)
	}
	if !strings.Contains(m.status, "Reloading") {
		t.Errorf("status = %q, want a reloading message", m.status)
	}
}

func TestFetchRunsOnceAtATime(t *testing.T) {
	m := loaded(t, fullPage("a"))

	m, cmd := step(t, m, press("R"))
	if cmd == nil {
		t.Fatal("R did not start a fetch")
	}
	if !m.fetching {
		t.Fatal("fetching flag not set")
	}
	if !strings.Contains(m.status, "Fetching") {
		t.Errorf("status = %q, want a fetching message", m.status)
	}

	_, cmd = step(t, m, press("R"))
	if cmd != nil {
		t.Error("a second R started another fetch while one was running")
	}
}

func TestFetchResultReloadsAndReportsCounts(t *testing.T) {
	m := loaded(t, fullPage("a"))
	m, _ = step(t, m, press("R"))

	m, cmd := step(t, m, scrapedMsg{feeds: 12, saved: 7})

	if m.fetching {
		t.Error("fetching flag still set after the result arrived")
	}
	if cmd == nil {
		t.Fatal("fetch result did not reload the list")
	}
	if !strings.Contains(m.status, "7 new posts") {
		t.Errorf("status = %q, want the new post count", m.status)
	}
	if !strings.Contains(m.status, "12 feeds") {
		t.Errorf("status = %q, want the feed count", m.status)
	}
}

func TestScrapeSummary(t *testing.T) {
	cases := []struct {
		name string
		msg  scrapedMsg
		want string
	}{
		{"nothing due", scrapedMsg{}, "No feeds were due"},
		{"all fine", scrapedMsg{feeds: 3, saved: 5}, "5 new posts from 3 feeds"},
		{"some failed", scrapedMsg{feeds: 3, saved: 5, failed: 2}, "2 feed(s) failed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scrapeSummary(tc.msg); !strings.Contains(got, tc.want) {
				t.Errorf("scrapeSummary(%+v) = %q, want it to contain %q", tc.msg, got, tc.want)
			}
		})
	}
}

func TestFetchErrorClearsFlag(t *testing.T) {
	m := loaded(t, fullPage("a"))
	m, _ = step(t, m, press("R"))

	m, _ = step(t, m, errMsg{errors.New("mreza pukla")})

	if m.fetching {
		t.Error("a failed fetch left the fetching flag set")
	}
	if !strings.Contains(m.View(), "mreza pukla") {
		t.Errorf("error not shown:\n%s", m.View())
	}
}

func TestReloadKeepsFeedAndSort(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := withFeeds(t, loaded(t, fullPage("a")), feed)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = step(t, m, press("S"))

	before := m.sortDir
	m, _ = step(t, m, press("r"))

	if m.feedID != feed.FeedID {
		t.Error("reload lost the feed filter")
	}
	if m.sortDir != before {
		t.Errorf("reload changed sort: %q, want %q", m.sortDir, before)
	}
}

func TestFetchKeysIgnoredWhileTyping(t *testing.T) {
	m := loaded(t, fullPage("a"))

	m, _ = step(t, m, press("s"))
	m = typeText(t, m, "rR")

	if m.fetching {
		t.Error("R started a fetch while typing a search")
	}
	if got, want := m.search.Value(), "rR"; got != want {
		t.Errorf("search value = %q, want %q", got, want)
	}
}

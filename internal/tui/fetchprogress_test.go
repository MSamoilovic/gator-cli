package tui

import (
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/feeds"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

func fetching(t *testing.T, done, total int) model {
	t.Helper()

	m := loaded(t, fullPage("a"))
	m, _ = step(t, m, press("R"))
	m, _ = step(t, m, fetchProgressMsg{done: done, total: total})
	return m
}

func TestFetchProgressCountsUpInTheFooter(t *testing.T) {
	m := loaded(t, fullPage("a"))
	m, _ = step(t, m, press("R"))

	if got := m.footer(); !strings.Contains(got, "Fetching feeds") {
		t.Fatalf("footer = %q, want it to say a fetch is running before the first feed answers", got)
	}

	m, cmd := step(t, m, fetchProgressMsg{done: 37, total: 160})
	if cmd == nil {
		t.Error("the model did not wait for the next progress, so the counter would freeze at 37")
	}
	if got := m.footer(); !strings.Contains(got, "37/160") {
		t.Errorf("footer = %q, want the counter 37/160", got)
	}
}

func TestFetchProgressOutlivesTheStatusTimeout(t *testing.T) {
	m := fetching(t, 37, 160)

	m, _ = step(t, m, press("y"))
	m, _ = step(t, m, copiedMsg{url: "https://example.com"})
	if got := m.footer(); !strings.Contains(got, "Copied") {
		t.Fatalf("footer = %q, want the user's own action to interrupt the counter", got)
	}

	m, _ = step(t, m, statusExpiredMsg{token: m.statusToken})
	if got := m.footer(); !strings.Contains(got, "37/160") {
		t.Errorf("footer = %q, want the counter back once the status expired — a fetch runs far "+
			"longer than the 3s status", got)
	}
}

func TestFetchProgressLeavesWithTheResult(t *testing.T) {
	m := fetching(t, 160, 160)

	m, _ = step(t, m, scrapedMsg{feeds: 160, saved: 49})
	if m.fetchDone != 0 || m.fetchTotal != 0 {
		t.Errorf("the counter kept %d/%d, so the next fetch would start from stale numbers",
			m.fetchDone, m.fetchTotal)
	}
	if got := m.footer(); strings.Contains(got, "Fetching feeds") {
		t.Errorf("footer = %q, want the summary instead of a running fetch", got)
	}
}

func TestProgressArrivingAfterTheResultIsDropped(t *testing.T) {
	m := fetching(t, 159, 160)
	m, _ = step(t, m, scrapedMsg{feeds: 160, saved: 49})

	m, cmd := step(t, m, fetchProgressMsg{done: 160, total: 160})
	if cmd != nil {
		t.Error("a progress buffered past the result waited on a closed fetch, leaking the goroutine")
	}
	if strings.Contains(m.footer(), "160/160") {
		t.Errorf("footer = %q, want the summary to stand after the fetch ended", m.footer())
	}
}

func TestFetchProgressRunsTheSpinner(t *testing.T) {
	m := loaded(t, fullPage("a"))

	if _, cmd := step(t, m, spinner.TickMsg{}); cmd != nil {
		t.Error("the spinner ticks while nothing is running")
	}

	m, _ = step(t, m, press("R"))
	if _, cmd := step(t, m, spinner.TickMsg{}); cmd == nil {
		t.Error("the spinner stopped during a fetch, so the footer would sit still for 15 seconds")
	}
}

func TestTheFetchCounterFitsEightyColumnsWithTheUserTag(t *testing.T) {
	m := fetching(t, 160, 160)

	const tag = 6
	if w := lipgloss.Width(m.fetchLine()); w+tag >= m.width {
		t.Errorf("the fetch line is %d columns; with the %d-column @user tag it does not fit %d",
			w, tag, m.width)
	}
	if got := m.footer(); !strings.Contains(got, "@marko") {
		t.Errorf("footer = %q, want the user tag kept while fetching", got)
	}
}

func TestFetchProgressReachesTheModelFromTheStore(t *testing.T) {
	m, f := backed(t, "Go 1.26 is out")
	f.RefreshSteps = []feeds.Result{{Saved: 1}, {NotModified: true}, {Saved: 2}}

	_, got := pressing[fetchProgressMsg](t, m, "R")
	if got.total != len(f.RefreshSteps) {
		t.Errorf("progress = %d/%d, want a total of %d from the store — the model only knows the "+
			"feeds you follow, not the ones the aggregator fetches", got.done, got.total, len(f.RefreshSteps))
	}
	if got.done < 1 {
		t.Errorf("progress = %d/%d, want at least one feed counted", got.done, got.total)
	}
}

package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/store/storetest"

	tea "github.com/charmbracelet/bubbletea"
)

const testServer = "https://gator.example.test"

func remoteReady(t *testing.T, f *storetest.Fake) model {
	t.Helper()
	m, _ := step(t, newModel(t.Context(), f, f.Identity, uiState{SortDir: sortDesc}, testServer), tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, postsLoadedMsg{posts: []store.Post{testPost("Prvi")}})
	return m
}

func TestExplainTurnsStoreErrorsIntoSentences(t *testing.T) {
	m := model{server: testServer}

	tests := []struct {
		name string
		err  error
		want string
	}{
		{"istekao token", store.ErrUnauthorized, "gator login --server " + testServer},
		{"token zavijen u kontekst", fmt.Errorf("loading posts: %w", store.ErrUnauthorized), "gator login --server " + testServer},
		{"limit plana", fmt.Errorf("%w: 50 feeds", store.ErrPlanLimit), "Plan limit reached"},
		{"server nedostupan", fmt.Errorf("reaching %s: %w: %w", testServer, store.ErrUnreachable, errors.New("dial tcp: refused")), "Cannot reach " + testServer},
		{"refresh nije podrzan", fmt.Errorf("aggregator: %w", errors.ErrUnsupported), "refreshes feeds on its own"},
		{"neznata greska zadrzava tekst", errors.New("baza pukla"), "Error: baza pukla"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.explain(tt.err)
			if !strings.Contains(got, tt.want) {
				t.Errorf("explain() = %q, want it to contain %q", got, tt.want)
			}
			if strings.Contains(got, "dial tcp") {
				t.Errorf("explain() leaked the transport error: %q", got)
			}
		})
	}
}

func TestExplainNamesNoServerWhenLocal(t *testing.T) {
	m := model{}

	got := m.explain(fmt.Errorf("%w", store.ErrUnreachable))
	if strings.Contains(got, "  ") || strings.Contains(got, "reach  ") {
		t.Errorf("explain() left a hole where the server name goes: %q", got)
	}
	if !strings.Contains(got, "the server") {
		t.Errorf("explain() = %q, want a readable fallback name", got)
	}
}

func TestExpiredTokenIsShownAsInstructionsNotAStackTrace(t *testing.T) {
	f := storetest.New("marko")
	f.Err = fmt.Errorf("loading posts: %w", store.ErrUnauthorized)

	m, _ := step(t, newModel(t.Context(), f, f.Identity, uiState{SortDir: sortDesc}, testServer), tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, errMsg{f.Err})

	view := m.View()
	if !strings.Contains(view, "gator login --server "+testServer) {
		t.Errorf("the fatal screen does not say how to sign in again:\n%s", view)
	}
	if strings.Contains(view, "loading posts:") {
		t.Errorf("the fatal screen shows the wrapped Go error:\n%s", view)
	}
	if !strings.Contains(view, "q") {
		t.Errorf("the fatal screen does not say how to leave:\n%s", view)
	}
}

func TestAnUnreachableServerOffersARetryRatherThanOnlyQuit(t *testing.T) {
	f := storetest.New("marko")
	err := fmt.Errorf("reaching %s: %w: %w", testServer, store.ErrUnreachable, errors.New("dial tcp 10.0.0.1:443: i/o timeout"))

	m, _ := step(t, newModel(t.Context(), f, f.Identity, uiState{SortDir: sortDesc}, testServer), tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, errMsg{err})

	view := m.View()
	if !strings.Contains(view, "Press r") {
		t.Errorf("an unreachable server should offer a retry:\n%s", view)
	}
	if strings.Contains(view, "i/o timeout") {
		t.Errorf("the fatal screen leaked the dial error:\n%s", view)
	}
}

func TestATransientErrorBecomesAStatusLineNotARawError(t *testing.T) {
	f := storetest.New("marko")
	f.Post("Prvi", "https://example.test/1")

	m := remoteReady(t, f)
	m, _ = step(t, m, errMsg{fmt.Errorf("saving: %w", store.ErrUnauthorized)})

	if m.err != nil {
		t.Fatal("a transient error after the first load should not blank the screen")
	}
	if !strings.Contains(m.status, "gator login") {
		t.Errorf("status = %q, want the sign-in instruction", m.status)
	}
	if strings.Contains(m.status, "saving:") {
		t.Errorf("status = %q, want no wrapped Go error", m.status)
	}
}

func TestFetchIsRefusedWithoutTouchingTheServerWhenRemote(t *testing.T) {
	f := storetest.New("marko")
	f.Post("Prvi", "https://example.test/1")
	f.RefreshErr = errors.New("the store must not be called at all")

	m := remoteReady(t, f)
	m, cmd := step(t, m, press("R"))

	if cmd == nil {
		t.Fatal("pressing R produced no command at all, not even a status")
	}
	if m.fetching {
		t.Error("R set the fetching flag in remote mode")
	}
	if !strings.Contains(m.status, "refreshes feeds on its own") {
		t.Errorf("status = %q, want an explanation of who fetches", m.status)
	}
	if !strings.Contains(m.status, "press r") {
		t.Errorf("status = %q, want it to point at the key that does work", m.status)
	}
}

func TestFetchStillWorksLocally(t *testing.T) {
	f := storetest.New("marko")
	f.Post("Prvi", "https://example.test/1")
	f.Refreshed = store.RefreshResult{Feeds: 3, Saved: 7}

	m, _ := step(t, newModel(t.Context(), f, f.Identity, uiState{SortDir: sortDesc}, ""), tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, postsLoadedMsg{posts: []store.Post{testPost("Prvi")}})

	m, got := pressing[scrapedMsg](t, m, "R")
	if !m.fetching {
		t.Error("R did not start a fetch on a local store")
	}
	if got.saved != 7 || got.feeds != 3 {
		t.Errorf("scrape result = %+v, want the store's numbers", got)
	}
}

func TestRemoteModeHidesFetchFromThePaletteAndTheFullHelp(t *testing.T) {
	f := storetest.New("marko")
	f.Post("Prvi", "https://example.test/1")

	local, _ := step(t, newModel(t.Context(), f, f.Identity, uiState{SortDir: sortDesc}, ""), tea.WindowSizeMsg{Width: 80, Height: 24})
	if !strings.Contains(local.help.FullHelpView(local.keys.fullHelp(false)), "R fetch") {
		t.Fatal("the local ? help does not list R fetch, so this test proves nothing")
	}

	m := remoteReady(t, f)

	if full := m.help.FullHelpView(m.keys.fullHelp(m.remote())); strings.Contains(full, "R fetch") {
		t.Errorf("? help still offers R fetch against a server:\n%s", full)
	}

	m = openPalette(t, m)
	for _, label := range paletteLabels(m) {
		if strings.Contains(label, "Fetch feeds now") {
			t.Error("the palette still offers Fetch feeds now against a server")
		}
	}
}

func TestThePaletteKeepsEverythingTheServerCanDo(t *testing.T) {
	f := storetest.New("marko")
	f.Post("Prvi", "https://example.test/1")

	m := openPalette(t, remoteReady(t, f))
	labels := strings.Join(paletteLabels(m), "\n")

	for _, want := range []string{"Add a feed by URL", "Browse the feed catalog", "Move this feed to a folder", "Reload the list"} {
		if !strings.Contains(labels, want) {
			t.Errorf("remote palette dropped %q, which the server does support:\n%s", want, labels)
		}
	}
}

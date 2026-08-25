package cli

import (
	"testing"
	"time"

	"gator-cli/internal/database"
)

func statRow(name string, posts, recent, read int64, published time.Time) database.GetFeedStatsForUserRow {
	return database.GetFeedStatsForUserRow{
		FeedName:      name,
		PostCount:     posts,
		RecentCount:   recent,
		ReadCount:     read,
		LastPublished: published,
	}
}

func TestAgo(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		when time.Time
		want string
	}{
		{"nulto vreme je nikad", time.Time{}, neverLabel},
		{"minuti", now.Add(-20 * time.Minute), "20m"},
		{"sati", now.Add(-5 * time.Hour), "5h"},
		{"dani", now.Add(-4 * 24 * time.Hour), "4d"},
		{"meseci", now.Add(-40 * 24 * time.Hour), "1mo"},
		{"godine", now.Add(-400 * 24 * time.Hour), "1y"},
		{"granica sata", now.Add(-59 * time.Minute), "59m"},
		{"granica dana", now.Add(-23 * time.Hour), "23h"},
		// Feed koji objavi datum u buducnosti ne sme da izgleda kao najstariji.
		{"buducnost", now.Add(2 * time.Hour), "just now"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ago(tt.when, now); got != tt.want {
				t.Errorf("ago() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPercent(t *testing.T) {
	tests := []struct {
		part, whole int64
		want        string
	}{
		{0, 0, "0%"}, // feed bez postova ne deli nulom
		{0, 100, "0%"},
		{50, 100, "50%"},
		{100, 100, "100%"},
		// Zaokruzivanje na nulu bi tvrdilo da nista nije procitano.
		{3, 1126, "<1%"},
		{1, 100000, "<1%"},
	}

	for _, tt := range tests {
		if got := percent(tt.part, tt.whole); got != tt.want {
			t.Errorf("percent(%d, %d) = %q, want %q", tt.part, tt.whole, got, tt.want)
		}
	}
}

func TestPlural(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 feeds"},
		{1, "1 feed"},
		{2, "2 feeds"},
	}

	for _, tt := range tests {
		if got := plural(tt.n, "feed"); got != tt.want {
			t.Errorf("plural(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestStatsOrderRejectsUnknownNames(t *testing.T) {
	if _, err := statsOrder("nonsense"); err == nil {
		t.Fatal("expected an error for an unknown sort, got nil")
	}
}

func TestStatsOrderKnowsEveryNameItAdvertises(t *testing.T) {
	// Imena iz poruke o gresci moraju stvarno da rade.
	for _, name := range []string{"posts", "week", "read", "unread", "stale", "name"} {
		if _, err := statsOrder(name); err != nil {
			t.Errorf("statsOrder(%q): %v", name, err)
		}
	}
}

func TestStatsOrderByVolume(t *testing.T) {
	big := statRow("Lobsters", 216, 23, 0, time.Now())
	small := statRow("Go Blog", 11, 1, 0, time.Now())

	less, err := statsOrder("posts")
	if err != nil {
		t.Fatal(err)
	}
	if !less(big, small) {
		t.Error("the louder feed should come first")
	}
	if less(small, big) {
		t.Error("ordering is not consistent")
	}
}

func TestStatsOrderByStalePutsSilentFeedsFirst(t *testing.T) {
	now := time.Now()
	fresh := statRow("CNBC", 89, 50, 0, now.Add(-3*24*time.Hour))
	quiet := statRow("Julia Evans", 20, 0, 0, now.Add(-40*24*time.Hour))
	// Feed bez ijednog posta nosi nulto vreme i time je najstariji od svih.
	empty := statRow("Rust Blog", 0, 0, 0, time.Time{})

	less, err := statsOrder("stale")
	if err != nil {
		t.Fatal(err)
	}
	if !less(quiet, fresh) {
		t.Error("a feed that went quiet should sort before an active one")
	}
	if !less(empty, quiet) {
		t.Error("a feed with no posts at all should sort first")
	}
}

func TestStatsOrderByNameIgnoresCase(t *testing.T) {
	less, err := statsOrder("name")
	if err != nil {
		t.Fatal(err)
	}
	if !less(statRow("ars technica", 0, 0, 0, time.Time{}), statRow("BBC", 0, 0, 0, time.Time{})) {
		t.Error("lowercase names must not sort after uppercase ones")
	}
}

func TestStatsOrderByUnread(t *testing.T) {
	mostlyRead := statRow("A", 100, 0, 95, time.Time{})
	untouched := statRow("B", 50, 0, 0, time.Time{})

	less, err := statsOrder("unread")
	if err != nil {
		t.Fatal(err)
	}
	// 50 nepročitanih je više od 5, iako je ukupan broj manji.
	if !less(untouched, mostlyRead) {
		t.Error("unread ordering compares the backlog, not the total")
	}
}

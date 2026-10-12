package feeds

import (
	"testing"
	"time"
)

func TestPruneRefusesANonPositiveRetention(t *testing.T) {
	for _, retention := range []time.Duration{0, -time.Hour} {
		if _, err := Prune(t.Context(), nil, retention, DefaultKeepPerFeed); err == nil {
			t.Errorf("Prune(%s) returned no error, want one before it touches the database", retention)
		}
	}
}

func TestPruneRefusesANegativeKeepPerFeed(t *testing.T) {
	if _, err := Prune(t.Context(), nil, DefaultRetention, -1); err == nil {
		t.Error("Prune(keepPerFeed=-1) returned no error, want one before it touches the database")
	}
}

func TestDefaultRetentionIsSeventyTwoHours(t *testing.T) {
	if got, want := DefaultRetention, 72*time.Hour; got != want {
		t.Errorf("DefaultRetention = %s, want %s", got, want)
	}
}

func TestDefaultKeepPerFeedIsTen(t *testing.T) {
	if got, want := DefaultKeepPerFeed, 10; got != want {
		t.Errorf("DefaultKeepPerFeed = %d, want %d", got, want)
	}
}

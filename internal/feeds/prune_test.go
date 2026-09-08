package feeds

import (
	"testing"
	"time"
)

func TestPruneRefusesANonPositiveRetention(t *testing.T) {
	for _, retention := range []time.Duration{0, -time.Hour} {
		if _, err := Prune(t.Context(), nil, retention); err == nil {
			t.Errorf("Prune(%s) returned no error, want one before it touches the database", retention)
		}
	}
}

func TestDefaultRetentionIsSeventyTwoHours(t *testing.T) {
	if got, want := DefaultRetention, 72*time.Hour; got != want {
		t.Errorf("DefaultRetention = %s, want %s", got, want)
	}
}

package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const stateFileName = ".gator-state.json"

// uiState je ono sto TUI pamti izmedju pokretanja. Drzi se odvojeno od
// ~/.gatorconfig.json da se korisnicki config ne zagadi UI sitnicama.
type uiState struct {
	FeedID     string `json:"feed_id,omitempty"`
	FeedName   string `json:"feed_name,omitempty"`
	SortDir    string `json:"sort_dir,omitempty"`
	UnreadOnly bool   `json:"unread_only,omitempty"`
	SinceHours int    `json:"since_hours,omitempty"`
}

func statePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, stateFileName), nil
}

// loadState nikad ne prijavljuje gresku: neispravno ili nepostojece stanje
// samo znaci da se krece od pocetka, a ne da TUI ne sme da se otvori.
func loadState() uiState {
	var s uiState

	path, err := statePath()
	if err != nil {
		return s.sanitized()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return s.sanitized()
	}

	if err := json.Unmarshal(data, &s); err != nil {
		return uiState{}.sanitized()
	}
	return s.sanitized()
}

// sanitized odbacuje vrednosti koje bi model dovele u stanje koje sam ne moze
// da napravi — npr. rucno izmenjen fajl sa nepoznatim sortom.
func (s uiState) sanitized() uiState {
	if s.SortDir != sortAsc && s.SortDir != sortDesc {
		s.SortDir = sortDesc
	}
	if _, err := uuid.Parse(s.FeedID); err != nil {
		s.FeedID, s.FeedName = "", ""
	}
	if sinceLabel(time.Duration(s.SinceHours)*time.Hour) == "" {
		s.SinceHours = 0
	}
	return s
}

func (s uiState) save() error {
	path, err := statePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.sanitized(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (s uiState) feedUUID() uuid.UUID {
	id, err := uuid.Parse(s.FeedID)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func (s uiState) since() time.Duration {
	return time.Duration(s.SinceHours) * time.Hour
}

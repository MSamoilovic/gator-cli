package offline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
)

const (
	FileName = ".gator-cache.json"
	maxPosts = 200
)

type snapshot struct {
	Server     string               `json:"server"`
	SyncedAt   time.Time            `json:"synced_at"`
	User       *wire.User           `json:"user,omitempty"`
	Posts      []wire.Post          `json:"posts,omitempty"`
	Bookmarked []wire.Post          `json:"bookmarked,omitempty"`
	Subs       []store.Subscription `json:"subscriptions,omitempty"`
	Unread     []store.UnreadCount  `json:"unread,omitempty"`
	Read       []uuid.UUID          `json:"read_ids,omitempty"`
	Saved      []uuid.UUID          `json:"bookmark_ids,omitempty"`
}

func load(path, server string) snapshot {
	data, err := os.ReadFile(path)
	if err != nil {
		return snapshot{Server: server}
	}

	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil || snap.Server != server {
		return snapshot{Server: server}
	}
	return snap
}

func (snap snapshot) save(path string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), FileName+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if err := json.NewEncoder(tmp).Encode(snap); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (snap *snapshot) mergePosts(posts []store.Post) {
	merged := wire.ToPosts(snap.Posts)
	at := make(map[uuid.UUID]int, len(merged))
	for i, p := range merged {
		at[p.ID] = i
	}

	for _, p := range posts {
		if i, ok := at[p.ID]; ok {
			merged[i] = p
			continue
		}
		at[p.ID] = len(merged)
		merged = append(merged, p)
	}

	slices.SortStableFunc(merged, func(a, b store.Post) int {
		return postTime(b).Compare(postTime(a))
	})
	if len(merged) > maxPosts {
		merged = merged[:maxPosts]
	}
	snap.Posts = wire.FromPosts(merged)
}

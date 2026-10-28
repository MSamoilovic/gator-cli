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
	Dropped    int                  `json:"dropped,omitempty"`
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

func (snap *snapshot) apply(c change) {
	switch c.Op {
	case opRead:
		snap.setRead(c.PostID, c.Value)
	case opReadAll:
		for _, id := range c.PostIDs {
			snap.setRead(id, true)
		}
	case opBookmark:
		snap.setSaved(c.PostID, c.Value)
	case opCategorize:
		for i := range snap.Subs {
			if snap.Subs[i].FeedUrl == c.URL {
				snap.Subs[i].Category = c.Category
			}
		}
	}
}

func (snap *snapshot) setRead(id uuid.UUID, read bool) {
	switch had := slices.Contains(snap.Read, id); {
	case read && !had:
		snap.Read = append(snap.Read, id)
		snap.countUnread(id, -1)
	case !read && had:
		snap.Read = slices.DeleteFunc(snap.Read, func(x uuid.UUID) bool { return x == id })
		snap.countUnread(id, 1)
	}
}

func (snap *snapshot) setSaved(id uuid.UUID, saved bool) {
	switch had := slices.Contains(snap.Saved, id); {
	case saved && !had:
		snap.Saved = append(snap.Saved, id)
		if i := slices.IndexFunc(snap.Posts, func(p wire.Post) bool { return p.ID == id }); i >= 0 {
			snap.Bookmarked = append(snap.Bookmarked, snap.Posts[i])
		}
	case !saved && had:
		snap.Saved = slices.DeleteFunc(snap.Saved, func(x uuid.UUID) bool { return x == id })
		snap.Bookmarked = slices.DeleteFunc(snap.Bookmarked, func(p wire.Post) bool { return p.ID == id })
	}
}

func (snap *snapshot) countUnread(postID uuid.UUID, delta int64) {
	i := slices.IndexFunc(snap.Posts, func(p wire.Post) bool { return p.ID == postID })
	if i < 0 {
		return
	}

	feedID := snap.Posts[i].FeedID
	for j := range snap.Unread {
		if snap.Unread[j].FeedID == feedID {
			snap.Unread[j].Unread = max(snap.Unread[j].Unread+delta, 0)
			return
		}
	}
	if delta > 0 {
		snap.Unread = append(snap.Unread, store.UnreadCount{FeedID: feedID, Unread: delta})
	}
}

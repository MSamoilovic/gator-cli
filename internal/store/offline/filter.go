package offline

import (
	"slices"
	"strings"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
)

func (snap snapshot) selectPosts(q store.PostQuery) []store.Post {
	read := idSet(snap.Read)
	names := make(map[uuid.UUID]string, len(snap.Subs))
	for _, sub := range snap.Subs {
		names[sub.FeedID] = strings.ToLower(sub.FeedName)
	}

	needle := strings.ToLower(q.Query)
	feedName := strings.ToLower(q.FeedName)
	since := wallClock(q.Since)

	out := make([]store.Post, 0, len(snap.Posts))
	for _, p := range wire.ToPosts(snap.Posts) {
		switch {
		case q.FeedID != uuid.Nil && p.FeedID != q.FeedID:
			continue
		case feedName != "" && !strings.Contains(names[p.FeedID], feedName):
			continue
		case needle != "" && !mentions(p, needle):
			continue
		case q.Read == store.ReadUnread && read[p.ID]:
			continue
		case q.Read == store.ReadOnly && !read[p.ID]:
			continue
		case !q.Since.IsZero() && postTime(p).Before(since):
			continue
		}
		out = append(out, p)
	}

	slices.SortStableFunc(out, func(a, b store.Post) int {
		if strings.EqualFold(q.SortDir, "asc") {
			return postTime(a).Compare(postTime(b))
		}
		return postTime(b).Compare(postTime(a))
	})

	if int(q.Offset) >= len(out) {
		return nil
	}
	out = out[q.Offset:]
	if q.Limit > 0 && int(q.Limit) < len(out) {
		out = out[:q.Limit]
	}
	return out
}

func mentions(p store.Post, needle string) bool {
	return strings.Contains(strings.ToLower(p.Title), needle) ||
		strings.Contains(strings.ToLower(p.Description.String), needle)
}

func postTime(p store.Post) time.Time {
	if p.PublishedAt.Valid {
		return p.PublishedAt.Time
	}
	return p.CreatedAt
}

func wallClock(t time.Time) time.Time {
	local := t.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(),
		local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), time.UTC)
}

func idSet(ids []uuid.UUID) map[uuid.UUID]bool {
	set := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

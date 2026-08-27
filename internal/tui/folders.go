package tui

import (
	"sort"

	"gator-cli/internal/database"

	"github.com/google/uuid"
)

const rootFolder = "(uncategorized)"

type folder struct {
	name  string
	feeds []database.GetFeedFollowsForUserRow
}

func groupFeeds(rows []database.GetFeedFollowsForUserRow) []folder {
	grouped := make(map[string][]database.GetFeedFollowsForUserRow)
	for _, r := range rows {
		grouped[r.Category] = append(grouped[r.Category], r)
	}

	names := make([]string, 0, len(grouped))
	for name := range grouped {
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if _, ok := grouped[""]; ok {
		names = append(names, "")
	}

	folders := make([]folder, 0, len(names))
	for _, name := range names {
		feeds := grouped[name]
		sort.Slice(feeds, func(i, j int) bool { return feeds[i].FeedName < feeds[j].FeedName })

		label := name
		if label == "" {
			label = rootFolder
		}
		folders = append(folders, folder{name: label, feeds: feeds})
	}
	return folders
}

func hasFolders(rows []database.GetFeedFollowsForUserRow) bool {
	for _, r := range rows {
		if r.Category != "" {
			return true
		}
	}
	return false
}

func folderOf(rows []database.GetFeedFollowsForUserRow, feedID uuid.UUID) string {
	for _, r := range rows {
		if r.FeedID != feedID {
			continue
		}
		if r.Category == "" {
			return rootFolder
		}
		return r.Category
	}
	return ""
}

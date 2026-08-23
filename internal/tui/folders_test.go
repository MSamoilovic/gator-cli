package tui

import (
	"testing"

	"gator-cli/internal/database"

	"github.com/google/uuid"
)

func follow(name, category string) database.GetFeedFollowsForUserRow {
	return database.GetFeedFollowsForUserRow{
		FeedID:   uuid.New(),
		FeedName: name,
		FeedUrl:  "https://example.test/" + name,
		Category: category,
	}
}

func folderNames(folders []folder) []string {
	names := make([]string, len(folders))
	for i, f := range folders {
		names[i] = f.name
	}
	return names
}

func feedNames(f folder) []string {
	names := make([]string, len(f.feeds))
	for i, r := range f.feeds {
		names[i] = r.FeedName
	}
	return names
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestGroupFeedsSortsFoldersWithRootLast(t *testing.T) {
	// Isti redosled kao `gator following` i `gator export`: folderi azbucno,
	// nekategorisani na kraju.
	rows := []database.GetFeedFollowsForUserRow{
		follow("BBC", ""),
		follow("XKCD", "Comics"),
		follow("Ars", "Tech"),
		follow("ESPN", ""),
		follow("SMBC", "Comics"),
	}

	got := folderNames(groupFeeds(rows))
	want := []string{"Comics", "Tech", rootFolder}
	if !equal(got, want) {
		t.Errorf("folders = %v, want %v", got, want)
	}
}

func TestGroupFeedsSortsFeedsByName(t *testing.T) {
	rows := []database.GetFeedFollowsForUserRow{
		follow("Zed", "Tech"),
		follow("Ars", "Tech"),
		follow("Mono", "Tech"),
	}

	folders := groupFeeds(rows)
	if len(folders) != 1 {
		t.Fatalf("got %d folders, want 1", len(folders))
	}
	if got, want := feedNames(folders[0]), []string{"Ars", "Mono", "Zed"}; !equal(got, want) {
		t.Errorf("feeds = %v, want %v", got, want)
	}
}

func TestGroupFeedsWithNoCategories(t *testing.T) {
	rows := []database.GetFeedFollowsForUserRow{follow("BBC", ""), follow("ESPN", "")}

	got := folderNames(groupFeeds(rows))
	if want := []string{rootFolder}; !equal(got, want) {
		t.Errorf("folders = %v, want %v", got, want)
	}
}

func TestGroupFeedsOnNothing(t *testing.T) {
	if got := groupFeeds(nil); len(got) != 0 {
		t.Errorf("groupFeeds(nil) = %v, want none", folderNames(got))
	}
}

func TestHasFolders(t *testing.T) {
	tests := []struct {
		name string
		rows []database.GetFeedFollowsForUserRow
		want bool
	}{
		{"nista", nil, false},
		{"sve nekategorisano", []database.GetFeedFollowsForUserRow{follow("BBC", ""), follow("ESPN", "")}, false},
		{"jedan kategorisan", []database.GetFeedFollowsForUserRow{follow("BBC", ""), follow("Ars", "Tech")}, true},
		{"svi kategorisani", []database.GetFeedFollowsForUserRow{follow("Ars", "Tech")}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasFolders(tt.rows); got != tt.want {
				t.Errorf("hasFolders() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFolderOf(t *testing.T) {
	tech := follow("Ars", "Tech")
	root := follow("BBC", "")
	rows := []database.GetFeedFollowsForUserRow{tech, root}

	if got := folderOf(rows, tech.FeedID); got != "Tech" {
		t.Errorf("folderOf(categorised) = %q, want Tech", got)
	}
	// Nekategorisan feed se vraca pod imenom pod kojim je i prikazan.
	if got := folderOf(rows, root.FeedID); got != rootFolder {
		t.Errorf("folderOf(uncategorised) = %q, want %q", got, rootFolder)
	}
	if got := folderOf(rows, uuid.New()); got != "" {
		t.Errorf("folderOf(unknown) = %q, want empty", got)
	}
}

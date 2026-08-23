package tui

import (
	"sort"

	"gator-cli/internal/database"

	"github.com/google/uuid"
)

// rootFolder je ono sto se prikazuje umesto prazne kategorije. Namerno je isto
// kao rootLabel u `gator following` — isti pojam ne sme da ima dva imena.
const rootFolder = "(uncategorized)"

// folder je jedna grupa feedova u panelu.
type folder struct {
	name  string
	feeds []database.GetFeedFollowsForUserRow
}

// groupFeeds slaze feedove u foldere: folderi azbucno, nekategorisani na kraju,
// feedovi po imenu unutar svakog. Isti redosled kao `gator following` i `gator
// export`, da panel i CLI ne pricaju razlicito o istoj stvari.
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

// hasFolders kaze da li uopste ima sta da se grupise. Korisnik koji nista nije
// kategorisao dobija ravan spisak — jedno jedino "(uncategorized)" zaglavlje
// iznad svega je cista buka.
func hasFolders(rows []database.GetFeedFollowsForUserRow) bool {
	for _, r := range rows {
		if r.Category != "" {
			return true
		}
	}
	return false
}

// folderOf nalazi folder u kom feed stoji, pod imenom pod kojim je prikazan.
// Prazan rezultat znaci da se feed ne prati.
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

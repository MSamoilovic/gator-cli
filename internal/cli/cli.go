// Package cli je sloj komandne linije: registar komandi, njihovi handleri i
// stanje koje dele. Sve sto handler radi je parsiranje argumenata i ispis —
// logika koju deli sa TUI-jem zivi u internal/feeds i internal/catalog.
package cli

import (
	"gator-cli/internal/config"
	"gator-cli/internal/database"
)

// state je ono sto svaki handler dobija: veza sa bazom i procitan config.
type state struct {
	Db  *database.Queries
	Cfg *config.Config
}

// entry je jedna komanda sa svime sto se o njoj zna: kako se zove, sta uzima,
// sta radi i ko sme da je pozove. Iz iste tabele se pravi registar, birac
// komandi i ispis `gator help`, pa ne moze da se desi da komanda postoji a da
// je u meniju nema — ili obrnuto.
type entry struct {
	name    string
	args    string // "[name] <url>"; prazno kad komanda ne uzima argumente
	summary string
	group   string

	// Tacno jedno od run i runAuth je postavljeno. runAuth je i jedini izvor
	// istine o tome trazi li komanda prijavu: iz njega se izvodi i umotavanje
	// u middleware i to da li se komanda nudi izlogovanom korisniku.
	run     func(*state, command) error
	runAuth func(*state, command, database.User) error

	guest  bool // nudi se i kad korisnik nije prijavljen
	hidden bool // postoji, ali se ne nudi u meniju
}

func (e entry) needsLogin() bool { return e.runAuth != nil }

func (e entry) handler() func(*state, command) error {
	if e.runAuth != nil {
		return middlewareLoggedIn(e.runAuth)
	}
	return e.run
}

// allCommands je poredjano po poslu, ne azbucno — mapa komandi redosled ionako
// ne bi sacuvala. Citanje je prvo jer je to ono zbog cega se gator i otvara.
//
// Funkcija je, a ne promenljiva, jer `help` ispisuje bas ovu tabelu: kao
// promenljiva bi tabela zavisila od handlera koji zavisi od nje.
func allCommands() []entry {
	return []entry{
		{group: "reading", name: "tui", summary: "Open the interactive reader", runAuth: handlerTUI},
		{group: "reading", name: "browse", args: "[flags]", summary: "Read posts; --no-tui prints them instead", runAuth: handlerBrowse},
		{group: "reading", name: "search", args: "<query>", summary: "Search post titles and bodies", runAuth: handlerSearch},
		{group: "reading", name: "bookmarks", summary: "List saved posts", runAuth: handlerBookmarks},
		{group: "reading", name: "bookmark", args: "<url>", summary: "Save a post", runAuth: handlerBookmark},
		{group: "reading", name: "unbookmark", args: "<url>", summary: "Remove a saved post", runAuth: handlerUnbookmark},

		{group: "feeds", name: "discover", args: "[category]", summary: "Pick feeds from the built-in catalog", runAuth: handlerDiscover},
		{group: "feeds", name: "following", summary: "List the feeds you follow, grouped by folder", runAuth: handlerFollowing},
		{group: "feeds", name: "addfeed", args: "[name] <url>", summary: "Add a feed and follow it", runAuth: handlerAddFeed},
		{group: "feeds", name: "feeds", summary: "List every feed in the database", run: handlerFeeds},
		{group: "feeds", name: "stats", args: "[flags]", summary: "Which feeds you actually read, and which just arrive", runAuth: handlerStats},
		{group: "feeds", name: "follow", args: "<url>", summary: "Follow a feed someone else added", runAuth: handlerFollow},
		{group: "feeds", name: "unfollow", args: "<url>", summary: "Stop following a feed", runAuth: handlerUnfollow},
		{group: "feeds", name: "categorize", args: "<url> <folder>", summary: "Move a feed into a folder", runAuth: handlerCategorize},
		{group: "feeds", name: "import", args: "<file>", summary: "Follow everything in an OPML file (- for stdin)", runAuth: handlerImport},
		{group: "feeds", name: "export", args: "[file]", summary: "Write your subscriptions out as OPML", runAuth: handlerExport},

		{group: "aggregation", name: "agg", args: "<duration>", summary: "Fetch every feed in a loop, e.g. 15m", run: handlerAgg},
		{group: "aggregation", name: "supervise", args: "<duration>", summary: "Keep agg running, restart it on crash", run: handlerSupervise},

		{group: "account", name: "register", args: "<username>", summary: "Create a new user and log in", run: handlerRegister, guest: true},
		{group: "account", name: "login", args: "<username>", summary: "Log in as an existing user", run: handlerLogin, guest: true},
		{group: "account", name: "users", summary: "List all users", run: handlerUsers, guest: true},
		{group: "account", name: "reset", summary: "Delete all users", run: handlerReset, hidden: true},

		{group: "other", name: "help", summary: "Print the list of commands", run: handlerHelp, guest: true},
	}
}

// Run izvrsi jednu komandu iz args, gde je args[0] njeno ime. Bez argumenata
// otvara birac komandi umesto da odmah padne. Greska se vraca pozivaocu;
// nijedan handler ne zove os.Exit sam.
func Run(cfg *config.Config, db *database.Queries, args []string) error {
	s := state{Cfg: cfg, Db: db}
	cmds := defaultCommands()

	if len(args) == 0 {
		return runMenu(&s, cmds)
	}
	return cmds.run(&s, command{Name: args[0], Args: args[1:]})
}

func defaultCommands() commands {
	cmds := commands{registeredCommands: make(map[string]func(*state, command) error)}
	for _, e := range allCommands() {
		cmds.register(e.name, e.handler())
	}
	return cmds
}

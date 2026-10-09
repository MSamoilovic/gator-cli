package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/MSamoilovic/gator-cli/internal/config"
	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/store/local"

	_ "github.com/lib/pq"
)

var version = "dev"

type state struct {
	Store  store.Store
	Db     *database.Queries
	DB     *sql.DB
	Cfg    *config.Config
	Schema fs.FS
}

type entry struct {
	name    string
	args    string
	summary string
	group   string

	run     handlerFunc
	runAuth func(context.Context, *state, command, store.User) error

	guest  bool
	hidden bool
	noDB   bool
}

func (e entry) needsLogin() bool { return e.runAuth != nil }

func (e entry) handler() handlerFunc {
	if e.runAuth != nil {
		return middlewareLoggedIn(e.runAuth)
	}
	return e.run
}

func allCommands() []entry {
	return []entry{
		{group: "reading", name: "tui", summary: "Open the interactive reader", runAuth: handlerTUI},
		{group: "reading", name: "browse", args: "[flags]", summary: "Read posts; --no-tui prints them instead", runAuth: handlerBrowse},
		{group: "reading", name: "search", args: "<query>", summary: "Search post titles and bodies", runAuth: handlerSearch},
		{group: "reading", name: "article", args: "<post_url>", summary: "Fetch the full text of a post the feed only teased", runAuth: handlerArticle},
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
		{group: "aggregation", name: "prune", args: "[flags]", summary: "Delete posts older than the retention window", run: handlerPrune},

		{group: "account", name: "register", args: "[flags] <username>", summary: "Create a new user and log in", run: handlerRegister, guest: true},
		{group: "account", name: "login", args: "<username>", summary: "Log in as an existing user", run: handlerLogin, guest: true},
		{group: "account", name: "users", summary: "List all users", run: handlerUsers, guest: true},
		{group: "account", name: "reset", args: "[flags]", summary: "Delete every row in the database", run: handlerReset, hidden: true},

		{group: "setup", name: "init", args: "[flags]", summary: "Create the config and the database schema", run: handlerInit, guest: true, noDB: true},
		{group: "setup", name: "migrate", args: "[flags]", summary: "Apply any database migrations this build carries", run: handlerMigrate},

		{group: "other", name: "help", summary: "Print the list of commands", run: handlerHelp, guest: true, noDB: true},
		{group: "other", name: "version", summary: "Print the version of gator", run: handlerVersion, guest: true, noDB: true},
	}
}

func Run(schema fs.FS, args []string) error {
	ctx := context.Background()
	cmds := defaultCommands()

	if len(args) == 0 {
		return runMenu(ctx, schema, cmds)
	}

	e, ok := lookup(args[0])
	if !ok {
		return fmt.Errorf("command %s doesn't exist (try: gator help)", args[0])
	}

	cmd := command{Name: args[0], Args: args[1:]}
	if e.noDB {
		return cmds.run(ctx, &state{Schema: schema}, cmd)
	}

	s, closeDB, err := open(schema)
	if err != nil {
		return err
	}
	defer closeDB()

	return cmds.run(ctx, s, cmd)
}

func lookup(name string) (entry, bool) {
	for _, e := range allCommands() {
		if e.name == name {
			return e, true
		}
	}
	return entry{}, false
}

func open(schema fs.FS) (*state, func() error, error) {
	cfg, err := config.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("reading config: %w", err)
	}

	db, err := connect(cfg.DBURL)
	if err != nil {
		return nil, nil, err
	}

	q := database.New(db)
	return &state{
		Store:  local.New(q, cfg.CurrentUserName),
		Db:     q,
		DB:     db,
		Cfg:    &cfg,
		Schema: schema,
	}, db.Close, nil
}

func connect(dbURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	return db, nil
}

func defaultCommands() commands {
	cmds := commands{registeredCommands: make(map[string]handlerFunc)}
	for _, e := range allCommands() {
		cmds.register(e.name, e.handler())
	}
	return cmds
}

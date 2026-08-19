// Package cli je sloj komandne linije: registar komandi, njihovi handleri i
// stanje koje dele. Sve sto handler radi je parsiranje argumenata i ispis —
// logika koju deli sa TUI-jem zivi u internal/feeds i internal/catalog.
package cli

import (
	"errors"

	"gator-cli/internal/config"
	"gator-cli/internal/database"
)

// state je ono sto svaki handler dobija: veza sa bazom i procitan config.
type state struct {
	Db  *database.Queries
	Cfg *config.Config
}

// Run izvrsi jednu komandu iz args, gde je args[0] njeno ime. Greska se vraca
// pozivaocu; nijedan handler ne zove os.Exit sam.
func Run(cfg *config.Config, db *database.Queries, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: gator <command> [args...]")
	}

	s := state{Cfg: cfg, Db: db}
	cmds := defaultCommands()
	return cmds.run(&s, command{Name: args[0], Args: args[1:]})
}

func defaultCommands() commands {
	cmds := commands{registeredCommands: make(map[string]func(*state, command) error)}

	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("supervise", handlerSupervise)

	cmds.register("discover", middlewareLoggedIn(handlerDiscover))
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("browse", middlewareLoggedIn(handlerBrowse))
	cmds.register("bookmark", middlewareLoggedIn(handlerBookmark))
	cmds.register("unbookmark", middlewareLoggedIn(handlerUnbookmark))
	cmds.register("bookmarks", middlewareLoggedIn(handlerBookmarks))
	cmds.register("search", middlewareLoggedIn(handlerSearch))
	cmds.register("tui", middlewareLoggedIn(handlerTUI))
	cmds.register("import", middlewareLoggedIn(handlerImport))
	cmds.register("export", middlewareLoggedIn(handlerExport))

	return cmds
}

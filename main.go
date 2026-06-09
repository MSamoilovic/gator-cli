package main

import (
	"database/sql"
	"fmt"
	"gator-ci/internal/config"
	"gator-ci/internal/database"
	"os"

	_ "github.com/lib/pq"
)

type state struct {
    Db *database.Queries
	Cfg *config.Config
}

func main() {

	cfg, err := config.Read()
    if err != nil {
        fmt.Println("Error reading config:", err)
        os.Exit(1)
    }

    db, err := sql.Open("postgres", cfg.DBURL)
    if err != nil {
		fmt.Printf("no connection to database: %v", err)
	}

    s := state{Cfg: &cfg, Db: database.New(db)}

	cmds := commands{registeredCommands: make(map[string]func(*state, command) error)}
	cmds.register("login", handlerLogin)
    cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))

	if len(os.Args) < 2 {
        fmt.Println("Usage: gator <command> [args...]")
        os.Exit(1)
    }

	cmd := command{
        Name: os.Args[1],
        Args: os.Args[2:],
    }

	if err := cmds.run(&s, cmd); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
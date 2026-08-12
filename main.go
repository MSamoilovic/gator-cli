package main

import (
	"database/sql"
	"fmt"
	"os"

	"gator-cli/internal/cli"
	"gator-cli/internal/config"
	"gator-cli/internal/database"

	_ "github.com/lib/pq"
)

func main() {
	// Sav izlaz iz main-a ide kroz run, da bi defer db.Close() stigao da se
	// izvrsi i kad komanda padne — os.Exit ne pokrece odlozene pozive.
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Read()
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	db, err := sql.Open("postgres", cfg.DBURL)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}

	return cli.Run(&cfg, database.New(db), os.Args[1:])
}

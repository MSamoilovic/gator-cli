package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/config"
	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/migrate"

	"github.com/google/uuid"
)

const defaultDBURL = "postgres://postgres@localhost:5432/gator?sslmode=disable"

func handlerInit(s *state, cmd command) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	dbURL := fs.String("db-url", "", "PostgreSQL connection string")
	username := fs.String("user", "", "username to create and log in as")
	force := fs.Bool("force", false, "overwrite an existing config")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: init [--db-url url] [--user name] [--force]")
	}

	ctx := context.Background()

	if existing, err := config.Read(); err == nil && !*force {
		return fmt.Errorf("already initialised (%s holds a config for %s); run gator migrate to apply new migrations, or init --force to start over",
			config.Path(), describeTarget(existing.DBURL))
	}

	if strings.TrimSpace(*dbURL) == "" && !isTerminal(os.Stdin) {
		return errors.New("no --db-url given and no terminal to ask on")
	}
	url := resolveDBURL(*dbURL, os.Stdin, os.Stdout)

	db, err := connect(url)
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Printf("Connected to %s\n", describeTarget(url))

	res, err := migrate.Up(ctx, db, s.Schema)
	if err != nil {
		return err
	}
	reportMigration(res)

	cfg := config.Config{DBURL: url}
	if err := cfg.Write(); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	fmt.Printf("Wrote %s\n", config.Path())

	if *username == "" {
		fmt.Println("\nNext: gator register <username>")
		return nil
	}

	user, err := database.New(db).CreateUser(ctx, database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      *username,
	})
	if err != nil {
		return fmt.Errorf("creating user %q: %w", *username, err)
	}
	if err := cfg.SetUsername(user.Name); err != nil {
		return fmt.Errorf("logging in as %q: %w", user.Name, err)
	}

	fmt.Printf("Registered and logged in as %s\n", user.Name)
	fmt.Println("\nNext: gator discover")
	return nil
}

func handlerMigrate(s *state, cmd command) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	status := fs.Bool("status", false, "list migrations this build carries that the database lacks")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: migrate [--status]")
	}

	ctx := context.Background()

	if *status {
		return reportStatus(ctx, s)
	}

	res, err := migrate.Up(ctx, s.DB, s.Schema)
	if err != nil {
		return err
	}
	reportMigration(res)
	return nil
}

func reportStatus(ctx context.Context, s *state) error {
	version, err := migrate.Version(ctx, s.DB, s.Schema)
	if err != nil {
		return err
	}

	pending, err := migrate.Pending(ctx, s.DB, s.Schema)
	if err != nil {
		return err
	}

	fmt.Printf("Schema version %d\n", version)
	if len(pending) == 0 {
		fmt.Println("Up to date.")
		return nil
	}

	fmt.Printf("%d pending:\n", len(pending))
	for _, name := range pending {
		fmt.Printf("  %s\n", name)
	}
	fmt.Println("\nRun gator migrate to apply them.")
	return nil
}

func reportMigration(res migrate.Result) {
	if n := res.Applied(); n > 0 {
		fmt.Printf("Applied %d migration(s), schema version %d\n", n, res.To)
		return
	}
	fmt.Printf("Schema already up to date at version %d\n", res.To)
}

func resolveDBURL(given string, in io.Reader, out io.Writer) string {
	if given = strings.TrimSpace(given); given != "" {
		return given
	}

	fmt.Fprintf(out, "PostgreSQL connection string [%s]: ", defaultDBURL)

	line, _ := bufio.NewReader(in).ReadString('\n')
	if line = strings.TrimSpace(line); line != "" {
		return line
	}
	return defaultDBURL
}

func describeTarget(dbURL string) string {
	name, err := databaseName(dbURL)
	if err != nil {
		return "the configured database"
	}
	return name
}

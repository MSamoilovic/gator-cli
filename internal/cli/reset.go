package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/MSamoilovic/gator-cli/internal/database"
)

var errResetAborted = errors.New("reset aborted")

func handlerReset(s *state, cmd command) error {
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "report what would be deleted and exit")
	assumeYes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: reset [--dry-run] [--yes]")
	}

	ctx := context.Background()

	dbName, err := localDatabaseName(s.Cfg.DBURL)
	if err != nil {
		return err
	}

	counts, err := s.Db.CountAllRows(ctx)
	if err != nil {
		return fmt.Errorf("counting rows: %w", err)
	}

	printCounts(os.Stdout, dbName, counts)

	if total(counts) == 0 {
		fmt.Println("Nothing to delete.")
		return nil
	}
	if *dryRun {
		fmt.Println("\nDry run — nothing was deleted.")
		return nil
	}

	if !*assumeYes {
		if !isTerminal(os.Stdin) {
			return errors.New("refusing to delete without a terminal to confirm on (pass --yes)")
		}
		if err := confirm(os.Stdin, os.Stdout, dbName); err != nil {
			return err
		}
	}

	if err := s.Db.TruncateAll(ctx); err != nil {
		return fmt.Errorf("truncating: %w", err)
	}

	fmt.Printf("Deleted everything in %s.\n", dbName)
	return nil
}

func printCounts(w io.Writer, dbName string, c database.CountAllRowsRow) {
	fmt.Fprintf(w, "About to delete every row in %s:\n", dbName)
	for _, row := range []struct {
		name string
		n    int64
	}{
		{"users", c.Users},
		{"feeds", c.Feeds},
		{"posts", c.Posts},
		{"feed_follows", c.FeedFollows},
		{"bookmarks", c.Bookmarks},
		{"post_reads", c.PostReads},
	} {
		fmt.Fprintf(w, "  %-13s %d\n", row.name, row.n)
	}
}

func total(c database.CountAllRowsRow) int64 {
	return c.Users + c.Feeds + c.Posts + c.FeedFollows + c.Bookmarks + c.PostReads
}

func confirm(in io.Reader, out io.Writer, dbName string) error {
	fmt.Fprintf(out, "\nType the database name (%s) to confirm: ", dbName)

	line, _ := bufio.NewReader(in).ReadString('\n')
	if strings.TrimSpace(line) != dbName {
		return errResetAborted
	}
	return nil
}

func localDatabaseName(dbURL string) (string, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "", fmt.Errorf("parsing db_url: %w", err)
	}
	if host := u.Hostname(); !isLoopback(host) {
		return "", fmt.Errorf("refusing to delete: db_url points at %s, not a local database", host)
	}
	return databaseName(dbURL)
}

func databaseName(dbURL string) (string, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "", fmt.Errorf("parsing db_url: %w", err)
	}

	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		return "", errors.New("db_url names no database")
	}
	return name, nil
}

func isLoopback(host string) bool {
	if host == "localhost" || host == "" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

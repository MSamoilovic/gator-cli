package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/menu"
)

func runMenu(ctx context.Context, schema fs.FS, cmds commands) error {
	if !interactive() {
		printUsage(os.Stderr)
		return errors.New("no command given")
	}

	s, closeDB, dbErr := open(schema)
	if dbErr != nil {
		s = &state{Schema: schema}
	} else {
		defer closeDB()
	}

	user, loggedIn := currentUser(ctx, s)

	choice, ok, err := menu.Select(menu.Config{
		Title:    "gator",
		Greeting: greeting(user, loggedIn, dbErr == nil),
		Items:    offered(loggedIn, dbErr == nil),
	})
	if err != nil {
		return fmt.Errorf("opening the command menu: %w", err)
	}
	if !ok {
		return nil
	}
	return cmds.run(ctx, s, command{Name: choice.Name, Args: choice.Args})
}

func currentUser(ctx context.Context, s *state) (database.User, bool) {
	if s.Cfg == nil || s.Cfg.CurrentUserName == "" {
		return database.User{}, false
	}
	user, err := s.Db.GetUser(ctx, s.Cfg.CurrentUserName)
	if err != nil {
		return database.User{}, false
	}
	return user, true
}

func greeting(user database.User, loggedIn, configured bool) string {
	switch {
	case !configured:
		return "No working database — see the README for ~/.gatorconfig.json"
	case loggedIn:
		return "Hello, " + user.Name + " — what would you like to do?"
	default:
		return "Not logged in — register or log in to get started"
	}
}

func offered(loggedIn, configured bool) []menu.Item {
	cmds := allCommands()
	items := make([]menu.Item, 0, len(cmds))
	for _, e := range cmds {
		switch {
		case e.hidden:
			continue
		case !configured && !e.noDB:
			continue
		case !loggedIn && !e.guest:
			continue
		}
		items = append(items, menu.Item{
			Name:    e.name,
			Args:    e.args,
			Summary: e.summary,
			Group:   e.group,
		})
	}
	return items
}

func handlerHelp(context.Context, *state, command) error {
	printUsage(os.Stdout)
	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "gator — a CLI RSS feed aggregator")
	fmt.Fprintln(w, "\nusage: gator <command> [args...]")

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	group := ""
	for _, e := range allCommands() {
		if e.hidden {
			continue
		}
		if e.group != group {
			group = e.group
			fmt.Fprintf(tw, "\n%s\n", group)
		}
		fmt.Fprintf(tw, "  %s\t%s\n", strings.TrimSpace(e.name+" "+e.args), e.summary)
	}
	tw.Flush()

	fmt.Fprintln(w, "\nRun gator with no arguments to pick a command interactively.")
}

func interactive() bool { return isTerminal(os.Stdin) && isTerminal(os.Stdout) }

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

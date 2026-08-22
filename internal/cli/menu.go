package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"gator-cli/internal/database"
	"gator-cli/internal/menu"
)

// runMenu je ono sto se desi kad se gator pokrene bez argumenata: na
// interaktivnom terminalu birac komandi, inace ispis upotrebe i greska — da
// skripta koja pozove `gator` bez komande i dalje padne umesto da se zaglavi
// cekajuci taster koji nikad nece stici.
func runMenu(s *state, cmds commands) error {
	if !interactive() {
		printUsage(os.Stderr)
		return errors.New("no command given")
	}

	user, loggedIn := currentUser(s)

	choice, ok, err := menu.Select(menu.Config{
		Title:    "gator",
		Greeting: greeting(user, loggedIn),
		Items:    offered(loggedIn),
	})
	if err != nil {
		return fmt.Errorf("opening the command menu: %w", err)
	}
	if !ok {
		// Izlaz iz menija bez izbora nije greska.
		return nil
	}
	return cmds.run(s, command{Name: choice.Name, Args: choice.Args})
}

// currentUser gleda da li ime iz configa i dalje odgovara nekom korisniku.
// Obrisan korisnik ili prazan config su isto sto i neprijavljen — zato se
// greska ne vraca, nego pretvara u "nije prijavljen".
func currentUser(s *state) (database.User, bool) {
	if s.Cfg.CurrentUserName == "" {
		return database.User{}, false
	}
	user, err := s.Db.GetUser(context.Background(), s.Cfg.CurrentUserName)
	if err != nil {
		return database.User{}, false
	}
	return user, true
}

func greeting(user database.User, loggedIn bool) string {
	if loggedIn {
		return "Hello, " + user.Name + " — what would you like to do?"
	}
	return "Not logged in — register or log in to get started"
}

// offered bira sta se nudi. Izlogovanom korisniku se pokazuju samo komande
// koje mu i mogu proci: sve ostalo bi zavrsilo na "user not logged in".
func offered(loggedIn bool) []menu.Item {
	cmds := allCommands()
	items := make([]menu.Item, 0, len(cmds))
	for _, e := range cmds {
		if e.hidden || (!loggedIn && !e.guest) {
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

func handlerHelp(*state, command) error {
	printUsage(os.Stdout)
	return nil
}

// printUsage ispisuje sve komande grupisane po poslu. Ide i na stderr kad
// izlaz nije terminal, pa je namerno bez boja i okvira.
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

// interactive je tacno kad i ulaz i izlaz idu ka terminalu. Bubble Tea cita
// tastere sa stdin-a, pa `gator < /dev/null` ne sme da otvori birac.
func interactive() bool { return isTerminal(os.Stdin) && isTerminal(os.Stdout) }

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

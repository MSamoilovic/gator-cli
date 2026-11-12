package cli

import (
	"github.com/MSamoilovic/gator-cli/internal/menu"
	"github.com/MSamoilovic/gator-cli/internal/store"
)

type runNext struct {
	cmd    command
	reopen bool
}

func (runNext) Error() string { return "another command was picked" }

func readerChoice(choice menu.Choice, chosen bool, err error) error {
	switch {
	case err != nil:
		return err
	case !chosen:
		return nil
	}
	return runNext{cmd: command{Name: choice.Name, Args: choice.Args}}
}

func readerMenu(s *state, user store.User) []menu.Item {
	items := reachable(s, offered(true, true, user.IsAdmin))

	out := make([]menu.Item, 0, len(items))
	for _, it := range items {
		if e, ok := lookup(it.Name); ok && e.inReader {
			continue
		}
		out = append(out, it)
	}
	return out
}

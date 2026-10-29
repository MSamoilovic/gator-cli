package cli

import (
	"github.com/MSamoilovic/gator-cli/internal/menu"
)

type runNext struct {
	cmd command
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

func readerMenu(s *state) []menu.Item {
	items := reachable(s, offered(true, true))

	out := make([]menu.Item, 0, len(items))
	for _, it := range items {
		if e, ok := lookup(it.Name); ok && e.inReader {
			continue
		}
		out = append(out, it)
	}
	return out
}

package tui

import (
	"errors"

	"github.com/MSamoilovic/gator-cli/internal/store"
)

func (m model) serverName() string {
	if m.server == "" {
		return "the server"
	}
	return m.server
}

func (m model) explain(err error) string {
	switch {
	case errors.Is(err, store.ErrUnauthorized):
		return "Signed out — run: gator login --server " + m.serverName()
	case errors.Is(err, store.ErrPlanLimit):
		return "Plan limit reached — " + err.Error()
	case errors.Is(err, store.ErrUnreachable):
		return "Cannot reach " + m.serverName() + " — check the connection, then press r"
	case errors.Is(err, errors.ErrUnsupported):
		return m.serverName() + " refreshes feeds on its own — press r to reload"
	}
	return "Error: " + err.Error()
}

func (m model) fatalView() string {
	hint := "Press q to quit."
	switch {
	case errors.Is(m.err, store.ErrUnauthorized):
		hint = "Quit with q, sign in again, then start the reader."
	case errors.Is(m.err, store.ErrUnreachable):
		hint = "Press r to try again, or q to quit."
	}
	return "\n  " + m.explain(m.err) + "\n\n  " + hint + "\n"
}

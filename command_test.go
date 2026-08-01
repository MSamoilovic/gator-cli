package main

import (
	"errors"
	"strings"
	"testing"
)

func newCommands() commands {
	return commands{registeredCommands: make(map[string]func(*state, command) error)}
}

func TestRunDispatchesToHandler(t *testing.T) {
	c := newCommands()

	var gotCmd command
	c.register("browse", func(_ *state, cmd command) error {
		gotCmd = cmd
		return nil
	})

	want := command{Name: "browse", Args: []string{"--limit", "5"}}
	if err := c.run(&state{}, want); err != nil {
		t.Fatalf("run: %v", err)
	}

	if gotCmd.Name != want.Name {
		t.Errorf("handler got name %q, want %q", gotCmd.Name, want.Name)
	}
	if strings.Join(gotCmd.Args, " ") != strings.Join(want.Args, " ") {
		t.Errorf("handler got args %v, want %v", gotCmd.Args, want.Args)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	c := newCommands()

	err := c.run(&state{}, command{Name: "nope"})
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error = %q, want it to name the command", err)
	}
}

func TestRunDoesNotDispatchUnknownCommand(t *testing.T) {
	c := newCommands()

	called := false
	c.register("browse", func(*state, command) error {
		called = true
		return nil
	})

	if err := c.run(&state{}, command{Name: "browsee"}); err == nil {
		t.Fatal("expected error for near-miss command name, got nil")
	}
	if called {
		t.Error("handler ran for a command that was not registered")
	}
}

func TestRunWrapsHandlerError(t *testing.T) {
	c := newCommands()

	sentinel := errors.New("boom")
	c.register("addfeed", func(*state, command) error { return sentinel })

	err := c.run(&state{}, command{Name: "addfeed"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error does not unwrap to the handler error: %v", err)
	}
	if !strings.Contains(err.Error(), "addfeed") {
		t.Errorf("error = %q, want it to name the command", err)
	}
}

func TestRegisterOverwrites(t *testing.T) {
	c := newCommands()

	c.register("login", func(*state, command) error { return errors.New("stara") })
	c.register("login", func(*state, command) error { return nil })

	if err := c.run(&state{}, command{Name: "login"}); err != nil {
		t.Errorf("second registration did not win: %v", err)
	}
}

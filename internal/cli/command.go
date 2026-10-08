package cli

import (
	"context"
	"fmt"
)

type command struct {
	Name string
	Args []string
}

type handlerFunc func(context.Context, *state, command) error

type commands struct {
	registeredCommands map[string]handlerFunc
}

func (c *commands) register(name string, f handlerFunc) {
	c.registeredCommands[name] = f
}

func (c *commands) run(ctx context.Context, s *state, cmd command) error {
	f, ok := c.registeredCommands[cmd.Name]
	if !ok {
		return fmt.Errorf("command %s doesn't exist (try: gator help)", cmd.Name)
	}

	if err := f(ctx, s, cmd); err != nil {
		return fmt.Errorf("can't execute %s: %w", cmd.Name, err)
	}
	return nil
}

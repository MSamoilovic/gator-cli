package cli

import "fmt"

type command struct {
	Name string
	Args []string
}

type commands struct {
	registeredCommands map[string]func(*state, command) error
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.registeredCommands[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	f, ok := c.registeredCommands[cmd.Name]

	if !ok {
		return fmt.Errorf("command %s doesn't exist", cmd.Name)
	}

	err := f(s, cmd)

	if err != nil {
		return fmt.Errorf("can't execute %s: %w", cmd.Name, err)
	}

	return nil
}

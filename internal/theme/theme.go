package theme

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const (
	Auto  = "auto"
	Dark  = "dark"
	Light = "light"
	Mono  = "mono"
)

func Names() []string { return []string{Auto, Dark, Light, Mono} }

func Apply(name string) error {
	switch name {
	case "", Auto:
		return nil
	case Dark:
		lipgloss.SetHasDarkBackground(true)
	case Light:
		lipgloss.SetHasDarkBackground(false)
	case Mono:
		lipgloss.SetColorProfile(termenv.Ascii)
	default:
		return fmt.Errorf("unknown theme %q: use one of %v", name, Names())
	}
	return nil
}

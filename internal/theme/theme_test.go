package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func coloured() (string, lipgloss.Style) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	return "x", lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#000001", Dark: "#000002"})
}

func TestEveryNamedThemeApplies(t *testing.T) {
	for _, name := range append(Names(), "") {
		if err := Apply(name); err != nil {
			t.Errorf("Apply(%q) = %v, want no error", name, err)
		}
	}
}

func TestAnUnknownThemeSaysWhatIsAllowed(t *testing.T) {
	err := Apply("darkk")
	if err == nil {
		t.Fatal("Apply accepted a theme that does not exist, so a typo in the config would be invisible")
	}
	for _, name := range Names() {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not offer %q", err, name)
		}
	}
	if !strings.Contains(err.Error(), "darkk") {
		t.Errorf("error %q does not quote the value that was wrong", err)
	}
}

func TestDarkAndLightPickDifferentSidesOfAnAdaptiveColour(t *testing.T) {
	text, style := coloured()

	if err := Apply(Dark); err != nil {
		t.Fatal(err)
	}
	dark := style.Render(text)

	if err := Apply(Light); err != nil {
		t.Fatal(err)
	}
	light := style.Render(text)

	if dark == light {
		t.Errorf("dark and light render an adaptive colour identically (%q); the theme does nothing", dark)
	}
}

func TestMonoRendersWithoutAnyEscapeCodes(t *testing.T) {
	text, style := coloured()

	if got := style.Render(text); !strings.Contains(got, "\x1b") {
		t.Fatalf("the measurement is broken: %q carries no colour to strip", got)
	}

	if err := Apply(Mono); err != nil {
		t.Fatal(err)
	}
	if got := style.Render(text); strings.Contains(got, "\x1b") {
		t.Errorf("mono rendered %q, want no escape codes", got)
	}
}

func TestAutoLeavesTheProfileAlone(t *testing.T) {
	text, style := coloured()
	before := style.Render(text)

	if err := Apply(Auto); err != nil {
		t.Fatal(err)
	}
	if got := style.Render(text); got != before {
		t.Errorf("auto changed the rendering from %q to %q", before, got)
	}
}

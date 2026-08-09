package tui

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"gator-cli/internal/database"

	"github.com/charmbracelet/lipgloss"
)

const detailChromeHeight = 6

var (
	blockTagRe  = regexp.MustCompile(`(?i)</?(br|p|div|li|tr|h[1-6])[^>]*>`)
	anyTagRe    = regexp.MustCompile(`<[^>]*>`)
	blankLineRe = regexp.MustCompile(`\n{3,}`)
)

func stripHTML(s string) string {
	s = blockTagRe.ReplaceAllString(s, "\n")
	s = anyTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = blankLineRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// renderDetailHeader uvek vraca tacno detailChromeHeight-1 redova; racunica
// visine viewport-a zavisi od toga.
func renderDetailHeader(p database.Post, width int, scroll float64) string {
	published := "unknown"
	if p.PublishedAt.Valid {
		published = p.PublishedAt.Time.Format("2006-01-02 15:04")
	}

	meta := fmt.Sprintf("Published: %s · %3.0f%%", published, scroll*100)

	line := lipgloss.NewStyle().MaxWidth(width)
	return fmt.Sprintf("%s\n%s\n%s\n\n",
		line.Render(p.Title),
		line.Render(p.Url),
		line.Render(meta),
	)
}

func renderDetailBody(p database.Post, width int) string {
	if !p.Description.Valid {
		return "(no description)"
	}

	desc := stripHTML(p.Description.String)
	if desc == "" {
		return "(no description)"
	}
	return lipgloss.NewStyle().Width(width).Render(desc)
}

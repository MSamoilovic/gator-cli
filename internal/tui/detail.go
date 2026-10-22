package tui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/text"

	"github.com/charmbracelet/lipgloss"
)

func renderDetailHeader(p database.Post, feed string, width int, scroll float64) string {
	percent := fmt.Sprintf("%3.0f%%", scroll*100)
	source := detailSource(p, feed, width-lipgloss.Width(percent)-1)
	gap := max(width-lipgloss.Width(source)-lipgloss.Width(percent), 1)

	line := lipgloss.NewStyle().MaxWidth(width)
	return fmt.Sprintf("%s\n%s\n\n",
		lipgloss.NewStyle().Width(width).Render(p.Title),
		line.Render(source+strings.Repeat(" ", gap)+percent),
	)
}

func detailSource(p database.Post, feed string, budget int) string {
	parts := []string{feed, hostOf(p.Url), publishedOn(p)}
	if p.FullText != "" {
		parts = append(parts, "full text")
	}

	for len(parts) > 1 {
		if line := joinSource(parts); lipgloss.Width(line) <= budget {
			return line
		}
		parts = parts[:len(parts)-1]
	}
	return joinSource(parts)
}

func joinSource(parts []string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " · ")
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return strings.TrimPrefix(u.Host, "www.")
}

func publishedOn(p database.Post) string {
	if !p.PublishedAt.Valid {
		return "no date"
	}
	return p.PublishedAt.Time.Format("2006-01-02 15:04")
}

func renderDetailBody(p database.Post, width int) string {
	body := p.FullText
	if body == "" && p.Description.Valid {
		body = text.StripHTML(p.Description.String)
	}
	if body == "" {
		return "(no description)"
	}
	return lipgloss.NewStyle().Width(width).Render(body)
}

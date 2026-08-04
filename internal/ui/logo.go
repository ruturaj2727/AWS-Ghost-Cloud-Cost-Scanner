package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Gradient colors for the logo
	logoPurple = lipgloss.Color("#9D4EDD")
	logoPink   = lipgloss.Color("#FF006E")
	logoCyan   = lipgloss.Color("#00F5D4")
	logoYellow = lipgloss.Color("#FFBE0B")
)

// LogoStyle creates a styled text for each line
func LogoStyle(text string, color lipgloss.Color) string {
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Render(text)
}

// GetLogo returns the modern minimal logo
func GetLogo() string {
	lines := []string{
		"        👻 AWS GHOST",
		"   ─────────────────────",
		"   Scan • Detect • Clean",
	}

	colors := []lipgloss.Color{logoPurple, logoPink, logoCyan}

	result := ""
	for i, line := range lines {
		result += LogoStyle(line, colors[i%len(colors)]) + "\n"
	}

	return result
}

// GetCompactLogo returns a smaller version of the logo
func GetCompactLogo() string {
	return lipgloss.NewStyle().
		Foreground(logoPurple).
		Bold(true).
		Render("👻 AWS GHOST")
}

// GetWelcomeMessage returns a styled welcome message
func GetWelcomeMessage() string {
	title := lipgloss.NewStyle().
		Foreground(logoPink).
		Bold(true).
		MarginTop(1).
		MarginBottom(1).
		Render("Scan your AWS account for forgotten, idle, and wasteful resources")

	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true).
		Render("Read-only • Safe • Honest")

	return title + "\n" + subtitle
}

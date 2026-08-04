package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Color constants for progress
	progressCyan   = lipgloss.Color("#00F5D4")
	progressPink   = lipgloss.Color("#FF006E")
	progressPurple = lipgloss.Color("#9D4EDD")

	// Progress bar styles
	progressContainer = lipgloss.NewStyle().
				Padding(0, 1).
				MarginBottom(1)

	scanningStyle = lipgloss.NewStyle().
			Foreground(progressCyan).
			Bold(true)

	doneStyle = lipgloss.NewStyle().
			Foreground(progressPink).
			Bold(true)
)

// SimpleProgress is a non-interactive progress indicator
type SimpleProgress struct {
	resourceTypes []string
	current       int
	startTime     time.Time
}

// NewSimpleProgress creates a new simple progress indicator
func NewSimpleProgress(resourceTypes []string) *SimpleProgress {
	return &SimpleProgress{
		resourceTypes: resourceTypes,
		current:       0,
		startTime:     time.Now(),
	}
}

// Start displays the initial progress
func (sp *SimpleProgress) Start() {
	fmt.Println()
	fmt.Println(scanningStyle.Render(" Starting AWS Ghost Scan..."))
	fmt.Println()
}

// Update updates the progress for a resource type
func (sp *SimpleProgress) Update(resourceType string) {
	sp.current++

	// Calculate percentage
	percent := float64(sp.current) / float64(len(sp.resourceTypes)) * 100

	// Create progress bar
	barWidth := 30
	filled := int(float64(barWidth) * percent / 100)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// Color the bar based on progress
	barColor := progressCyan
	if percent > 50 {
		barColor = progressPurple
	}
	if percent > 80 {
		barColor = progressPink
	}

	coloredBar := lipgloss.NewStyle().Foreground(barColor).Render(bar)

	// Format the progress line
	line := fmt.Sprintf(
		"\r  %s %s %d/%d (%.0f%%)",
		doneStyle.Render("✓"),
		resourceType,
		sp.current,
		len(sp.resourceTypes),
		percent,
	)

	fmt.Print(line + " " + coloredBar)
}

// Complete marks the progress as complete
func (sp *SimpleProgress) Complete() {
	duration := time.Since(sp.startTime)
	fmt.Println()
	fmt.Println()
	fmt.Println(doneStyle.Render(fmt.Sprintf("✨ Scan completed in %s", duration.Round(time.Millisecond))))
	fmt.Println()
}

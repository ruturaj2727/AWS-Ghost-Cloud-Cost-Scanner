package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Table styles
	headerStyle = lipgloss.NewStyle().
			Foreground(logoPurple).
			Bold(true).
			Border(lipgloss.NormalBorder()).
			BorderForeground(logoCyan).
			Padding(0, 1)

	rowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			Padding(0, 1)

	altRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB")).
			Padding(0, 1).
			Background(lipgloss.Color("#1F2937"))

	highCostStyle = lipgloss.NewStyle().
			Foreground(logoPink).
			Bold(true)

	mediumCostStyle = lipgloss.NewStyle().
			Foreground(logoYellow)

	lowCostStyle = lipgloss.NewStyle().
			Foreground(logoCyan)

	summaryStyle = lipgloss.NewStyle().
			Foreground(logoPurple).
			Bold(true).
			MarginTop(1).
			MarginBottom(1)
)

// ResourceIcon returns an emoji icon based on resource type
func ResourceIcon(resourceType string) string {
	icons := map[string]string{
		"ebs":          "💾",
		"eip":          "🌐",
		"nat":          "🚪",
		"loadbalancer": "⚖️",
		"snapshots":    "📸",
		"ecr":          "🐳",
	}

	if icon, ok := icons[resourceType]; ok {
		return icon
	}
	return "📦"
}

// FormatCost formats a cost value with appropriate styling
func FormatCost(cost float64) string {
	costStr := fmt.Sprintf("$%.2f", cost)

	switch {
	case cost >= 100:
		return highCostStyle.Render(costStr)
	case cost >= 10:
		return mediumCostStyle.Render(costStr)
	default:
		return lowCostStyle.Render(costStr)
	}
}

// FormatResourceTable creates a beautiful table of resources
func FormatResourceTable(resources []types.Resource) string {
	if len(resources) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true).
			Render("  No ghost resources found! 🎉")
	}

	// Table headers
	headers := []string{"Type", "ID", "Region", "Cost/Mo", "Idle Days"}

	// Calculate column widths
	typeWidth := 12
	idWidth := 30
	regionWidth := 12
	costWidth := 10
	daysWidth := 10

	// Build header row
	headerRow := fmt.Sprintf(
		"  %-*s %-*s %-*s %-*s %-*s",
		typeWidth, headers[0],
		idWidth, headers[1],
		regionWidth, headers[2],
		costWidth, headers[3],
		daysWidth, headers[4],
	)

	// Build separator
	separator := strings.Repeat("─", typeWidth+idWidth+regionWidth+costWidth+daysWidth+14)

	// Build data rows
	var rows []string
	for i, r := range resources {
		icon := ResourceIcon(r.Type)
		style := rowStyle
		if i%2 == 1 {
			style = altRowStyle
		}

		row := fmt.Sprintf(
			"  %s %-*s %-*s %-*s %s %-*s %d",
			icon,
			typeWidth-2, r.Type,
			idWidth, truncateString(r.ID, idWidth),
			regionWidth, r.Region,
			FormatCost(r.MonthlyCost),
			daysWidth, r.IdleDays,
		)

		rows = append(rows, style.Render(row))
	}

	// Combine all parts
	result := headerStyle.Render(headerRow) + "\n"
	result += lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563")).Render("  "+separator) + "\n"
	result += strings.Join(rows, "\n")

	return result
}

// FormatSummary creates a styled summary section
func FormatSummary(result types.ScanResult) string {
	var lines []string

	// Account info
	accountLine := fmt.Sprintf("  Account: %s | Region: %s", result.AccountID, result.Region)
	lines = append(lines, lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Render(accountLine))

	// Resources found
	resourceLine := fmt.Sprintf("  Resources Found: %d", len(result.Resources))
	lines = append(lines, lipgloss.NewStyle().
		Foreground(logoCyan).
		Render(resourceLine))

	// Total cost
	costLine := fmt.Sprintf("  Total Monthly Cost: %s", FormatCost(result.TotalCost))
	lines = append(lines, costLine)

	// Scan duration
	durationLine := fmt.Sprintf("  Scan Duration: %s", result.ScanDuration.Round(time.Millisecond))
	lines = append(lines, lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Render(durationLine))

	// Scanned types
	if len(result.ScannedTypes) > 0 {
		typesLine := fmt.Sprintf("  Scanned Types: %s", strings.Join(result.ScannedTypes, ", "))
		lines = append(lines, lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Render(typesLine))
	}

	return summaryStyle.Render(strings.Join(lines, "\n"))
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// FormatCard creates a card-style display for a single resource
func FormatCard(r types.Resource) string {
	icon := ResourceIcon(r.Type)

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(logoCyan).
		Padding(1, 2).
		MarginBottom(1).
		Width(60)

	title := lipgloss.NewStyle().
		Foreground(logoPurple).
		Bold(true).
		Render(fmt.Sprintf("%s %s", icon, r.Type))

	id := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Render(fmt.Sprintf("ID: %s", r.ID))

	region := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Render(fmt.Sprintf("Region: %s", r.Region))

	cost := lipgloss.NewStyle().
		Render(fmt.Sprintf("Cost: %s", FormatCost(r.MonthlyCost)))

	idle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Render(fmt.Sprintf("Idle: %d days", r.IdleDays))

	content := fmt.Sprintf(
		"%s\n%s\n%s\n%s\n%s",
		title,
		id,
		region,
		cost,
		idle,
	)

	return cardStyle.Render(content)
}

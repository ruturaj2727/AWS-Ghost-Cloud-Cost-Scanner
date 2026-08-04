package ui

import (
	"fmt"
	"strings"

	"github.com/NotHarshhaa/aws-ghost/pkg/types"
)

// Formatter handles output formatting with modern UI
type Formatter struct {
	useColor bool
	quiet    bool
}

// NewFormatter creates a new formatter
func NewFormatter(useColor, quiet bool) *Formatter {
	return &Formatter{
		useColor: useColor,
		quiet:    quiet,
	}
}

// Format formats the scan result with beautiful UI
func (f *Formatter) Format(result types.ScanResult) (string, error) {
	if f.quiet {
		return f.formatQuiet(result), nil
	}
	
	var builder strings.Builder
	
	// Add logo
	builder.WriteString(GetLogo())
	builder.WriteString("\n")
	
	// Add welcome message
	builder.WriteString(GetWelcomeMessage())
	builder.WriteString("\n\n")
	
	// Add resource table
	builder.WriteString(FormatResourceTable(result.Resources))
	builder.WriteString("\n\n")
	
	// Add summary
	builder.WriteString(FormatSummary(result))
	
	return builder.String(), nil
}

// formatQuiet returns a minimal one-line output
func (f *Formatter) formatQuiet(result types.ScanResult) string {
	return fmt.Sprintf(
		"Found %d ghost resources costing $%.2f/month in %s",
		len(result.Resources),
		result.TotalCost,
		result.Region,
	)
}

// FormatJSON formats the result as JSON (for compatibility)
func (f *Formatter) FormatJSON(result types.ScanResult) (string, error) {
	// This would use the existing JSON formatter
	// For now, return a placeholder
	return fmt.Sprintf(`{"account_id":"%s","region":"%s","total_cost":%.2f,"resources":%d}`,
		result.AccountID,
		result.Region,
		result.TotalCost,
		len(result.Resources),
	), nil
}

// FormatMarkdown formats the result as Markdown
func (f *Formatter) FormatMarkdown(result types.ScanResult) (string, error) {
	var builder strings.Builder
	
	builder.WriteString("# AWS Ghost Scan Report\n\n")
	builder.WriteString(fmt.Sprintf("**Account:** %s\n\n", result.AccountID))
	builder.WriteString(fmt.Sprintf("**Region:** %s\n\n", result.Region))
	builder.WriteString(fmt.Sprintf("**Total Monthly Cost:** $%.2f\n\n", result.TotalCost))
	builder.WriteString(fmt.Sprintf("**Resources Found:** %d\n\n", len(result.Resources)))
	
	builder.WriteString("## Ghost Resources\n\n")
	builder.WriteString("| Type | ID | Region | Cost/Mo | Idle Days |\n")
	builder.WriteString("|------|----|----|---------|----------|\n")
	
	for _, r := range result.Resources {
		builder.WriteString(fmt.Sprintf("| %s | %s | %s | $%.2f | %d |\n",
			r.Type, r.ID, r.Region, r.MonthlyCost, r.IdleDays))
	}
	
	return builder.String(), nil
}

// FormatCSV formats the result as CSV
func (f *Formatter) FormatCSV(result types.ScanResult) (string, error) {
	var builder strings.Builder
	
	builder.WriteString("Type,ID,Region,MonthlyCost,IdleDays\n")
	
	for _, r := range result.Resources {
		builder.WriteString(fmt.Sprintf("%s,%s,%s,%.2f,%d\n",
			r.Type, r.ID, r.Region, r.MonthlyCost, r.IdleDays))
	}
	
	return builder.String(), nil
}

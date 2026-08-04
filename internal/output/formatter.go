package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/NotHarshhaa/aws-ghost/pkg/types"
)

// Formatter defines the output formatter interface
type Formatter interface {
	Format(result types.ScanResult) (string, error)
}

// TextFormatter outputs human-readable text
type TextFormatter struct {
	NoColor bool
	Quiet   bool
}

// NewTextFormatter creates a new text formatter
func NewTextFormatter(noColor, quiet bool) *TextFormatter {
	return &TextFormatter{
		NoColor: noColor,
		Quiet:   quiet,
	}
}

func (f *TextFormatter) Format(result types.ScanResult) (string, error) {
	if f.Quiet {
		return fmt.Sprintf("Ghosts Found: %d resources — estimated waste: $%.2f/month\n", len(result.Resources), result.TotalCost), nil
	}

	var sb strings.Builder

	// Header
	sb.WriteString(" AWS Ghost Scanner ────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Account   %s\n", result.AccountID))
	sb.WriteString(fmt.Sprintf("  Region    %s\n", result.Region))
	sb.WriteString(fmt.Sprintf("  Scanned   %d resource types in %s\n", len(result.ScannedTypes), result.ScanDuration))
	sb.WriteString("\n")

	// Summary
	sb.WriteString(fmt.Sprintf(" Ghosts Found: %d resources — estimated waste: $%.2f/month\n\n", len(result.Resources), result.TotalCost))

	// Group by resource type
	grouped := f.groupByType(result.Resources)
	for resourceType, resources := range grouped {
		typeCost := 0.0
		for _, r := range resources {
			typeCost += r.MonthlyCost
		}

		sb.WriteString(fmt.Sprintf(" %s ($%.2f/mo)\n", f.getTypeTitle(resourceType), typeCost))
		for _, r := range resources {
			sb.WriteString(fmt.Sprintf("  %-20s %-20s idle %3d days     $%.2f/mo\n", f.formatID(r), f.formatMetadata(r), r.IdleDays, r.MonthlyCost))
		}
		sb.WriteString("\n")
	}

	// Annual savings
	sb.WriteString(fmt.Sprintf(" Estimated annual savings if cleaned: $%.2f\n", result.TotalCost*12))
	sb.WriteString("\n Run `aws-ghost report --output markdown > ghost-report.md` to export\n")
	sb.WriteString("──────────────────────────────────────────────────────────────────\n")

	return sb.String(), nil
}

func (f *TextFormatter) groupByType(resources []types.Resource) map[string][]types.Resource {
	grouped := make(map[string][]types.Resource)
	for _, r := range resources {
		grouped[r.Type] = append(grouped[r.Type], r)
	}
	return grouped
}

func (f *TextFormatter) getTypeTitle(resourceType string) string {
	titles := map[string]string{
		"ebs":          "EBS Volumes (unattached)",
		"eip":          "Elastic IPs (unattached)",
		"loadbalancer": "Load Balancers (zero traffic, 7d)",
		"nat":          "NAT Gateways (zero traffic, 7d)",
		"snapshots":    "RDS/EC2 Snapshots (older than 90 days)",
		"ecr":          "ECR Images (unused)",
	}
	if title, ok := titles[resourceType]; ok {
		return title
	}
	return strings.ToUpper(resourceType)
}

func (f *TextFormatter) formatID(r types.Resource) string {
	if r.Name != "" {
		return fmt.Sprintf("%s %s", r.ID, r.Name)
	}
	return r.ID
}

func (f *TextFormatter) formatMetadata(r types.Resource) string {
	if size, ok := r.Metadata["size"]; ok {
		return size
	}
	if size, ok := r.Metadata["size_gb"]; ok {
		return size + " GB"
	}
	return ""
}

// JSONFormatter outputs JSON
type JSONFormatter struct{}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

func (f *JSONFormatter) Format(result types.ScanResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// MarkdownFormatter outputs Markdown
type MarkdownFormatter struct{}

// NewMarkdownFormatter creates a new Markdown formatter
func NewMarkdownFormatter() *MarkdownFormatter {
	return &MarkdownFormatter{}
}

func (f *MarkdownFormatter) Format(result types.ScanResult) (string, error) {
	var sb strings.Builder

	sb.WriteString("# AWS Ghost Report\n\n")
	sb.WriteString(fmt.Sprintf("**Account:** %s  \n", result.AccountID))
	sb.WriteString(fmt.Sprintf("**Region:** %s  \n", result.Region))
	sb.WriteString(fmt.Sprintf("**Scan Date:** %s  \n\n", result.ScanDuration))
	sb.WriteString(fmt.Sprintf("**Ghosts Found:** %d resources  \n", len(result.Resources)))
	sb.WriteString(fmt.Sprintf("**Estimated Waste:** $%.2f/month\n\n", result.TotalCost))

	sb.WriteString("## Ghost Resources\n\n")

	grouped := f.groupByType(result.Resources)
	for resourceType, resources := range grouped {
		typeCost := 0.0
		for _, r := range resources {
			typeCost += r.MonthlyCost
		}

		sb.WriteString(fmt.Sprintf("### %s ($%.2f/month)\n\n", f.getTypeTitle(resourceType), typeCost))
		sb.WriteString("| ID | Name | State | Idle Days | Monthly Cost |\n")
		sb.WriteString("|---|---|---|---|---|\n")

		for _, r := range resources {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %d | $%.2f |\n", r.ID, r.Name, r.State, r.IdleDays, r.MonthlyCost))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("## Summary\n\nEstimated annual savings if cleaned: **$%.2f**\n", result.TotalCost*12))

	return sb.String(), nil
}

func (f *MarkdownFormatter) groupByType(resources []types.Resource) map[string][]types.Resource {
	grouped := make(map[string][]types.Resource)
	for _, r := range resources {
		grouped[r.Type] = append(grouped[r.Type], r)
	}
	return grouped
}

func (f *MarkdownFormatter) getTypeTitle(resourceType string) string {
	titles := map[string]string{
		"ebs":          "EBS Volumes (unattached)",
		"eip":          "Elastic IPs (unattached)",
		"loadbalancer": "Load Balancers (zero traffic, 7d)",
		"nat":          "NAT Gateways (zero traffic, 7d)",
		"snapshots":    "RDS/EC2 Snapshots (older than 90 days)",
		"ecr":          "ECR Images (unused)",
	}
	if title, ok := titles[resourceType]; ok {
		return title
	}
	return strings.ToUpper(resourceType)
}

// CSVFormatter outputs CSV
type CSVFormatter struct{}

// NewCSVFormatter creates a new CSV formatter
func NewCSVFormatter() *CSVFormatter {
	return &CSVFormatter{}
}

func (f *CSVFormatter) Format(result types.ScanResult) (string, error) {
	var sb strings.Builder
	writer := csv.NewWriter(&sb)

	// Write header
	header := []string{"ID", "Type", "Region", "Name", "State", "Idle Days", "Monthly Cost"}
	if err := writer.Write(header); err != nil {
		return "", err
	}

	// Write rows
	for _, r := range result.Resources {
		row := []string{
			r.ID,
			r.Type,
			r.Region,
			r.Name,
			r.State,
			fmt.Sprintf("%d", r.IdleDays),
			fmt.Sprintf("%.2f", r.MonthlyCost),
		}
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return sb.String(), nil
}

// GetFormatter returns the appropriate formatter based on format string
func GetFormatter(format string, noColor, quiet bool) (Formatter, error) {
	switch strings.ToLower(format) {
	case "json":
		return NewJSONFormatter(), nil
	case "markdown", "md":
		return NewMarkdownFormatter(), nil
	case "csv":
		return NewCSVFormatter(), nil
	case "text", "default", "":
		return NewTextFormatter(noColor, quiet), nil
	default:
		return nil, fmt.Errorf("unknown output format: %s", format)
	}
}

// WriteOutput writes the formatted output to a file or stdout
func WriteOutput(output string, filename string) error {
	if filename == "" || filename == "-" {
		fmt.Print(output)
		return nil
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.WriteString(file, output)
	return err
}

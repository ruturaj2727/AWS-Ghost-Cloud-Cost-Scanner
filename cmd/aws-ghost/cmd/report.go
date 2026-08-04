package cmd

import (
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/output"
	"github.com/NotHarshhaa/aws-ghost/internal/scanner"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/spf13/cobra"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate a report of ghost resources",
	Long:  `Generate a detailed report of ghost resources in various formats.`,
	RunE:  runReport,
}

func init() {
	reportCmd.Flags().StringVarP(&region, "region", "r", "us-east-1", "AWS region to scan")
	reportCmd.Flags().BoolVar(&allRegions, "all-regions", false, "Scan all enabled regions")
	reportCmd.Flags().StringVarP(&profile, "profile", "p", "", "AWS named profile")
	reportCmd.Flags().StringVar(&only, "only", "", "Comma-separated resource types to scan")
	reportCmd.Flags().StringVar(&skip, "skip", "", "Comma-separated resource types to skip")
	reportCmd.Flags().StringVarP(&outputFmt, "output", "o", "markdown", "Output format: text, json, markdown, csv")
	reportCmd.Flags().Float64Var(&minCost, "min-cost", 0, "Only show resources above this monthly cost ($)")
	reportCmd.Flags().IntVar(&idleDays, "idle-days", 7, "Days of inactivity to consider a resource idle")
	reportCmd.Flags().StringVarP(&outputFile, "file", "f", "", "Output file (default: stdout)")
}

var outputFile string

func runReport(cmd *cobra.Command, args []string) error {
	startTime := time.Now()

	// Parse include/exclude lists
	onlyList := parseList(only)
	skipList := parseList(skip)

	// Create AWS client
	client, err := aws.NewClient(profile, region)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	// Create scanner registry
	registry := scanner.NewRegistry(client)

	// Get filtered scanners
	scanners := registry.GetFiltered(onlyList, skipList)

	// Scan
	var allResources []types.Resource
	var scannedTypes []string

	for name, scn := range scanners {
		config := types.ScanConfig{
			Region:   region,
			Profile:  profile,
			IdleDays: idleDays,
			MinCost:  minCost,
		}

		resources, err := scn.Scan(config)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to scan %s: %v\n", name, err)
			continue
		}

		// Filter by min cost
		var filtered []types.Resource
		for _, r := range resources {
			if r.MonthlyCost >= minCost {
				filtered = append(filtered, r)
			}
		}

		allResources = append(allResources, filtered...)
		scannedTypes = append(scannedTypes, name)
	}

	// Calculate total cost
	totalCost := 0.0
	for _, r := range allResources {
		totalCost += r.MonthlyCost
	}

	// Create result
	result := types.ScanResult{
		AccountID:    client.AccountID,
		Region:       region,
		Resources:    allResources,
		TotalCost:    totalCost,
		ScanDuration: time.Since(startTime),
		ScannedTypes: scannedTypes,
	}

	// Format output
	formatter, err := output.GetFormatter(outputFmt, false, false)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	outputStr, err := formatter.Format(result)
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Write output
	if err := output.WriteOutput(outputStr, outputFile); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	if outputFile != "" {
		fmt.Printf("Report written to %s\n", outputFile)
	}

	return nil
}

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/scanner"
	"github.com/NotHarshhaa/aws-ghost/internal/ui"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/spf13/cobra"
)

var (
	daysBack         int
	trendsOutputFile string
	compareFile      string
	regions          []string
	profiles         []string
)

var trendsCmd = &cobra.Command{
	Use:   "trends",
	Short: "Analyze cost trends and historical waste patterns",
	Long: `Analyze cost trends and historical waste patterns over time.

This command helps you track how your AWS waste changes over time, identify
trends, and measure the impact of your cleanup efforts.

Features:
• Historical comparison with previous scans
• Trend analysis and visualization
• Cost savings tracking
• Resource type evolution tracking
• Multi-account trend comparison

Examples:
  # Show trends for the last 30 days
  aws-ghost trends --days-back 30

  # Compare with a specific previous scan
  aws-ghost trends --compare previous-scan.json

  # Export trends to JSON
  aws-ghost trends --output trends.json

  # Show trends with markdown output
  aws-ghost trends --output markdown`,
	RunE: runTrends,
}

func init() {
	rootCmd.AddCommand(trendsCmd)

	trendsCmd.Flags().IntVar(&daysBack, "days-back", 30, "Number of days to analyze for trends")
	trendsCmd.Flags().StringVar(&trendsOutputFile, "output", "", "Output file for trends report (supports: text, json, markdown)")
	trendsCmd.Flags().StringVar(&compareFile, "compare", "", "Previous scan file to compare against")
	trendsCmd.Flags().StringSliceVar(&regions, "region", []string{}, "AWS regions to analyze")
	trendsCmd.Flags().StringSliceVar(&profiles, "profile", []string{}, "AWS profiles to analyze")
	trendsCmd.Flags().BoolVar(&allRegions, "all-regions", false, "Analyze all enabled regions")
}

func runTrends(cmd *cobra.Command, args []string) error {
	fmt.Println(ui.GetCompactLogo())
	fmt.Println()
	fmt.Println("📈 AWS Ghost Cost Trends Analysis")
	fmt.Println()

	// Determine scan scope
	scanRegions := determineScanRegions(regions, allRegions)
	scanProfiles := determineScanProfiles(profiles)

	// Get historical data
	historicalData, err := getHistoricalData(daysBack)
	if err != nil {
		fmt.Printf("⚠️  Could not load historical data: %v\n", err)
		fmt.Println("💡 This is normal for first-time usage")
	}

	// Run current scan
	currentResults, err := runCurrentScan(scanRegions, scanProfiles)
	if err != nil {
		return fmt.Errorf("failed to run current scan: %w", err)
	}

	// Generate trends analysis
	trends := generateTrendsAnalysis(historicalData, currentResults)

	// Output results
	if outputFile != "" {
		err = exportTrends(trends, outputFile)
		if err != nil {
			return fmt.Errorf("failed to export trends: %w", err)
		}
		fmt.Printf("📄 Trends exported to: %s\n", outputFile)
	} else {
		displayTrends(trends)
	}

	// Save current scan for future comparison
	err = saveTrendsScan(currentResults)
	if err != nil {
		fmt.Printf("⚠️  Could not save current scan: %v\n", err)
	}

	return nil
}

type TrendData struct {
	CurrentDate     time.Time                `json:"current_date"`
	PeriodDays      int                      `json:"period_days"`
	CurrentWaste    float64                  `json:"current_waste"`
	PreviousWaste   float64                  `json:"previous_waste"`
	WasteChange     float64                  `json:"waste_change"`
	WasteChangePct  float64                  `json:"waste_change_percent"`
	ResourceTypes   map[string]ResourceTrend `json:"resource_types"`
	RegionBreakdown map[string]float64       `json:"region_breakdown"`
	TopWasters      []ResourceTrend          `json:"top_wasters"`
	SavedAmount     float64                  `json:"saved_amount"`
	SavedAmountPct  float64                  `json:"saved_amount_percent"`
}

type ResourceTrend struct {
	Type           string  `json:"type"`
	CurrentCost    float64 `json:"current_cost"`
	PreviousCost   float64 `json:"previous_cost"`
	CostChange     float64 `json:"cost_change"`
	CostChangePct  float64 `json:"cost_change_percent"`
	CurrentCount   int     `json:"current_count"`
	PreviousCount  int     `json:"previous_count"`
	CountChange    int     `json:"count_change"`
	CountChangePct float64 `json:"count_change_percent"`
}

func getHistoricalData(days int) ([]types.ScanResult, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	ghostDir := filepath.Join(homeDir, ".aws-ghost")
	scansDir := filepath.Join(ghostDir, "scans")

	var historicalData []types.ScanResult

	// Read scan files from the last N days
	cutoffDate := time.Now().AddDate(0, 0, -days)

	err = filepath.Walk(scansDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".json") {
			return nil
		}

		if info.ModTime().Before(cutoffDate) {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		var scanResult types.ScanResult
		decoder := json.NewDecoder(file)
		err = decoder.Decode(&scanResult)
		if err != nil {
			return nil
		}

		historicalData = append(historicalData, scanResult)
		return nil
	})

	return historicalData, err
}

func runCurrentScan(regions, profiles []string) ([]types.ScanResult, error) {
	var results []types.ScanResult

	for _, profile := range profiles {
		for _, region := range regions {
			client, err := aws.NewClient(profile, region)
			if err != nil {
				continue
			}

			registry := scanner.NewRegistry(client)
			// Parse include/exclude lists
			onlyList := parseList(only)
			skipList := parseList(skip)
			scanners := registry.GetFiltered(onlyList, skipList)

			var allResources []types.Resource
			for _, scanner := range scanners {
				scanConfig := types.ScanConfig{
					Region:   region,
					Profile:  profile,
					MinCost:  minCost,
					IdleDays: 7,
				}
				resources, err := scanner.Scan(scanConfig)
				if err != nil {
					continue
				}
				allResources = append(allResources, resources...)
			}

			result := types.ScanResult{
				AccountID:    client.AccountID,
				Region:       region,
				Resources:    allResources,
				TotalCost:    calculateTrendsTotalCost(allResources),
				ScanDuration: 0, // Would be calculated during actual scan
				ScannedTypes: getScannedTypes(scanners),
			}

			results = append(results, result)
		}
	}

	return results, nil
}

func generateTrendsAnalysis(historical []types.ScanResult, current []types.ScanResult) TrendData {
	trends := TrendData{
		CurrentDate:     time.Now(),
		PeriodDays:      daysBack,
		ResourceTypes:   make(map[string]ResourceTrend),
		RegionBreakdown: make(map[string]float64),
	}

	// Calculate current waste
	var currentWaste float64
	for _, result := range current {
		currentWaste += result.TotalCost
		trends.RegionBreakdown[result.Region] = result.TotalCost
	}
	trends.CurrentWaste = currentWaste

	// Calculate previous waste and trends
	if len(historical) > 0 {
		var previousWaste float64
		resourceTypeHistory := make(map[string][]float64)
		resourceCountHistory := make(map[string][]int)

		for _, result := range historical {
			previousWaste += result.TotalCost

			// Track resource types over time
			typeCounts := make(map[string]int)
			typeCosts := make(map[string]float64)

			for _, resource := range result.Resources {
				typeCounts[resource.Type]++
				typeCosts[resource.Type] += resource.MonthlyCost
			}

			for resType, cost := range typeCosts {
				resourceTypeHistory[resType] = append(resourceTypeHistory[resType], cost)
			}

			for resType, count := range typeCounts {
				resourceCountHistory[resType] = append(resourceCountHistory[resType], count)
			}
		}

		trends.PreviousWaste = previousWaste
		trends.WasteChange = currentWaste - previousWaste
		if previousWaste > 0 {
			trends.WasteChangePct = (trends.WasteChange / previousWaste) * 100
		}

		// Calculate resource type trends
		currentTypeCosts := make(map[string]float64)
		currentTypeCounts := make(map[string]int)

		for _, result := range current {
			for _, resource := range result.Resources {
				currentTypeCosts[resource.Type] += resource.MonthlyCost
				currentTypeCounts[resource.Type]++
			}
		}

		for resType, currentCost := range currentTypeCosts {
			trend := ResourceTrend{
				Type:         resType,
				CurrentCost:  currentCost,
				CurrentCount: currentTypeCounts[resType],
			}

			if historical, exists := resourceTypeHistory[resType]; exists && len(historical) > 0 {
				avgPreviousCost := average(historical)
				trend.PreviousCost = avgPreviousCost
				trend.CostChange = currentCost - avgPreviousCost
				if avgPreviousCost > 0 {
					trend.CostChangePct = (trend.CostChange / avgPreviousCost) * 100
				}
			}

			if counts, exists := resourceCountHistory[resType]; exists && len(counts) > 0 {
				avgPreviousCount := averageInt(counts)
				trend.PreviousCount = int(avgPreviousCount)
				trend.CountChange = currentTypeCounts[resType] - int(avgPreviousCount)
				if avgPreviousCount > 0 {
					trend.CountChangePct = (float64(trend.CountChange) / float64(avgPreviousCount)) * 100
				}
			}

			trends.ResourceTypes[resType] = trend
		}

		// Calculate saved amount (reduction in waste)
		if trends.WasteChange < 0 {
			trends.SavedAmount = -trends.WasteChange
			if trends.PreviousWaste > 0 {
				trends.SavedAmountPct = (trends.SavedAmount / trends.PreviousWaste) * 100
			}
		}
	}

	// Generate top wasters
	var topWasters []ResourceTrend
	for _, trend := range trends.ResourceTypes {
		topWasters = append(topWasters, trend)
	}

	// Sort by cost change (descending)
	for i := 0; i < len(topWasters)-1; i++ {
		for j := i + 1; j < len(topWasters); j++ {
			if topWasters[j].CostChange > topWasters[i].CostChange {
				topWasters[i], topWasters[j] = topWasters[j], topWasters[i]
			}
		}
	}

	// Keep top 10
	if len(topWasters) > 10 {
		topWasters = topWasters[:10]
	}

	trends.TopWasters = topWasters

	return trends
}

func displayTrends(trends TrendData) {
	fmt.Printf("📊 Analysis Period: Last %d days\n", trends.PeriodDays)
	fmt.Printf("📅 Analysis Date: %s\n\n", trends.CurrentDate.Format("2006-01-02"))

	// Overall waste trends
	fmt.Println("💰 Overall Waste Trends:")
	fmt.Printf("   Current waste:     $%.2f/month\n", trends.CurrentWaste)
	if trends.PreviousWaste > 0 {
		fmt.Printf("   Previous waste:    $%.2f/month\n", trends.PreviousWaste)
		fmt.Printf("   Change:            $%.2f (%.1f%%)\n", trends.WasteChange, trends.WasteChangePct)
		if trends.SavedAmount > 0 {
			fmt.Printf("   💸 Savings:          $%.2f (%.1f%% reduction)\n", trends.SavedAmount, trends.SavedAmountPct)
		}
	}
	fmt.Println()

	// Resource type trends
	fmt.Println("📋 Resource Type Trends:")
	for resType, trend := range trends.ResourceTypes {
		changeSymbol := "→"
		if trend.CostChange > 0 {
			changeSymbol = "↑"
		} else if trend.CostChange < 0 {
			changeSymbol = "↓"
		}

		fmt.Printf("   %s: $%.2f %s $%.2f (%.1f%%) | %d %s %d (%.1f%%)\n",
			resType,
			trend.CurrentCost,
			changeSymbol,
			trend.CostChange,
			trend.CostChangePct,
			trend.CurrentCount,
			changeSymbol,
			trend.CountChange,
			trend.CountChangePct)
	}
	fmt.Println()

	// Regional breakdown
	fmt.Println("🌍 Regional Breakdown:")
	for region, cost := range trends.RegionBreakdown {
		fmt.Printf("   %s: $%.2f/month\n", region, cost)
	}
	fmt.Println()

	// Top wasters
	if len(trends.TopWasters) > 0 {
		fmt.Println("🚨 Top Waste Contributors:")
		for i, trend := range trends.TopWasters {
			fmt.Printf("   %d. %s: $%.2f (%+.1f%%)\n", i+1, trend.Type, trend.CurrentCost, trend.CostChangePct)
		}
	}
}

func exportTrends(trends TrendData, filename string) error {
	var data []byte
	var err error

	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".json":
		data, err = json.MarshalIndent(trends, "", "  ")
	case ".md":
		data = []byte(generateMarkdownTrends(trends))
	default:
		data = []byte(generateTextTrends(trends))
	}

	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func generateMarkdownTrends(trends TrendData) string {
	var sb strings.Builder

	sb.WriteString("# AWS Ghost Cost Trends Report\n\n")
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n", trends.CurrentDate.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Period:** Last %d days\n\n", trends.PeriodDays))

	sb.WriteString("## Overall Waste Trends\n\n")
	sb.WriteString("| Metric | Amount | Change |\n")
	sb.WriteString("|--------|--------|--------|\n")
	sb.WriteString(fmt.Sprintf("| Current Waste | $%.2f/month | - |\n", trends.CurrentWaste))

	if trends.PreviousWaste > 0 {
		sb.WriteString(fmt.Sprintf("| Previous Waste | $%.2f/month | $%.2f (%.1f%%) |\n",
			trends.PreviousWaste, trends.WasteChange, trends.WasteChangePct))
		if trends.SavedAmount > 0 {
			sb.WriteString(fmt.Sprintf("| 💸 Savings | - | $%.2f (%.1f%% reduction) |\n",
				trends.SavedAmount, trends.SavedAmountPct))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Resource Type Trends\n\n")
	sb.WriteString("| Resource Type | Current Cost | Change | Current Count | Count Change |\n")
	sb.WriteString("|---------------|--------------|---------|---------------|--------------|\n")

	for resType, trend := range trends.ResourceTypes {
		changeSymbol := "→"
		if trend.CostChange > 0 {
			changeSymbol = "↑"
		} else if trend.CostChange < 0 {
			changeSymbol = "↓"
		}

		sb.WriteString(fmt.Sprintf("| %s | $%.2f | %s $%.2f (%.1f%%) | %d | %s %d (%.1f%%) |\n",
			resType, trend.CurrentCost, changeSymbol, trend.CostChange, trend.CostChangePct,
			trend.CurrentCount, changeSymbol, trend.CountChange, trend.CountChangePct))
	}
	sb.WriteString("\n")

	sb.WriteString("## Regional Breakdown\n\n")
	for region, cost := range trends.RegionBreakdown {
		sb.WriteString(fmt.Sprintf("- **%s:** $%.2f/month\n", region, cost))
	}
	sb.WriteString("\n")

	if len(trends.TopWasters) > 0 {
		sb.WriteString("## Top Waste Contributors\n\n")
		for i, trend := range trends.TopWasters {
			sb.WriteString(fmt.Sprintf("%d. **%s:** $%.2f (%+.1f%%)\n", i+1, trend.Type, trend.CurrentCost, trend.CostChangePct))
		}
	}

	return sb.String()
}

func generateTextTrends(trends TrendData) string {
	var sb strings.Builder

	sb.WriteString("AWS Ghost Cost Trends Report\n")
	sb.WriteString(strings.Repeat("=", 50))
	sb.WriteString(fmt.Sprintf("\nGenerated: %s\n", trends.CurrentDate.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Period: Last %d days\n\n", trends.PeriodDays))

	sb.WriteString("Overall Waste Trends:\n")
	sb.WriteString(fmt.Sprintf("  Current waste:     $%.2f/month\n", trends.CurrentWaste))

	if trends.PreviousWaste > 0 {
		sb.WriteString(fmt.Sprintf("  Previous waste:    $%.2f/month\n", trends.PreviousWaste))
		sb.WriteString(fmt.Sprintf("  Change:            $%.2f (%.1f%%)\n", trends.WasteChange, trends.WasteChangePct))
		if trends.SavedAmount > 0 {
			sb.WriteString(fmt.Sprintf("  💸 Savings:          $%.2f (%.1f%% reduction)\n", trends.SavedAmount, trends.SavedAmountPct))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("Resource Type Trends:\n")
	for resType, trend := range trends.ResourceTypes {
		changeSymbol := "→"
		if trend.CostChange > 0 {
			changeSymbol = "↑"
		} else if trend.CostChange < 0 {
			changeSymbol = "↓"
		}

		sb.WriteString(fmt.Sprintf("  %s: $%.2f %s $%.2f (%.1f%%) | %d %s %d (%.1f%%)\n",
			resType, trend.CurrentCost, changeSymbol, trend.CostChange, trend.CostChangePct,
			trend.CurrentCount, changeSymbol, trend.CountChange, trend.CountChangePct))
	}
	sb.WriteString("\n")

	return sb.String()
}

func saveTrendsScan(results []types.ScanResult) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	ghostDir := filepath.Join(homeDir, ".aws-ghost")
	scansDir := filepath.Join(ghostDir, "scans")

	err = os.MkdirAll(scansDir, 0755)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("scan-%s.json", time.Now().Format("20060102-150405"))
	filepath := filepath.Join(scansDir, filename)

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0644)
}

func calculateTrendsTotalCost(resources []types.Resource) float64 {
	var total float64
	for _, resource := range resources {
		total += resource.MonthlyCost
	}
	return total
}

func getScannedTypes(scanners map[string]types.Scanner) []string {
	var types []string
	for scannerType := range scanners {
		types = append(types, scannerType)
	}
	return types
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func averageInt(values []int) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	return float64(sum) / float64(len(values))
}

package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/scanner"
	"github.com/NotHarshhaa/aws-ghost/internal/security"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/spf13/cobra"
)

var (
	anomalyDays      int
	anomalyThreshold float64
	anomalyOutput    string
	historyFile      string
)

var anomalyCmd = &cobra.Command{
	Use:   "anomaly",
	Short: "Detect cost anomalies in AWS resource waste",
	Long:  `Analyze historical waste data to detect unusual spending patterns and anomalies.`,
	RunE:  runAnomaly,
}

func init() {
	anomalyCmd.Flags().IntVar(&anomalyDays, "days", 30, "Number of days to analyze for anomalies")
	anomalyCmd.Flags().Float64Var(&anomalyThreshold, "threshold", 2.0, "Standard deviation threshold for anomaly detection")
	anomalyCmd.Flags().StringVarP(&anomalyOutput, "output", "o", "text", "Output format: text, json")
	anomalyCmd.Flags().StringVar(&historyFile, "history-file", "", "Path to historical data file (default: auto-detected)")
}

func runAnomaly(cmd *cobra.Command, args []string) error {
	// Get historical data
	history, err := loadHistoryData()
	if err != nil {
		return fmt.Errorf("failed to load history data: %w", err)
	}

	if len(history) < 3 {
		fmt.Println("⚠️  Insufficient historical data for anomaly detection.")
		fmt.Println("   Run scans regularly to build history.")
		fmt.Println("   Minimum 3 data points required.")
		return nil
	}

	// Calculate statistics
	mean, stdDev := calculateStatistics(history)

	// Run current scan
	client, err := aws.NewClient(profile, region)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	secConfig := types.GetSecurityConfig(types.SecurityLevelMedium)
	validator := security.NewValidator(secConfig, client.Config)

	credInfo, err := validator.ValidateCredentials(cmd.Context())
	if err != nil {
		return fmt.Errorf("security validation failed: %w", err)
	}

	registry := scanner.NewRegistry(client)
	scanners := registry.GetFiltered(nil, nil)

	var totalCost float64
	resourceCosts := make(map[string]float64)

	for name, scn := range scanners {
		config := types.ScanConfig{
			Region:   region,
			Profile:  profile,
			IdleDays: idleDays,
			MinCost:  0,
		}

		resources, err := scn.Scan(config)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to scan %s: %v\n", name, err)
			continue
		}

		for _, r := range resources {
			totalCost += r.MonthlyCost
			resourceCosts[name] += r.MonthlyCost
		}
	}

	// Detect anomalies
	upperThreshold := mean + (anomalyThreshold * stdDev)
	lowerThreshold := math.Max(0, mean-(anomalyThreshold*stdDev))

	isAnomaly := totalCost > upperThreshold || totalCost < lowerThreshold
	percentChange := ((totalCost - mean) / mean) * 100

	fmt.Printf("\n🔍 Cost Anomaly Detection\n")
	fmt.Printf("========================\n")
	fmt.Printf("Account: %s\n", credInfo.AccountID)
	fmt.Printf("Region: %s\n", region)
	fmt.Printf("Analysis Period: Last %d days\n", anomalyDays)
	fmt.Printf("\n")
	fmt.Printf("Statistics:\n")
	fmt.Printf("  Historical Mean: $%.2f\n", mean)
	fmt.Printf("  Std Deviation: $%.2f\n", stdDev)
	fmt.Printf("  Upper Threshold: $%.2f\n", upperThreshold)
	fmt.Printf("  Lower Threshold: $%.2f\n", lowerThreshold)
	fmt.Printf("\n")
	fmt.Printf("Current Scan:\n")
	fmt.Printf("  Total Waste: $%.2f\n", totalCost)
	fmt.Printf("  Change from Mean: %.1f%%\n", percentChange)
	fmt.Printf("\n")

	if isAnomaly {
		if totalCost > upperThreshold {
			fmt.Printf("🚨 ANOMALY DETECTED: Waste is %.1f%% above normal\n", percentChange)
			fmt.Printf("   Current: $%.2f vs Expected: < $%.2f\n", totalCost, upperThreshold)
		} else {
			fmt.Printf("✅ POSITIVE ANOMALY: Waste is %.1f%% below normal\n", -percentChange)
			fmt.Printf("   Current: $%.2f vs Expected: > $%.2f\n", totalCost, lowerThreshold)
		}
	} else {
		fmt.Printf("✅ No anomalies detected. Waste is within normal range.\n")
	}

	// Resource-level anomaly detection
	fmt.Printf("\nResource-Level Analysis:\n")
	fmt.Printf("-------------------------\n")

	for resource, cost := range resourceCosts {
		if cost > 10 { // Only show significant costs
			fmt.Printf("  %s: $%.2f\n", resource, cost)
		}
	}

	// Save current scan to history
	if err := saveAnomalyScan(totalCost, resourceCosts); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save scan to history: %v\n", err)
	}

	fmt.Printf("\n")

	return nil
}

func loadHistoryData() ([]float64, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	historyPath := filepath.Join(configDir, "aws-ghost", "history.json")

	if historyFile != "" {
		historyPath = historyFile
	}

	data, err := os.ReadFile(historyPath)
	if err != nil {
		return []float64{}, nil
	}

	var history struct {
		Scans []struct {
			TotalCost float64 `json:"total_cost"`
			Timestamp string  `json:"timestamp"`
		} `json:"scans"`
	}

	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}

	var costs []float64
	cutoff := time.Now().AddDate(0, 0, -anomalyDays)

	for _, scan := range history.Scans {
		scanTime, err := time.Parse(time.RFC3339, scan.Timestamp)
		if err != nil {
			continue
		}

		if scanTime.After(cutoff) {
			costs = append(costs, scan.TotalCost)
		}
	}

	return costs, nil
}

func calculateStatistics(data []float64) (mean, stdDev float64) {
	if len(data) == 0 {
		return 0, 0
	}

	// Calculate mean
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	mean = sum / float64(len(data))

	// Calculate standard deviation
	sumSquares := 0.0
	for _, v := range data {
		diff := v - mean
		sumSquares += diff * diff
	}

	variance := sumSquares / float64(len(data))
	stdDev = math.Sqrt(variance)

	return mean, stdDev
}

func saveAnomalyScan(totalCost float64, resourceCosts map[string]float64) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	ghostDir := filepath.Join(configDir, "aws-ghost")
	if err := os.MkdirAll(ghostDir, 0755); err != nil {
		return err
	}

	historyPath := filepath.Join(ghostDir, "history.json")

	var history struct {
		Scans []struct {
			TotalCost     float64            `json:"total_cost"`
			ResourceCosts map[string]float64 `json:"resource_costs"`
			Timestamp     string             `json:"timestamp"`
		} `json:"scans"`
	}

	data, err := os.ReadFile(historyPath)
	if err == nil {
		json.Unmarshal(data, &history)
	}

	// Add new scan
	newScan := struct {
		TotalCost     float64            `json:"total_cost"`
		ResourceCosts map[string]float64 `json:"resource_costs"`
		Timestamp     string             `json:"timestamp"`
	}{
		TotalCost:     totalCost,
		ResourceCosts: resourceCosts,
		Timestamp:     time.Now().Format(time.RFC3339),
	}

	history.Scans = append(history.Scans, newScan)

	// Keep only last 90 days
	cutoff := time.Now().AddDate(0, 0, -90)
	var filtered []struct {
		TotalCost     float64            `json:"total_cost"`
		ResourceCosts map[string]float64 `json:"resource_costs"`
		Timestamp     string             `json:"timestamp"`
	}

	for _, scan := range history.Scans {
		scanTime, _ := time.Parse(time.RFC3339, scan.Timestamp)
		if scanTime.After(cutoff) {
			filtered = append(filtered, scan)
		}
	}

	history.Scans = filtered

	data, err = json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(historyPath, data, 0644)
}

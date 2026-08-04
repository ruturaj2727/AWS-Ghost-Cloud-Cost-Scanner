package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	budgetAmount float64
	budgetPeriod string
	budgetFile   string
	setBudget    bool
	checkBudget  bool
	notifyBudget bool
	webhookURL   string
)

var budgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Manage and check budget alerts for AWS waste",
	Long:  `Set budget thresholds and get alerts when ghost resource waste exceeds your budget.`,
	RunE:  runBudget,
}

func init() {
	budgetCmd.Flags().Float64Var(&budgetAmount, "amount", 100, "Budget amount in USD")
	budgetCmd.Flags().StringVar(&budgetPeriod, "period", "monthly", "Budget period: monthly, weekly, daily")
	budgetCmd.Flags().StringVar(&budgetFile, "file", "", "Path to budget configuration file")
	budgetCmd.Flags().BoolVar(&setBudget, "set", false, "Set a new budget")
	budgetCmd.Flags().BoolVar(&checkBudget, "check", false, "Check current waste against budget")
	budgetCmd.Flags().BoolVar(&notifyBudget, "notify", false, "Send notification if budget exceeded")
	budgetCmd.Flags().StringVar(&webhookURL, "webhook", "", "Webhook URL for notifications (Slack, Teams, Discord)")
}

func runBudget(cmd *cobra.Command, args []string) error {
	if setBudget {
		return setBudgetConfig()
	}
	if checkBudget {
		return checkBudgetStatus()
	}

	// Default: show budget status
	return showBudgetStatus()
}

func setBudgetConfig() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	ghostDir := filepath.Join(configDir, "aws-ghost")
	if err := os.MkdirAll(ghostDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(ghostDir, "budget.json")

	budgetConfig := map[string]interface{}{
		"amount": budgetAmount,
		"period": budgetPeriod,
		"set_at": time.Now().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(budgetConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal budget config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write budget config: %w", err)
	}

	fmt.Printf("✅ Budget set: $%.2f/%s\n", budgetAmount, budgetPeriod)
	fmt.Printf("📁 Config saved to: %s\n", configPath)
	return nil
}

func checkBudgetStatus() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "aws-ghost", "budget.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("no budget configured. Use --set to configure a budget: %w", err)
	}

	var budgetConfig struct {
		Amount float64 `json:"amount"`
		Period string  `json:"period"`
	}

	if err := json.Unmarshal(data, &budgetConfig); err != nil {
		return fmt.Errorf("failed to parse budget config: %w", err)
	}

	// Run a scan to get current waste
	client, err := aws.NewClient(profile, region)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	secConfig := types.GetSecurityConfig(types.SecurityLevelMedium)
	validator := security.NewValidator(secConfig, client.Config)

	credInfo, err := validator.ValidateCredentials(context.Background())
	if err != nil {
		return fmt.Errorf("security validation failed: %w", err)
	}

	registry := scanner.NewRegistry(client)
	scanners := registry.GetFiltered(nil, nil)

	var totalCost float64
	for name, scn := range scanners {
		config := types.ScanConfig{
			Region:   region,
			Profile:  profile,
			IdleDays: idleDays,
			MinCost:  0,
		}

		resources, err := scn.Scan(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to scan %s: %v\n", name, err)
			continue
		}

		for _, r := range resources {
			totalCost += r.MonthlyCost
		}
	}

	usagePercent := (totalCost / budgetConfig.Amount) * 100

	fmt.Printf("\n💰 Budget Status\n")
	fmt.Printf("===============\n")
	fmt.Printf("Account: %s\n", credInfo.AccountID)
	fmt.Printf("Budget: $%.2f/%s\n", budgetConfig.Amount, budgetConfig.Period)
	fmt.Printf("Current Waste: $%.2f\n", totalCost)
	fmt.Printf("Usage: %.1f%%\n", usagePercent)

	if totalCost > budgetConfig.Amount {
		overBudget := totalCost - budgetConfig.Amount
		fmt.Printf("⚠️  OVER BUDGET by $%.2f\n", overBudget)

		if notifyBudget && webhookURL != "" {
			sendBudgetAlert(webhookURL, fmt.Sprintf("%.2f", budgetConfig.Amount), fmt.Sprintf("%.2f", totalCost), fmt.Sprintf("%.2f", overBudget), credInfo.AccountID)
		}
	} else {
		remaining := budgetConfig.Amount - totalCost
		fmt.Printf("✅ Under budget by $%.2f\n", remaining)
	}

	fmt.Printf("\n")
	return nil
}

func showBudgetStatus() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "aws-ghost", "budget.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Println("No budget configured. Use --set to configure a budget.")
		fmt.Println("\nExample:")
		fmt.Println("  aws-ghost budget --set --amount 100 --period monthly")
		return nil
	}

	var budgetConfig struct {
		Amount float64 `json:"amount"`
		Period string  `json:"period"`
		SetAt  string  `json:"set_at"`
	}

	if err := json.Unmarshal(data, &budgetConfig); err != nil {
		return fmt.Errorf("failed to parse budget config: %w", err)
	}

	fmt.Printf("\n💰 Current Budget Configuration\n")
	fmt.Printf("================================\n")
	fmt.Printf("Amount: $%.2f\n", budgetConfig.Amount)
	fmt.Printf("Period: %s\n", budgetConfig.Period)
	fmt.Printf("Set at: %s\n", budgetConfig.SetAt)
	fmt.Printf("\nCommands:\n")
	fmt.Printf("  aws-ghost budget --check         Check current waste against budget\n")
	fmt.Printf("  aws-ghost budget --set           Set a new budget\n")
	fmt.Printf("  aws-ghost budget --notify        Send notification if over budget\n")
	fmt.Printf("\n")

	return nil
}

func sendBudgetAlert(webhook, budget, current, over, account string) {
	alert := map[string]interface{}{
		"text": "⚠️ AWS Ghost Budget Alert",
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]string{
					"type":  "plain_text",
					"text":  "⚠️ AWS Ghost Budget Alert",
					"emoji": "true",
				},
			},
			{
				"type": "section",
				"fields": []map[string]interface{}{
					{
						"type": "mrkdwn",
						"text": fmt.Sprintf("*Account:*\n%s", account),
					},
					{
						"type": "mrkdwn",
						"text": fmt.Sprintf("*Budget:*\n$%s", budget),
					},
					{
						"type": "mrkdwn",
						"text": fmt.Sprintf("*Current Waste:*\n$%s", current),
					},
					{
						"type": "mrkdwn",
						"text": fmt.Sprintf("*Over Budget:*\n$%s", over),
					},
				},
			},
			{
				"type": "context",
				"elements": []map[string]interface{}{
					{
						"type": "mrkdwn",
						"text": fmt.Sprintf("Sent by aws-ghost at %s", time.Now().Format(time.RFC3339)),
					},
				},
			},
		},
	}

	data, err := json.Marshal(alert)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to marshal alert payload: %v\n", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhook, "application/json", bytes.NewBuffer(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to send budget alert: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("📢 Budget alert sent successfully to webhook\n")
	} else {
		fmt.Fprintf(os.Stderr, "❌ Webhook returned status %d\n", resp.StatusCode)
	}
}

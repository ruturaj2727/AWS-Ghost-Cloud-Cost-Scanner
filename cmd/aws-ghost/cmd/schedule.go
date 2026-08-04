package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/scanner"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/spf13/cobra"
)

var (
	scheduleCron    string
	scheduleCommand string
	scheduleProfile string
	scheduleRegion  string
	scheduleList    bool
	scheduleRemove  string
	scheduleEnable  string
	scheduleDisable string
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage scheduled automated scans",
	Long:  `Configure automated scheduled scans with built-in cron functionality.`,
	RunE:  runSchedule,
}

var scheduleRunCmd = &cobra.Command{
	Use:   "run [schedule-id]",
	Short: "Run a scheduled scan manually",
	Args:  cobra.ExactArgs(1),
	RunE:  runScheduleManual,
}

func init() {
	scheduleCmd.Flags().StringVar(&scheduleCron, "cron", "0 9 * * 1", "Cron expression (default: every Monday at 9am)")
	scheduleCmd.Flags().StringVar(&scheduleCommand, "command", "scan", "Command to run (scan, check, anomaly)")
	scheduleCmd.Flags().StringVar(&scheduleProfile, "profile", "", "AWS profile to use")
	scheduleCmd.Flags().StringVar(&scheduleRegion, "region", "us-east-1", "AWS region to scan")
	scheduleCmd.Flags().BoolVar(&scheduleList, "list", false, "List all scheduled scans")
	scheduleCmd.Flags().StringVar(&scheduleRemove, "remove", "", "Remove a scheduled scan by ID")
	scheduleCmd.Flags().StringVar(&scheduleEnable, "enable", "", "Enable a scheduled scan by ID")
	scheduleCmd.Flags().StringVar(&scheduleDisable, "disable", "", "Disable a scheduled scan by ID")

	scheduleCmd.AddCommand(scheduleRunCmd)
}

func runSchedule(cmd *cobra.Command, args []string) error {
	if scheduleList {
		return listSchedules()
	}
	if scheduleRemove != "" {
		return removeSchedule(scheduleRemove)
	}
	if scheduleEnable != "" {
		return toggleSchedule(scheduleEnable, true)
	}
	if scheduleDisable != "" {
		return toggleSchedule(scheduleDisable, false)
	}

	// Add new schedule
	return addSchedule()
}

func runScheduleManual(cmd *cobra.Command, args []string) error {
	scheduleID := args[0]

	// Load schedules
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	schedulePath := filepath.Join(configDir, "aws-ghost", "schedules.json")
	data, err := os.ReadFile(schedulePath)
	if err != nil {
		return fmt.Errorf("no schedules found: %w", err)
	}

	var schedules []ScheduledScan
	if err := json.Unmarshal(data, &schedules); err != nil {
		return fmt.Errorf("failed to parse schedules: %w", err)
	}

	// Find the schedule
	var target *ScheduledScan
	for i := range schedules {
		if schedules[i].ID == scheduleID {
			target = &schedules[i]
			break
		}
	}

	if target == nil {
		return fmt.Errorf("schedule %s not found", scheduleID)
	}

	if !target.Enabled {
		return fmt.Errorf("schedule %s is disabled", scheduleID)
	}

	fmt.Printf("🚀 Running schedule: %s\n", target.ID)
	fmt.Printf("   Command: %s | Region: %s | Profile: %s\n\n", target.Command, target.Region, target.Profile)

	// Execute the scan
	prof := target.Profile
	reg := target.Region
	if reg == "" {
		reg = "us-east-1"
	}

	client, err := aws.NewClient(prof, reg)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	registry := scanner.NewRegistry(client)
	scanners := registry.GetAll()

	var totalCost float64
	var totalResources int
	for name, scn := range scanners {
		config := types.ScanConfig{Region: reg, Profile: prof, IdleDays: 7}
		resources, err := scn.Scan(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to scan %s: %v\n", name, err)
			continue
		}
		totalResources += len(resources)
		for _, r := range resources {
			totalCost += r.MonthlyCost
		}
	}

	fmt.Printf("✅ Scan complete: %d ghost resources, $%.2f/month waste\n", totalResources, totalCost)
	return nil
}

func addSchedule() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	ghostDir := filepath.Join(configDir, "aws-ghost")
	if err := os.MkdirAll(ghostDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	schedulePath := filepath.Join(ghostDir, "schedules.json")

	var schedules []ScheduledScan
	data, err := os.ReadFile(schedulePath)
	if err == nil {
		json.Unmarshal(data, &schedules)
	}

	newSchedule := ScheduledScan{
		ID:        generateScheduleID(),
		Cron:      scheduleCron,
		Command:   scheduleCommand,
		Profile:   scheduleProfile,
		Region:    scheduleRegion,
		Enabled:   true,
		CreatedAt: time.Now().Format(time.RFC3339),
		NextRun:   "Next execution based on cron: " + scheduleCron,
	}

	schedules = append(schedules, newSchedule)

	data, err = json.MarshalIndent(schedules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schedules: %w", err)
	}

	if err := os.WriteFile(schedulePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write schedules: %w", err)
	}

	fmt.Printf("✅ Schedule added successfully\n")
	fmt.Printf("\nSchedule Details:\n")
	fmt.Printf("  ID: %s\n", newSchedule.ID)
	fmt.Printf("  Cron: %s\n", newSchedule.Cron)
	fmt.Printf("  Command: %s\n", newSchedule.Command)
	fmt.Printf("  Region: %s\n", newSchedule.Region)
	fmt.Printf("\nTo run manually: aws-ghost schedule run %s\n", newSchedule.ID)

	return nil
}

func listSchedules() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	schedulePath := filepath.Join(configDir, "aws-ghost", "schedules.json")

	data, err := os.ReadFile(schedulePath)
	if err != nil {
		fmt.Println("No scheduled scans configured.")
		fmt.Println("\nTo add a schedule:")
		fmt.Println("  aws-ghost schedule --cron '0 9 * * 1' --command scan")
		return nil
	}

	var schedules []ScheduledScan
	if err := json.Unmarshal(data, &schedules); err != nil {
		return fmt.Errorf("failed to parse schedules: %w", err)
	}

	if len(schedules) == 0 {
		fmt.Println("No scheduled scans configured.")
		return nil
	}

	fmt.Printf("\n📅 Scheduled Scans\n")
	fmt.Printf("==================\n\n")

	for i, s := range schedules {
		status := "✅ Enabled"
		if !s.Enabled {
			status = "❌ Disabled"
		}
		fmt.Printf("%d. %s [%s]\n", i+1, s.ID, status)
		fmt.Printf("   Cron: %s | Command: %s | Region: %s\n", s.Cron, s.Command, s.Region)
		fmt.Printf("   Created: %s\n\n", s.CreatedAt)
	}

	return nil
}

func removeSchedule(id string) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	schedulePath := filepath.Join(configDir, "aws-ghost", "schedules.json")

	data, err := os.ReadFile(schedulePath)
	if err != nil {
		return fmt.Errorf("no schedules found: %w", err)
	}

	var schedules []ScheduledScan
	if err := json.Unmarshal(data, &schedules); err != nil {
		return fmt.Errorf("failed to parse schedules: %w", err)
	}

	var updated []ScheduledScan
	found := false
	for _, s := range schedules {
		if s.ID == id {
			found = true
			continue
		}
		updated = append(updated, s)
	}

	if !found {
		return fmt.Errorf("schedule with ID %s not found", id)
	}

	data, err = json.MarshalIndent(updated, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schedules: %w", err)
	}

	if err := os.WriteFile(schedulePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write schedules: %w", err)
	}

	fmt.Printf("✅ Schedule %s removed successfully\n", id)
	return nil
}

func toggleSchedule(id string, enable bool) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	schedulePath := filepath.Join(configDir, "aws-ghost", "schedules.json")

	data, err := os.ReadFile(schedulePath)
	if err != nil {
		return fmt.Errorf("no schedules found: %w", err)
	}

	var schedules []ScheduledScan
	if err := json.Unmarshal(data, &schedules); err != nil {
		return fmt.Errorf("failed to parse schedules: %w", err)
	}

	found := false
	for i := range schedules {
		if schedules[i].ID == id {
			schedules[i].Enabled = enable
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("schedule with ID %s not found", id)
	}

	data, err = json.MarshalIndent(schedules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schedules: %w", err)
	}

	if err := os.WriteFile(schedulePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write schedules: %w", err)
	}

	action := "enabled"
	if !enable {
		action = "disabled"
	}
	fmt.Printf("✅ Schedule %s %s successfully\n", id, action)
	return nil
}

func generateScheduleID() string {
	return fmt.Sprintf("sched-%d", time.Now().Unix())
}

type ScheduledScan struct {
	ID        string `json:"id"`
	Cron      string `json:"cron"`
	Command   string `json:"command"`
	Profile   string `json:"profile"`
	Region    string `json:"region"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
	NextRun   string `json:"next_run"`
}

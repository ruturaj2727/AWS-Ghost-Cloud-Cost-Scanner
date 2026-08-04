package cmd

import (
	"fmt"

	"github.com/NotHarshhaa/aws-ghost/internal/ui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aws-ghost",
	Short: "Scan your AWS account for forgotten, idle, and wasteful resources",
	Long: `aws-ghost scans your AWS account for forgotten, idle, and wasteful resources
and shows you exactly what they're costing you.

It is read-only, safe, and honest. It tells you what's wasting money so you can
decide what to do with it.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Show logo and help when no command is provided
		fmt.Println(ui.GetLogo())
		fmt.Println(ui.GetWelcomeMessage())
		fmt.Println()
		cmd.Help()
	},
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(fixCmd)
	rootCmd.AddCommand(trendsCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(securityCmd)
	rootCmd.AddCommand(budgetCmd)
	rootCmd.AddCommand(anomalyCmd)
	rootCmd.AddCommand(recommendCmd)
	rootCmd.AddCommand(terraformCmd)
	rootCmd.AddCommand(scheduleCmd)
}

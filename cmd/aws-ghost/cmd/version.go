package cmd

import (
	"fmt"

	"github.com/NotHarshhaa/aws-ghost/internal/ui"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  `Print the version number of aws-ghost.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(ui.GetCompactLogo())
		fmt.Println()
		fmt.Println("aws-ghost v2.2.0")
		fmt.Println()
		fmt.Println("Scan your AWS account for forgotten, idle, and wasteful resources")
	},
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Generates a diff of the current AWS CLI config versus the available SSO accounts.",
	Long: clihelpers.LongHelpText(`
	Generates a diff of the current AWS CLI config versus the available SSO accounts.

	This command helps users identify discrepancies between their existing AWS CLI
	configuration and the SSO accounts that have been set up, allowing for easier
	synchronization and management of credentials.
	`),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("hello")
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)

	// diffCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

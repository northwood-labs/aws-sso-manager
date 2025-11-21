package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Simplifies updating accounts and roles in the AWS config.",
	Long: clihelpers.LongHelpText(`
	Simplifies updating accounts and roles in the AWS config.

	This command provides a streamlined way for users to update the AWS accounts
	and roles in their AWS SSO Vault configuration, ensuring that their setup
	remains current and accurate.
	`),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("update called")
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	// updateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

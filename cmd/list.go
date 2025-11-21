package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists all configured AWS SSO accounts and their associated roles.",
	Long: clihelpers.LongHelpText(`
	Lists all configured AWS SSO accounts and their associated roles.

	This command provides an overview of the SSO accounts that have been set up
	in AWS SSO, allowing users to see which accounts and roles are available for
	authentication.
	`),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("hello")
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	// listCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

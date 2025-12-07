package cmd

import (
	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Commands related to AWS SSO Vault configuration.",
	Long: clihelpers.LongHelpText(`
	Commands related to AWS SSO Vault configuration.

	This command group includes subcommands for initializing, listing, and
	managing AWS SSO Vault configurations, helping users set up and maintain
	their SSO settings effectively.
	`),
}

func init() {
	rootCmd.AddCommand(configCmd)

	// configCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

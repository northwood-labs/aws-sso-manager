package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

// configRmCmd represents the rm command
var configRmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Removes a value from the AWS SSO Vault configuration.",
	Long: clihelpers.LongHelpText(`
	Removes a value from the AWS SSO Vault configuration.

	This command allows users to delete specific configuration values
	from the AWS SSO Vault setup, helping to maintain and clean up stored settings
	and parameters.
	`),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("rm called")
	},
}

func init() {
	configCmd.AddCommand(configRmCmd)

	// rmCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

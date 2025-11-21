package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

// configSetCmd represents the set command
var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Writes a value to the AWS SSO Vault configuration.",
	Long: clihelpers.LongHelpText(`
	Writes a value to the AWS SSO Vault configuration.

	This command allows users to set or update specific configuration values
	in the AWS SSO Vault setup, enabling easy management of stored settings
	and parameters.
	`),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("set called")
	},
}

func init() {
	configCmd.AddCommand(configSetCmd)

	// setCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

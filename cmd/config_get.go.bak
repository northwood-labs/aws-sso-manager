package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

// configGetCmd represents the get command
var configGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Reads a value from the AWS SSO Vault configuration.",
	Long: clihelpers.LongHelpText(`
	Reads a value from the AWS SSO Vault configuration.

	This command allows users to retrieve specific configuration values
	from the AWS SSO Vault setup, facilitating easy access to stored settings
	and parameters.
	`),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("hello")
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)

	// getCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

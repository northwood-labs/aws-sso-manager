package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initializes AWS SSO Vault configuration.",
	Long: clihelpers.LongHelpText(`
	Initializes AWS SSO Vault configuration by setting up the SSO config for
	AWS CLI and/or AWS Vault.
	`),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("hello")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	// initCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

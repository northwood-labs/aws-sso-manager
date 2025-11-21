package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticates with AWS SSO and retrieves temporary credentials.",
	Long: clihelpers.LongHelpText(`
	Authenticates with AWS SSO and retrieves temporary credentials. This command
	can be used to manually trigger the authentication process and ensure that
	valid credentials are available for AWS CLI and AWS Vault.
	`),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("hello")
	},
}

func init() {
	rootCmd.AddCommand(authCmd)

	// authCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

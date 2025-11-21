package cmd

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

var rootCmd = &cobra.Command{
	Use:   "aws-sso-vault",
	Short: "Sets up your AWS SSO credentials into your AWS CLI config.",
	Long: clihelpers.LongHelpText(`
	AWS SSO Vault sets up your AWS Identity Center (née SSO) credentials into
	your AWS CLI config.

	This allows you to use the AWS CLI with your SSO accounts seamlessly. It
	also enables the use of AWS Vault with SSO.
	`),
}

func init() {
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.aws-sso-vault.yaml)")

	rootCmd.Flags().BoolP("json", "j", false, "output in JSON format")
}

// Execute configures the Cobra CLI app framework and executes the root command.
func Execute() {
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		os.Exit(1)
	}
}

// Root exposes the root command for tools like doc generators.
// https://cobra.dev/docs/how-to-guides/clis-for-llms/
func Root() *cobra.Command {
	return rootCmd
}

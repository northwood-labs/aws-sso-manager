package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	clihelpers "github.com/northwood-labs/cli-helpers"
	"github.com/northwood-labs/debug"
)

var (
	fConfigFile string
	fVerbose    int
	fJSON       bool

	asvConfig = viper.New()

	awsConfigFilePath string
	userHomeDir       string
	logger            *log.Logger
	err               error

	pp  = debug.GetSpew()
	ctx = context.Background()

	rootCmd = &cobra.Command{
		Use:   "aws-sso-vault",
		Short: "Sets up your AWS SSO credentials into your AWS CLI config.",
		Long: clihelpers.LongHelpText(`
		AWS SSO Vault sets up your AWS Identity Center (née SSO) credentials into
		your AWS CLI config.

		This allows you to use the AWS CLI with your SSO accounts seamlessly. It
		also enables the use of AWS Vault with SSO.
		`),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			logger = log.NewWithOptions(os.Stderr, log.Options{
				ReportCaller:    true,
				ReportTimestamp: true,
				TimeFormat:      time.Kitchen,
			})

			switch fVerbose {
			case 0:
				logger.SetLevel(log.WarnLevel)
			case 1:
				logger.SetLevel(log.InfoLevel)
			default:
				logger.SetLevel(log.DebugLevel)
			}

			return initializeConfig(cmd)
		},
	}
)

func init() {
	userHomeDir, err = os.UserHomeDir()
	if err != nil {
		cobra.CheckErr(err)
	}

	awsConfigFilePath = config.DefaultSharedConfigFilename() + "2"

	rootCmd.PersistentFlags().StringVarP(
		&fConfigFile, "config", "c", path.Join(userHomeDir, ".aws-sso-vault.toml"),
		"configuration file",
	)
	rootCmd.PersistentFlags().CountVarP(
		&fVerbose, "verbose", "v",
		"increase verbosity level (can be used multiple times)",
	)
	rootCmd.PersistentFlags().BoolVarP(&fJSON, "json", "j", false, "output in JSON format")
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

func initializeConfig(cmd *cobra.Command) error {
	asvConfig.SetEnvPrefix("ASV") // AWS SSO Vault
	asvConfig.SetEnvKeyReplacer(strings.NewReplacer(".", "*", "-", "*"))
	asvConfig.AutomaticEnv()

	if fConfigFile != path.Join(userHomeDir, ".aws-sso-vault.toml") {
		logger.Infof("Config file (.aws-sso-vault.toml) is set via flag to %s.", fConfigFile)

		// Use config file from the flag.
		_, err := os.Stat(fConfigFile)
		if os.IsNotExist(err) {
			logger.Infof("Config file (.aws-sso-vault.toml) does not exist at %s.", fConfigFile)

			return fmt.Errorf("config file (.aws-sso-vault.toml) does not exist at %s", fConfigFile)
		}

		asvConfig.SetConfigFile(fConfigFile)
	} else {
		_, err := os.Stat(fConfigFile)
		if os.IsNotExist(err) {
			logger.Info("Config file does not exist. Create a new one.")

			// Find a valid config file path.
			userHomeDir, err := os.UserHomeDir()
			cobra.CheckErr(err)

			asvConfig.AddConfigPath(userHomeDir)
			asvConfig.SetConfigName(".aws-sso-vault")
			asvConfig.SetConfigType("toml")

			err = asvConfig.WriteConfigAs(path.Join(userHomeDir, ".aws-sso-vault.toml"))
			if err != nil {
				logger.Info("!!!!!! This should not happen !!!!!!")

				var configFileAlreadyExistsError viper.ConfigFileAlreadyExistsError
				if !errors.As(err, &configFileAlreadyExistsError) {
					return fmt.Errorf("could not write config file: %w", err)
				}
			}
		}

		logger.Infof("Using the config file at %s.", fConfigFile)
		asvConfig.SetConfigFile(fConfigFile)
	}

	// If a config file is found, read it in. We use a robust error check to
	// ignore "file not found" errors, but panic on any other error.
	if err := asvConfig.ReadInConfig(); err != nil {
		// It's okay if the config file doesn't exist.
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return err
		}
	}

	// This is the magic that makes the flag values available through Viper. It
	// binds the full flag set of the command passed in.
	err := asvConfig.BindPFlags(cmd.Flags())
	if err != nil {
		return err
	}

	return nil
}

// Copyright 2025-2026, Northwood Labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

var (
	fConfigFile string
	fVerbose    int
	fJSON       bool

	asvConfig = viper.New()

	awsConfigFilePath string
	userHomeDir       string
	logger            = log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		TimeFormat:      time.Kitchen,
	})
	// Test seam to verify wrapper behavior without invoking real process exit.
	fangNotifySignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	fangExecute       = fang.Execute
	runRootCommand    = func(ctx context.Context, cmd *cobra.Command, signals ...os.Signal) error {
		return fangExecute(ctx, cmd, fang.WithNotifySignal(signals...))
	}
	osExit = os.Exit

	rootCmd = &cobra.Command{
		Use:   "aws-sso-manager",
		Short: "Sets up your AWS SSO credentials into your AWS CLI config.",
		Long: clihelpers.LongHelpText(`
		AWS SSO Manager sets up your AWS Identity Center (née SSO) credentials into
		your AWS CLI config.

		This allows you to use the AWS CLI with your SSO accounts seamlessly. It
		also enables the use of AWS Vault with SSO.
		`),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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
	var err error

	userHomeDir, err = os.UserHomeDir()
	if err != nil {
		cobra.CheckErr(err)
	}

	awsConfigFilePath = config.DefaultSharedConfigFilename()

	rootCmd.PersistentFlags().StringVarP(
		&fConfigFile, "config", "c", path.Join(userHomeDir, ".config", "aws-sso-manager", "config.toml"),
		"configuration file",
	)
	rootCmd.PersistentFlags().CountVarP(
		&fVerbose, "verbose", "v",
		"increase verbosity level (can be used multiple times)",
	)
}

// Execute configures the Cobra CLI app framework and executes the root command.
func Execute() {
	if err := runRootCommand(
		context.Background(),
		rootCmd,
		fangNotifySignals...,
	); err != nil {
		osExit(1)
	}
}

// Root exposes the root command for tools like doc generators.
// https://cobra.dev/docs/how-to-guides/clis-for-llms/
func Root() *cobra.Command {
	return rootCmd
}

func initializeConfig(cmd *cobra.Command) error {
	asvConfig.SetEnvPrefix("ASV") // AWS SSO Manager
	asvConfig.SetEnvKeyReplacer(strings.NewReplacer(".", "*", "-", "*"))
	asvConfig.AutomaticEnv()

	defaultConfigFile := path.Join(userHomeDir, ".config", "aws-sso-manager", "config.toml")

	if fConfigFile != defaultConfigFile {
		logger.Info("Config file is set via flag", "file", fConfigFile)

		// Use config file from the flag.
		_, err := os.Stat(fConfigFile)
		if os.IsNotExist(err) {
			logger.Info("Config file does not exist", "file", fConfigFile)

			return fmt.Errorf("config file does not exist at %s", fConfigFile)
		}

		asvConfig.SetConfigFile(fConfigFile)
	} else {
		_, err := os.Stat(fConfigFile)
		if os.IsNotExist(err) {
			logger.Info("Config file does not exist. Create a new one.")

			err = os.MkdirAll(filepath.Dir(defaultConfigFile), 0o0755)
			if err != nil {
				return fmt.Errorf("could not create config directory: %w", err)
			}

			asvConfig.SetConfigType("toml")

			err = asvConfig.WriteConfigAs(defaultConfigFile)
			if err != nil {
				logger.Info("!!!!!! This should not happen !!!!!!")

				var configFileAlreadyExistsError viper.ConfigFileAlreadyExistsError
				if !errors.As(err, &configFileAlreadyExistsError) {
					return fmt.Errorf("could not write config file: %w", err)
				}
			}
		}

		logger.Info("Using the config file", "file", fConfigFile)
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

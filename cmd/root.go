// Copyright 2025-2026, Northwood Labs, LLC <license@northwood-labs.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
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
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"charm.land/fang/v2"
	"charm.land/log/v2"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

const (
	noVerbose    = 0
	verboseInfo  = 1
	verboseDebug = 2
)

var (
	fConfigFile    string
	fCacheDuration string
	fVerbose       int
	fJSON          bool

	// AsmConfig is the Viper instance that merges TOML config, env vars, and
	// CLI flags into a single configuration source. It's a package-level var
	// so every command can read profile-specific settings without passing it
	// around.
	asmConfig = viper.New()

	awsConfigFilePath  string
	awsManagerCacheDir string
	cacheDuration      = 24 * time.Hour
	userHomeDir        string

	// Charmlogger defaults to stderr with caller info and timestamps. The
	// PersistentPreRunE adjusts the level based on -v count so that normal
	// usage is quiet and -vvv gives full debug output with source locations.
	charmlogger = log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		TimeFormat:      time.Kitchen,
	})

	logger *slog.Logger

	// FangNotifySignals, fangExecute, runRootCommand, and osExit are test seams.
	// They let tests verify signal handling and exit behavior without actually
	// killing the test process or requiring real signal delivery.
	fangNotifySignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	fangExecute       = fang.Execute
	runRootCommand    = func(ctx context.Context, cmd *cobra.Command, signals ...os.Signal) error {
		return fangExecute(ctx, cmd, fang.WithNotifySignal(signals...))
	}
	osExit = os.Exit

	rootCmd = &cobra.Command{
		Use:   "aws-sso-manager [subcommand]",
		Short: "Sets up your AWS SSO credentials into your AWS CLI config.",
		Long: clihelpers.LongHelpText(`
		AWS SSO Manager sets up your AWS Identity Center (née SSO) credentials into
		your AWS CLI config.

		This allows you to use the AWS CLI with your SSO accounts seamlessly. It
		also enables the use of AWS Vault with SSO.
		`),
		Example: strings.TrimSpace(dedent.Dedent(`
		# Initialize a new SSO profile.
		aws-sso-manager init [sso-profile-name]

		# Authenticate with an SSO profile.
		aws-sso-manager auth [sso-profile-name]

		# List all AWS accounts accessible with this SSO profile.
		aws-sso-manager list [sso-profile-name]

		# Update the .aws/config file with the AWS accounts/roles you have access to.
		aws-sso-manager update [sso-profile-name]
		`)),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			charmlogger.SetStyles(clihelpers.GetLoggerStyles())

			// Verbose levels: 0=warn (quiet default), 1=info (-v), 2=debug
			// (-vv), 3+=debug with source file:line (-vvv). ReportCaller is
			// expensive so it's only enabled at the highest level for deep
			// debugging.
			switch fVerbose {
			case noVerbose:
				charmlogger.SetLevel(log.WarnLevel)
				charmlogger.SetReportCaller(false)
			case verboseInfo:
				charmlogger.SetLevel(log.InfoLevel)
				charmlogger.SetReportCaller(false)
			case verboseDebug:
				charmlogger.SetLevel(log.DebugLevel)
				charmlogger.SetReportCaller(false)
			default:
				charmlogger.SetLevel(log.DebugLevel)
				charmlogger.SetReportCaller(true)
			}

			// At the end, convert charmlogger to a slog logger.
			logger = slog.New(charmlogger)

			parsedCacheDuration, err := parseCacheDurationFlag(fCacheDuration)
			if err != nil {
				return fmt.Errorf("could not parse cache duration: %w", err)
			}

			cacheDuration = parsedCacheDuration

			return initializeConfig(cmd)
		},
	}
)

// This runs too early in the process to use the logger.
func init() { // lint:allow_init
	var err error

	userHomeDir, err = os.UserHomeDir()
	if err != nil {
		cobra.CheckErr(err)
	}

	// The SDK doesn't appear to provide access to a fully-resolved config file
	// location, so we'll do it quick-and-dirty here.
	awsConfigFilePath = config.DefaultSharedConfigFilename()

	envConfigFile := os.Getenv("AWS_CONFIG_FILE")

	if envConfigFile != "" {
		awsConfigFilePath = envConfigFile
	}

	awsManagerCacheDir = path.Join(userHomeDir, ".config", "aws-sso-manager", "cache")
	fCacheDuration = "24h"

	rootCmd.PersistentFlags().StringVarP(
		&fConfigFile, "config", "c", path.Join(userHomeDir, ".config", "aws-sso-manager", "config.toml"),
		"configuration file",
	)
	rootCmd.PersistentFlags().StringVarP(
		&fCacheDuration, "cache-duration", "D", fCacheDuration,
		"duration to keep AWS account list cache entries (supports Go durations plus 'd', e.g. 24h, 1d, 6h30m)",
	)
	rootCmd.PersistentFlags().CountVarP(
		&fVerbose, "verbose", "v",
		"increase verbosity level (can be used multiple times)",
	)
}

// parseCacheDurationFlag extends Go's time.ParseDuration with a "d" (day) suffix
// because cache lifetimes are commonly expressed in days (e.g., "1d", "2d12h")
// and Go's stdlib doesn't support that. Day tokens are converted to hours before
// parsing so the rest of the duration string is handled normally.
func parseCacheDurationFlag(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, ErrCacheDurationEmpty // lint:allow_raw_number
	}

	dayTokenPattern := regexp.MustCompile(`(?i)(\d+)d`)
	normalized := trimmed

	matches := dayTokenPattern.FindAllStringSubmatch(trimmed, -1)
	for _, match := range matches {
		days, err := strconv.Atoi(match[1]) // lint:allow_raw_number
		if err != nil {
			return 0, fmt.Errorf("invalid day token in cache duration %q: %w", raw, err) // lint:allow_raw_number
		}

		normalized = strings.Replace(
			normalized,
			match[0],
			fmt.Sprintf("%dh", days*24), // lint:allow_raw_number
			1,                           // lint:allow_raw_number
		)
	}

	parsed, err := time.ParseDuration(normalized)
	if err != nil {
		return 0, fmt.Errorf("invalid cache duration %q: %w", raw, err) // lint:allow_raw_number
	}

	if parsed <= 0 { // lint:allow_raw_number
		return 0, fmt.Errorf("%w: %q", ErrCacheDurationInvalid, raw) // lint:allow_raw_number
	}

	return parsed, nil
}

// Execute configures the Cobra CLI app framework and executes the root command.
func Execute() {
	err := runRootCommand(
		context.Background(),
		rootCmd,
		fangNotifySignals...,
	)
	if err != nil {
		osExit(1) // lint:allow_raw_number
	}
}

// Root exposes the root command for tools like doc generators.
// https://cobra.dev/docs/how-to-guides/clis-for-llms/
func Root() *cobra.Command {
	return rootCmd
}

// initializeConfig wires up Viper to merge config file, env vars (ASM_ prefix),
// and CLI flags into a single config source. The precedence is:
// flags > env vars > config file > defaults. This lets users override any
// setting at any level without editing files.
func initializeConfig(cmd *cobra.Command) error {
	ctx := cmd.Context()

	asmConfig.SetEnvPrefix("ASM") // AWS SSO Manager.

	// Map dots and hyphens in config keys to underscores for env var lookup,
	// so "profile-name" in TOML becomes ASM_PROFILE_NAME as an env var.
	asmConfig.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	asmConfig.AutomaticEnv()

	defaultConfigFile := path.Join(userHomeDir, ".config", "aws-sso-manager", "config.toml")

	if fConfigFile != defaultConfigFile {
		logger.InfoContext(ctx, "Config file is set via flag", logKeyFile, fConfigFile)

		// Use config file from the flag.
		_, err := os.Stat(fConfigFile)
		if os.IsNotExist(err) {
			logger.InfoContext(ctx, "Config file does not exist", logKeyFile, fConfigFile)

			return fmt.Errorf("%w: %s", ErrConfigFileNotExist, fConfigFile)
		}

		asmConfig.SetConfigFile(fConfigFile)
	} else {
		err := ensureDefaultConfigFile(ctx, defaultConfigFile)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		logger.InfoContext(ctx, "Using the config file", logKeyFile, fConfigFile)
		asmConfig.SetConfigFile(fConfigFile)
	}

	// We can print globals here and they will show up in verbose logs for all
	// subcommands.
	logger.InfoContext(ctx, "Using AWS config file", logKeyFile, awsConfigFilePath)

	// If a config file is found, read it in. We use a robust error check to
	// ignore "file not found" errors, but panic on any other error.
	if err := asmConfig.ReadInConfig(); err != nil {
		// It's okay if the config file doesn't exist.
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return fmt.Errorf("could not read config file: %w", err)
		}
	}

	// This is the magic that makes the flag values available through Viper. It
	// binds the full flag set of the command passed in.
	err := asmConfig.BindPFlags(cmd.Flags())
	if err != nil {
		return fmt.Errorf("could not bind flags: %w", err)
	}

	return nil
}

// ensureDefaultConfigFile creates the default config file and its parent
// directory if they do not already exist. This avoids deep nesting in
// initializeConfig by isolating the "first-run" creation logic.
func ensureDefaultConfigFile(ctx context.Context, defaultConfigFile string) error {
	_, err := os.Stat(defaultConfigFile)
	if !os.IsNotExist(err) {
		return nil
	}

	logger.InfoContext(ctx, "Config file does not exist. Create a new one.")

	mkdirErr := os.MkdirAll(filepath.Dir(defaultConfigFile), 0o0755) // lint:allow_raw_number
	if mkdirErr != nil {
		return fmt.Errorf("could not create config directory: %w", mkdirErr)
	}

	asmConfig.SetConfigType("toml")

	writeErr := asmConfig.WriteConfigAs(defaultConfigFile)
	if writeErr != nil {
		logger.InfoContext(ctx, "Config file already exists")

		var configFileAlreadyExistsError viper.ConfigFileAlreadyExistsError
		if !errors.As(writeErr, &configFileAlreadyExistsError) {
			return fmt.Errorf("could not write config file: %w", writeErr)
		}
	}

	return nil
}

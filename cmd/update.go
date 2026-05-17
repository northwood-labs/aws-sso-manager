// Copyright 2025-2026, Northwood Labs
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
	"fmt"
	"os"
	"strings"

	"charm.land/huh/v2/spinner"
	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	configFile "github.com/northwood-labs/aws-config-parser/ini"
	clihelpers "github.com/northwood-labs/cli-helpers"
)

// updateCmd synchronizes ~/.aws/config with the current set of accounts and
// roles from AWS Identity Center. It rebuilds the managed block from scratch on
// every run so that stale profiles (from accounts or roles the user no longer
// has access to) are automatically removed.
var updateCmd = &cobra.Command{
	Use:   "update [sso-profile-name]",
	Short: "Simplifies updating accounts and roles in the AWS config.",
	Long: clihelpers.LongHelpText(`
	Simplifies updating accounts and roles in the AWS config.

	This command provides a streamlined way for users to update the AWS accounts
	and roles in their AWS SSO Manager configuration, ensuring that their setup
	remains current and accurate.
	`),
	Args:    cobra.RangeArgs(0, 1),
	Aliases: []string{"upgrade", "sync"},
	Example: strings.TrimSpace(dedent.Dedent(`
	# Update profiles in the default SSO profile.
	aws-sso-manager update

	# Update profiles in a specific SSO profile.
	aws-sso-manager update <sso-profile-name>
	`)),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := ""

		ctx := cmd.Context()

		logger.InfoContext(ctx, "Passed arguments", logKeyCount, len(args))

		if len(args) == 1 {
			profileName = args[0]
		} else {
			profileName = asmConfig.GetString("profile-name")
		}

		if profileName == "" {
			if err := promptProfileSelect(&profileName); err != nil {
				return fmt.Errorf("prompting for profile selection: %w", err)
			}
		}

		configLock, err := acquireAWSConfigLock(ctx)
		if err != nil {
			return fmt.Errorf("acquiring AWS config lock: %w", err)
		}
		defer func() {
			if releaseErr := configLock.Release(); releaseErr != nil {
				logger.ErrorContext(ctx, "Failed to release AWS config lock", logKeyErr, releaseErr)
			}
		}()

		logger.InfoContext(ctx, "Retrieving SSO session profile", logKeyProfile, profileName)

		tmpFilename, err := getManagedSection(profileName)
		cobra.CheckErr(err)

		logger.InfoContext(ctx, "Read the AWS config file", logKeyConfig, tmpFilename)

		sections, err := loadAWSConfig(ctx, tmpFilename)
		cobra.CheckErr(err)

		logger.InfoContext(ctx, "Retrieving SSO session profile", logKeyProfile, profileName)

		// Generate a SSO session profile from the profile name.
		sessionProfile, err := getSsoSession(ctx, profileName)
		if err != nil {
			return fmt.Errorf("getting SSO session for profile %q: %w", profileName, err)
		}

		sdkConfig, err := getSDKConfig(ctx, sessionProfile)
		if err != nil {
			return fmt.Errorf("getting SDK config: %w", err)
		}

		cache, err := getOrRefreshAuthenticatedCache(ctx, profileName, sessionProfile)
		if err != nil {
			return fmt.Errorf("could not ensure authentication for profile %q: %w", profileName, err)
		}

		ssoProfile := "sso-session " + profileName

		_, ok := sections.GetSection(ssoProfile)
		if !ok {
			cobra.CheckErr(fmt.Sprintf("config file does not have section [%s]; need to run init", ssoProfile))
		}

		err = spinner.New().
			WithOutput(os.Stderr).
			Title("Looking up accounts and roles...").
			Type(spinner.Dots).
			Action(func(accounts *listAccounts) func() {
				return func() {
					accts, fetchErr := listAWSAccounts(&listAWSAccountsInput{
						Cmd:           cmd,
						SDKConfig:     &sdkConfig,
						Cache:         cache,
						Logger:        logger,
						ProfileName:   profileName,
						AccountFilter: fAccounts,
						RoleFilter:    fRoles,
					})
					cobra.CheckErr(fetchErr)

					*accounts = accts
				}
			}(&accounts)).
			Run()
		if err != nil {
			cobra.CheckErr(err)
		}

		nextSections, counter, err := buildUpdatedManagedSections(sections, ssoProfile, profileName, accounts)
		if err != nil {
			return fmt.Errorf("building updated managed sections: %w", err)
		}

		f, err := os.OpenFile( // lint:allow_dynamic_filename
			tmpFilename,
			os.O_TRUNC|os.O_WRONLY,
			0o0644, // lint:allow_raw_number
		)
		cobra.CheckErr(err)

		defer func() {
			err = f.Close()
			cobra.CheckErr(err)
		}()

		_, err = f.WriteString(strings.TrimSpace(generateAWSConfig(nextSections)) + "\n")
		cobra.CheckErr(err)

		logger.DebugContext(ctx, "Temporary file", logKeyTempFile, tmpFilename)

		backupFilename, err := setManagedSection(tmpFilename, profileName)
		cobra.CheckErr(err)

		logger.DebugContext(ctx, "Backup file", logKeyBackupFile, backupFilename)

		// Set permissions to match the expected config file mode before rename.
		err = os.Chmod(backupFilename, 0o0644) // lint:allow_raw_number
		cobra.CheckErr(err)

		logger.DebugContext(ctx, "Deleted file", logKeyFile, tmpFilename)

		err = os.Remove(tmpFilename)
		cobra.CheckErr(err)

		// Atomic rename: the config file is replaced in a single OS operation,
		// eliminating the window where the file would be empty on an interrupted write.
		logger.DebugContext(ctx, "Rename file", logKeyFrom, backupFilename, logKeyTo, awsConfigFilePath)

		err = os.Rename(backupFilename, awsConfigFilePath)
		cobra.CheckErr(err)

		fmt.Printf("Updated %d profiles for %q.\n", counter, profileName)
		fmt.Fprintf(
			os.Stderr,
			"Note: used cached account data. Run \"aws-sso-manager list %s --no-cache\" first to fetch fresh data.\n",
			profileName,
		)

		return nil
	},
}

func init() { // lint:allow_init
	rootCmd.AddCommand(updateCmd)
}

// buildUpdatedManagedSections creates a fresh set of INI sections from the
// current account/role list. It intentionally starts from an empty Sections
// (preserving only the [sso-session]) so that profiles for accounts or roles
// the user no longer has access to are dropped. The returned count lets the
// caller report how many profiles were written.
func buildUpdatedManagedSections( // lint:allow_complexity
	sections configFile.Sections,
	ssoProfile,
	profileName string,
	accounts listAccounts,
) (configFile.Sections, int, error) {
	ctx := context.Background()

	ssoSection, ok := sections.GetSection(ssoProfile)
	if !ok {
		return configFile.NewSections(), 0, fmt.Errorf(
			"%w: [%s]; need to run init",
			ErrConfigSectionMissing,
			ssoProfile,
		)
	}

	// Rebuild the managed block from scratch on each update so that the
	// output reflects the complete current account/role list (stale profiles
	// from previous runs are intentionally dropped).
	nextSections := configFile.NewSections()

	nextSections = nextSections.SetSection(ssoProfile, ssoSection)

	// Resolve per-profile overrides once so every generated [profile ...]
	// block in this managed section shares the same values (Req 4.1, 4.2).
	resolvedRegion := asmConfig.GetString(profileName + ".settings.global.region")
	if resolvedRegion == "" {
		resolvedRegion = ssoSection.String("sso_region")
	}

	resolvedOutput := asmConfig.GetString(profileName + ".settings.global.output")
	if resolvedOutput == "" {
		resolvedOutput = "json"
	}

	// Optional keys — only written to the INI when explicitly configured.
	globalPrefix := profileName + ".settings.global."
	resolvedDurationSeconds := asmConfig.GetString(globalPrefix + "duration_seconds")
	resolvedSDKUAAppID := asmConfig.GetString(globalPrefix + "sdk_ua_app_id")
	resolvedUseDualstack := asmConfig.GetString(globalPrefix + "use_dualstack_endpoint")
	resolvedUseFIPS := asmConfig.GetString(globalPrefix + "use_fips_endpoint")
	resolvedTCPKeepAlive := asmConfig.GetString(globalPrefix + "tcp_keepalive")

	counter := 0

	for _, account := range accounts.Accounts {
		for _, role := range account.Roles {
			counter++

			profileHeaderName := "profile " + role.Profile

			logger.InfoContext(ctx, "Processing profile", logKeyProfile, profileHeaderName)

			section, ok := nextSections.GetSection(profileHeaderName)
			if !ok {
				logger.InfoContext(
					ctx,
					"Config file does not have section; creating new section",
					logKeySection,
					profileHeaderName,
				)

				section = configFile.NewSection(profileHeaderName)
			}

			m := map[string]string{}

			m["sso_session"] = profileName
			m["sso_account_id"] = role.AccountID
			m["sso_role_name"] = role.Name

			// Per-profile settings override the global defaults when non-empty.
			perPrefix := profileName + ".settings." + role.Profile + "."

			profileRegion := asmConfig.GetString(perPrefix + "region")
			if profileRegion != "" {
				m["region"] = profileRegion
			} else {
				m["region"] = resolvedRegion
			}

			profileOutput := asmConfig.GetString(perPrefix + "output")
			if profileOutput != "" {
				m["output"] = profileOutput
			} else {
				m["output"] = resolvedOutput
			}

			// Optional keys — written only when set at per-profile or global level.
			optionalKeys := []struct {
				iniKey       string
				globalVal    string
				configSuffix string
			}{
				{"duration_seconds", resolvedDurationSeconds, "duration_seconds"},
				{"sdk_ua_app_id", resolvedSDKUAAppID, "sdk_ua_app_id"},
				{"use_dualstack_endpoint", resolvedUseDualstack, "use_dualstack_endpoint"},
				{"use_fips_endpoint", resolvedUseFIPS, "use_fips_endpoint"},
				{"tcp_keepalive", resolvedTCPKeepAlive, "tcp_keepalive"},
			}

			for _, opt := range optionalKeys {
				if v := asmConfig.GetString(perPrefix + opt.configSuffix); v != "" {
					m[opt.iniKey] = v
				} else if opt.globalVal != "" {
					m[opt.iniKey] = opt.globalVal
				}
			}

			for iniKey, iniValue := range m {
				v, err := configFile.NewStringValue(iniValue)
				if err != nil {
					return configFile.NewSections(), 0, fmt.Errorf("failed to create '%s' value: %w", iniKey, err)
				}

				err = section.UpdateValue(iniKey, v)
				if err != nil {
					return configFile.NewSections(), 0, fmt.Errorf("failed to update '%s' value: %w", iniKey, err)
				}
			}

			logger.InfoContext(
				ctx,
				"Get the section or create it if it does not exist",
				logKeySection,
				profileHeaderName,
			)

			nextSections = nextSections.SetSection(profileHeaderName, section)
			if _, ok = nextSections.GetSection(profileHeaderName); !ok {
				return configFile.NewSections(), 0, fmt.Errorf(
					"%w: [%s]",
					ErrConfigSectionCreate,
					profileHeaderName,
				)
			}
		}
	}

	return nextSections, counter, nil
}

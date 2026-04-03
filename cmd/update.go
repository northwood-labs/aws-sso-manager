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
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
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
		var (
			ok          bool
			profileName string
			counter     int
		)

		logger.Info("Passed arguments", "count", len(args))

		if len(args) == 1 {
			profileName = args[0]
		} else {
			profileName = asmConfig.GetString("profile-name")
		}

		if profileName == "" {
			err := huh.NewInput().
				Title("SSO profile name").
				Description("should be short; no spaces").
				Value(&profileName).
				Run()
			if err != nil {
				return err
			}
		}

		configLock, err := acquireAWSConfigLock(cmd.Context())
		if err != nil {
			return err
		}
		defer func() {
			if releaseErr := configLock.Release(); releaseErr != nil {
				logger.Error("Failed to release AWS config lock", "err", releaseErr)
			}
		}()

		logger.Info("Retrieving SSO session profile", "profile", profileName)

		tmpFilename, err := getManagedSection(profileName)
		cobra.CheckErr(err)

		logger.Info("Read the AWS config file", "config", tmpFilename)

		sections, err := loadAWSConfig(tmpFilename)
		cobra.CheckErr(err)

		logger.Info("Retrieving SSO session profile", "profile", profileName)

		// Generate a SSO session profile from the profile name.
		sessionProfile, err := getSsoSession(profileName)
		if err != nil {
			return err
		}

		sdkConfig, err := getSDKConfig(cmd.Context(), sessionProfile)
		if err != nil {
			return err
		}

		cache, err := getOrRefreshAuthenticatedCache(cmd.Context(), profileName, sessionProfile)
		if err != nil {
			return fmt.Errorf("could not ensure authentication for profile %q: %w", profileName, err)
		}

		ssoProfile := fmt.Sprintf("sso-session %s", profileName)

		_, ok = sections.GetSection(ssoProfile)
		if !ok {
			cobra.CheckErr(fmt.Sprintf("config file does not have section [%s]; need to run init", ssoProfile))
		}

		err = spinner.New().
			Output(os.Stderr).
			Title("Looking up accounts and roles...").
			Type(spinner.Dots).
			Action(func(accounts *listAccounts) func() {
				return func() {
					accts, err := listAWSAccounts(listAWSAccountsInput{
						Cmd:           cmd,
						SDKConfig:     &sdkConfig,
						Cache:         cache,
						Logger:        logger,
						ProfileName:   profileName,
						AccountFilter: fAccounts,
						RoleFilter:    fRoles,
					})
					cobra.CheckErr(err)

					*accounts = accts
				}
			}(&accounts)).
			Run()
		if err != nil {
			cobra.CheckErr(err)
		}

		nextSections, counter, err := buildUpdatedManagedSections(sections, ssoProfile, profileName, accounts)
		if err != nil {
			return err
		}

		f, err := os.OpenFile(tmpFilename, os.O_TRUNC|os.O_WRONLY, 0o0644)
		cobra.CheckErr(err)

		defer func() {
			err = f.Close()
			cobra.CheckErr(err)
		}()

		_, err = f.WriteString(strings.TrimSpace(generateAWSConfig(nextSections)) + "\n")
		cobra.CheckErr(err)

		logger.Debug("", "temp file", tmpFilename)

		backupFilename, err := setManagedSection(tmpFilename, profileName)
		cobra.CheckErr(err)

		logger.Debug("", "backup file", backupFilename)

		// Set permissions to match the expected config file mode before rename.
		err = os.Chmod(backupFilename, 0o0644)
		cobra.CheckErr(err)

		logger.Debug("Deleted file", "file", tmpFilename)
		err = os.Remove(tmpFilename)
		cobra.CheckErr(err)

		// Atomic rename: the config file is replaced in a single OS operation,
		// eliminating the window where the file would be empty on an interrupted write.
		logger.Debug("Rename file", "from", backupFilename, "to", awsConfigFilePath)
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

// buildUpdatedManagedSections creates a fresh set of INI sections from the
// current account/role list. It intentionally starts from an empty Sections
// (preserving only the [sso-session]) so that profiles for accounts or roles
// the user no longer has access to are dropped. The returned count lets the
// caller report how many profiles were written.
func buildUpdatedManagedSections(
	sections configFile.Sections,
	ssoProfile,
	profileName string,
	accounts listAccounts,
) (configFile.Sections, int, error) {
	ssoSection, ok := sections.GetSection(ssoProfile)
	if !ok {
		return configFile.NewSections(), 0, fmt.Errorf(
			"config file does not have section [%s]; need to run init",
			ssoProfile,
		)
	}

	// Rebuild the managed block from scratch on each update so that the
	// output reflects the complete current account/role list (stale profiles
	// from previous runs are intentionally dropped).
	nextSections := configFile.NewSections()
	nextSections = nextSections.SetSection(ssoProfile, ssoSection)

	counter := 0

	for _, account := range accounts.Accounts {
		for _, role := range account.Roles {
			counter++
			profileHeaderName := fmt.Sprintf("profile %s", role.Profile)

			logger.Info("Processing profile", "profile", profileHeaderName)

			section, ok := nextSections.GetSection(profileHeaderName)
			if !ok {
				logger.Info("Config file does not have section; creating new section", "section", profileHeaderName)

				section = configFile.NewSection(profileHeaderName)
			}

			m := map[string]string{}

			m["sso_session"] = profileName
			m["sso_account_id"] = role.AccountID
			m["sso_role_name"] = role.Name
			m["region"] = ssoSection.String("sso_region")
			m["output"] = "json"

			for iniKey, iniValue := range m {
				if v, err := configFile.NewStringValue(iniValue); err != nil {
					return configFile.NewSections(), 0, fmt.Errorf("failed to create '%s' value: %w", iniKey, err)
				} else {
					err = section.UpdateValue(iniKey, v)
					if err != nil {
						return configFile.NewSections(), 0, fmt.Errorf("failed to update '%s' value: %w", iniKey, err)
					}
				}
			}

			logger.Info("Get the section or create it if it does not exist", "section", profileHeaderName)

			nextSections = nextSections.SetSection(profileHeaderName, section)
			if _, ok = nextSections.GetSection(profileHeaderName); !ok {
				return configFile.NewSections(), 0, fmt.Errorf(
					"failed to create or get section [%s]",
					profileHeaderName,
				)
			}
		}
	}

	return nextSections, counter, nil
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

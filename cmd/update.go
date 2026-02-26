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
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	configFile "github.com/northwood-labs/aws-config-parser/ini"
	clihelpers "github.com/northwood-labs/cli-helpers"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Simplifies updating accounts and roles in the AWS config.",
	Long: clihelpers.LongHelpText(`
	Simplifies updating accounts and roles in the AWS config.

	This command provides a streamlined way for users to update the AWS accounts
	and roles in their AWS SSO Vault configuration, ensuring that their setup
	remains current and accurate.
	`),
	Args: cobra.RangeArgs(0, 1),
	Aliases: []string{"upgrade", "sync"},
	Example: strings.TrimSpace(dedent.Dedent(`
	aws-sso-vault update
	aws-sso-vault update <sso-profile>
	`)),
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			ok                bool
			cacheData         cacheFileData
			section           configFile.Section
			profileName       string
			profileHeaderName string
			counter           int
		)

		logger.Infof("Passed %d arguments.", len(args))

		if len(args) == 1 {
			profileName = args[0]
		} else {
			profileName = asvConfig.GetString("profile-name")
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

		logger.Infof("Retrieving SSO session profile for %s...", profileName)

		tmpFilename, err := getManagedSection(profileName)
		cobra.CheckErr(err)

		logger.Infof("Read the AWS config file at %s.", tmpFilename)

		sections, err := loadAWSConfig(tmpFilename)
		cobra.CheckErr(err)

		logger.Infof("Retrieving SSO session profile for %s...", profileName)

		// Generate a SSO session profile from the profile name.
		sessionProfile, err := getSsoSession(profileName)
		if err != nil {
			return err
		}

		sdkConfig, err := getSDKConfig(sessionProfile)
		if err != nil {
			return err
		}

		// Where does the cache file live?
		cacheFilePath, err := getCacheFilePath(&sessionProfile)
		if err != nil {
			return err
		}

		// Can we read the cache?
		cache, err := cacheData.read(cacheFilePath)
		if err != nil {
			return fmt.Errorf("not authenticated: %w", err)
		}

		ssoProfile := fmt.Sprintf("sso-session %s", profileName)

		ssoSection, ok := sections.GetSection(ssoProfile)
		if !ok {
			cobra.CheckErr(fmt.Sprintf("config file does not have section [%s]; need to run init", profileHeaderName))
		}

		err = spinner.New().
			Output(os.Stderr).
			Title("Looking up accounts and roles...").
			Type(spinner.Dots).
			Action(func(accounts *listAccounts) func() {
				return func() {
					accts, err := listAWSAccounts(cmd, &sdkConfig, cache, profileName, fAccounts, fRoles)
					cobra.CheckErr(err)

					*accounts = accts
				}
			}(&accounts)).
			Run()
		if err != nil {
			logger.Fatal(err)
		}

		for _, account := range accounts.Accounts {
			for _, role := range account.Roles {
				counter++
				profileHeaderName = fmt.Sprintf("profile %s", role.Profile)

				logger.Infof("Processing profile [%s]...", profileHeaderName)

				section, ok = sections.GetSection(profileHeaderName)
				if !ok {
					logger.Infof("Config file does not have section [%s]; creating new section.", profileHeaderName)

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
						return fmt.Errorf("failed to create '%s' value: %w", iniKey, err)
					} else {
						err = section.UpdateValue(iniKey, v)
						if err != nil {
							return fmt.Errorf("failed to update '%s' value: %w", iniKey, err)
						}
					}
				}

				logger.Infof("Get the [%s] section or create it if it does not exist.", profileHeaderName)

				sections = sections.SetSection(profileHeaderName, section)
				if _, ok = sections.GetSection(profileHeaderName); !ok {
					return fmt.Errorf("failed to create or get section [%s]", profileHeaderName)
				}

				defer func() {
					if r := recover(); r != nil {
						fmt.Println("Recovered:", r)
					}
				}()

			}
		}

		f, err := os.OpenFile(tmpFilename, os.O_TRUNC|os.O_WRONLY, 0o0644)
		cobra.CheckErr(err)

		defer func() {
			err = f.Close()
			cobra.CheckErr(err)
		}()

		_, err = f.WriteString(strings.TrimSpace(generateAWSConfig(sections))+"\n")
		cobra.CheckErr(err)

		logger.Debug("", "temp file", tmpFilename)

		backupFilename, err := setManagedSection(tmpFilename, profileName)
		cobra.CheckErr(err)

		logger.Debug("", "backup file", backupFilename)

		// Blank out the temp file.
		err = truncate(tmpFilename, 0o0666)
		cobra.CheckErr(err)

		logger.Debugf("Open %q for reading.", backupFilename)
		fBackup, err := os.OpenFile(backupFilename, os.O_RDONLY, 0o0644)
		cobra.CheckErr(err)

		logger.Debugf("Open %q for writing.", awsConfigFilePath)
		fConfig, err := os.OpenFile(awsConfigFilePath, os.O_WRONLY, 0o0644)
		cobra.CheckErr(err)

		logger.Debug("Copying data.")
		_, err = io.Copy(fConfig, fBackup)
		cobra.CheckErr(err)

		logger.Debugf("Closed %q for reading.", backupFilename)
		err = fBackup.Close()
		cobra.CheckErr(err)

		logger.Debugf("Closed %q for reading.", awsConfigFilePath)
		err = fConfig.Close()
		cobra.CheckErr(err)

		logger.Debugf("Deleted %q.", backupFilename)
		err = os.Remove(backupFilename)
		cobra.CheckErr(err)

		logger.Debugf("Deleted %q.", tmpFilename)
		err = os.Remove(tmpFilename)
		cobra.CheckErr(err)

		fmt.Printf("Updated %d profiles for %q.\n", counter, profileName)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

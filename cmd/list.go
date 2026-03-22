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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

type (
	listAccounts struct {
		Accounts []listAccount `json:"accounts"`
	}

	listAccount struct {
		ID    string     `json:"id"`
		Name  string     `json:"name"`
		Email string     `json:"email"`
		Roles []listRole `json:"roles"`
	}

	listRole struct {
		AccountID string `json:"account_id"`
		Name      string `json:"name"`
		Profile   string `json:"profile"`
	}
)

var (
	fAccounts string
	fRoles    string

	accounts listAccounts
	// profileID string

	cellStyle   = lipgloss.NewStyle().Padding(0, 1)
	headerStyle = cellStyle.Bold(true)

	// listCmd represents the list command
	listCmd = &cobra.Command{
		Use:   "list",
		Short: "Lists all configured AWS SSO accounts and their associated roles.",
		Long: clihelpers.LongHelpText(`
		Lists all configured AWS SSO accounts and their associated roles.

		This command provides an overview of the SSO accounts that have been set up
		in AWS SSO, allowing users to see which accounts and roles are available for
		authentication.
		`),
		Args:    cobra.RangeArgs(0, 1),
		Aliases: []string{"ls"},
		Example: strings.TrimSpace(dedent.Dedent(`
		aws-sso-manager list
		aws-sso-manager list <sso-profile>
		aws-sso-manager list <sso-profile> --json
		aws-sso-manager list <sso-profile> --accounts <substring>
		aws-sso-manager list <sso-profile> --roles <substring>
		`)),
		RunE: func(cmd *cobra.Command, args []string) error {
			var profileName string

			logger.Info("Passed arguments", "count", len(args))

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

			// Where does the cache file live?
			cacheFilePath, err := getCacheFilePath(&sessionProfile)
			if err != nil {
				return err
			}

			var cacheData cacheFileData

			// Can we read the cache?
			cache, err := cacheData.read(cacheFilePath)
			if err != nil {
				return fmt.Errorf("not authenticated: %w", err)
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

			if fJSON {
				data, err := json.Marshal(accounts)
				if err != nil {
					return fmt.Errorf("could not marshal accounts to JSON: %w", err)
				}

				fmt.Println(string(data))

				return err
			}

			if len(accounts.Accounts) == 0 {
				fmt.Println("No AWS accounts are assigned to this user.")

				os.Exit(0)
			}

			// Prepare table for TUI
			t := table.New().
				Border(lipgloss.RoundedBorder()).
				Headers("ID", "Account Name", "Role Name", "Profile Name").
				StyleFunc(func(row, _ int) lipgloss.Style {
					switch {
					case row == table.HeaderRow:
						return headerStyle
					default:
						return cellStyle
					}
				})

			for i := range accounts.Accounts {
				for j := range accounts.Accounts[i].Roles {
					// rowCount := 0
					// rowCount++

					accountName := accounts.Accounts[i].Name
					roleName := accounts.Accounts[i].Roles[j].Name
					profile := getProfileName(profileName, accountName, roleName)

					t.Row(
						accounts.Accounts[i].ID,
						accountName,
						roleName,
						profile,
					)
				}
			}

			_, err = lipgloss.Println(t)
			if err != nil {
				return fmt.Errorf("could not print table: %w", err)
			}

			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&fAccounts, "accounts", "a", "", "Filter by account name substring")
	listCmd.Flags().StringVarP(&fRoles, "roles", "r", "", "Filter by role name substring")
	listCmd.Flags().BoolVarP(&fJSON, "json", "j", false, "output in JSON format")
}

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
	"bytes"
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
	fNoCache  bool
	fCSV      bool
	fMarkdown bool

	accounts listAccounts
	// profileID string

	cellStyle   = lipgloss.NewStyle().Padding(0, 1)
	headerStyle = cellStyle.Bold(true)

	listOutputHeaders = []string{"ID", "Account Name", "Role Name", "Profile Name"}

	// listCmd represents the list command
	listCmd = &cobra.Command{
		Use:   "list [sso-profile-name]",
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
		aws-sso-manager list <sso-profile-name>
		aws-sso-manager list <sso-profile-name> --json
		aws-sso-manager list <sso-profile-name> --csv
		aws-sso-manager list <sso-profile-name> --markdown
		aws-sso-manager list <sso-profile-name> --no-cache
		aws-sso-manager list <sso-profile-name> --accounts <substring>
		aws-sso-manager list <sso-profile-name> --roles <substring>
		`)),
		RunE: func(cmd *cobra.Command, args []string) error {
			var profileName string

			selectedOutputs := 0
			if fJSON {
				selectedOutputs++
			}
			if fCSV {
				selectedOutputs++
			}
			if fMarkdown {
				selectedOutputs++
			}
			if selectedOutputs > 1 {
				return fmt.Errorf("choose only one output format flag: --json, --csv, or --markdown")
			}

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

			cache, err := getOrRefreshAuthenticatedCache(cmd.Context(), profileName, sessionProfile)
			if err != nil {
				return fmt.Errorf("could not ensure authentication for profile %q: %w", profileName, err)
			}

			listInput := listAWSAccountsInput{
				Cmd:           cmd,
				SDKConfig:     &sdkConfig,
				Cache:         cache,
				Logger:        logger,
				ProfileName:   profileName,
				AccountFilter: fAccounts,
				RoleFilter:    fRoles,
			}

			if fNoCache {
				if err := deleteListAWSAccountsCache(listInput); err != nil {
					return fmt.Errorf("could not clear accounts cache: %w", err)
				}
			}

			err = spinner.New().
				Output(os.Stderr).
				Title("Looking up accounts and roles...").
				Type(spinner.Dots).
				Action(func(accounts *listAccounts) func() {
					return func() {
						accts, err := listAWSAccounts(listInput)
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

			rows := buildListOutputRows(profileName, accounts)

			if fCSV {
				fmt.Print(renderCSVTable(listOutputHeaders, rows))
				return nil
			}

			if fMarkdown {
				fmt.Print(renderMarkdownTable(listOutputHeaders, rows))
				return nil
			}

			// Prepare table for TUI
			t := table.New().
				Border(lipgloss.RoundedBorder()).
				Headers(listOutputHeaders...).
				StyleFunc(func(row, _ int) lipgloss.Style {
					switch {
					case row == table.HeaderRow:
						return headerStyle
					default:
						return cellStyle
					}
				})

			for _, row := range rows {
				t.Row(row...)
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
	listCmd.Flags().
		BoolVarP(&fNoCache, "no-cache", "n", false, "Delete list cache first, then fetch and cache fresh account data")
	listCmd.Flags().BoolVarP(&fJSON, "json", "j", false, "output in JSON format")
	listCmd.Flags().BoolVarP(&fCSV, "csv", "C", false, "output in CSV format")
	listCmd.Flags().BoolVarP(&fMarkdown, "markdown", "M", false, "output in GitHub-Flavored Markdown table format")
}

func buildListOutputRows(profileName string, accounts listAccounts) [][]string {
	rows := make([][]string, 0)

	for i := range accounts.Accounts {
		for j := range accounts.Accounts[i].Roles {
			accountName := accounts.Accounts[i].Name
			roleName := accounts.Accounts[i].Roles[j].Name
			profile := getProfileName(profileName, accountName, roleName)

			rows = append(rows, []string{
				accounts.Accounts[i].ID,
				accountName,
				roleName,
				profile,
			})
		}
	}

	return rows
}

func quoteCSVCell(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func renderCSVTable(headers []string, rows [][]string) string {
	var buffer bytes.Buffer

	for _, row := range append([][]string{headers}, rows...) {
		quoted := make([]string, len(row))
		for i := range row {
			quoted[i] = quoteCSVCell(row[i])
		}

		buffer.WriteString(strings.Join(quoted, ","))
		buffer.WriteByte('\n')
	}

	return buffer.String()
}

func padRight(value string, width int) string {
	return value + strings.Repeat(" ", width-len(value))
}

func renderMarkdownTable(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}

	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var buffer bytes.Buffer

	buffer.WriteString("|")
	for i, header := range headers {
		buffer.WriteString(" ")
		buffer.WriteString(padRight(header, widths[i]))
		buffer.WriteString(" |")
	}
	buffer.WriteByte('\n')

	buffer.WriteString("|")
	for i := range headers {
		separatorWidth := max(widths[i], 3)

		buffer.WriteString(" ")
		buffer.WriteString(strings.Repeat("-", separatorWidth))
		buffer.WriteString(" |")
	}
	buffer.WriteByte('\n')

	for _, row := range rows {
		buffer.WriteString("|")
		for i, cell := range row {
			buffer.WriteString(" ")
			buffer.WriteString(padRight(cell, widths[i]))
			buffer.WriteString(" |")
		}
		buffer.WriteByte('\n')
	}

	return buffer.String()
}

// Copyright 2025, Northwood Labs
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
	"math"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

type (
	listModel struct {
		help     help.Model
		lastKey  string
		keys     keyMap
		table    table.Model
		quitting bool
	}

	// keyMap defines a set of keybindings. To work for help it must satisfy
	// key.Map. It could also very easily be a map[string]key.Binding.
	keyMap struct {
		Up    key.Binding
		Down  key.Binding
		Help  key.Binding
		Enter key.Binding
		Quit  key.Binding
	}

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

const height = 20

var (
	fAccounts string
	fRoles    string

	accounts listAccounts
	// profileID string

	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))

	keys = keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "make selection"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q/esc", "quit"),
		),
	}

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
		Args: cobra.RangeArgs(0, 1),
		Example: strings.TrimSpace(dedent.Dedent(`
		aws-sso-vault list
		aws-sso-vault list <sso-profile>
		aws-sso-vault list <sso-profile> --json
		aws-sso-vault list <sso-profile> --accounts <substring>
		aws-sso-vault list <sso-profile> --roles <substring>
		`)),
		RunE: func(cmd *cobra.Command, args []string) error {
			var profileName string

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
				os.Exit(0)
			}

			// Prepare table for TUI
			columns := []table.Column{
				{Title: "ID", Width: 20},           // lint:allow_raw_number
				{Title: "Account Name", Width: 20}, // lint:allow_raw_number
				{Title: "Role Name", Width: 35},    // lint:allow_raw_number
				{Title: "Profile Name", Width: 25}, // lint:allow_raw_number
			}

			rows := []table.Row{}
			rowCount := 0

			for i := range accounts.Accounts {
				for j := range accounts.Accounts[i].Roles {
					rowCount++

					accountName := accounts.Accounts[i].Name
					roleName := accounts.Accounts[i].Roles[j].Name
					profile := getProfileName(profileName, accountName, roleName)

					rows = append(
						rows,
						table.Row{
							accounts.Accounts[i].ID,
							accountName,
							roleName,
							profile,
						},
					)
				}
			}

			t := table.New(
				table.WithColumns(columns),
				table.WithRows(rows),
				table.WithFocused(true),
				table.WithHeight(
					int(math.Min(height, float64(rowCount+1))),
				),
			)

			s := table.DefaultStyles()
			s.Header = s.Header.
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240")).
				BorderBottom(true).
				Bold(false)
			s.Selected = s.Selected.
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57")).
				Bold(false)
			t.SetStyles(s)

			m := listModel{
				table: t,
				keys:  keys,
				help:  help.New(),
			}

			if _, err := tea.NewProgram(m).Run(); err != nil {
				fmt.Println("Error running program:", err)
				os.Exit(1)
			}

			// if profileID != "" {
			// 	fmt.Println(profileID)
			// }

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

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMap) ShortHelp() []key.Binding { // lint:allow_large_memory // Implementing a model I have no control over.
	return []key.Binding{
		k.Help,
		k.Enter,
		k.Quit,
	}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keyMap) FullHelp() [][]key.Binding { // lint:allow_large_memory // Implementing a model I have no control over.
	return [][]key.Binding{
		{ // first column
			k.Up,
			k.Down,
		},
		{ // second column
			k.Help,
			k.Quit,
		},
		{ // third column
			k.Enter,
		},
	}
}

func (m listModel) Init() tea.Cmd { // lint:allow_large_memory // Implementing a model I have no control over.
	return nil
}

func (m listModel) Update( // lint:allow_large_memory // Implementing a model I have no control over.
	msg tea.Msg,
) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// If we set a width on the help menu it can gracefully truncate
		// its view as needed.
		m.help.Width = msg.Width

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			m.lastKey = "↑"
		case key.Matches(msg, m.keys.Down):
			m.lastKey = "↓"
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Enter):
			m.quitting = true
			// profileID = m.table.SelectedRow()[3]

			return m, tea.Quit
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true

			return m, tea.Quit
		}
	}

	m.table, cmd = m.table.Update(msg)

	return m, cmd
}

func (m listModel) View() string { // lint:allow_large_memory // Implementing a model I have no control over.
	if m.quitting {
		return ""
	}

	helpView := m.help.View(m.keys)

	return baseStyle.Render(m.table.View()) + "\n" + helpView
}

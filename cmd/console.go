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
	"log"
	"math"
	"net/url"
	"os"
	"regexp"
	"strings"

	nativeclipboard "github.com/aymanbagabas/go-nativeclipboard"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	configFile "github.com/northwood-labs/aws-config-parser/ini"
	clihelpers "github.com/northwood-labs/cli-helpers"
)

var (
	fAccountID string
	fRegion    string
	fRole      string
	fClipboard bool

	// consoleCmd represents the diff command
	consoleCmd = &cobra.Command{
		Use:   "console [sso-profile-name] [url]",
		Short: "Generate an AWS Console URL for an AWS Account ID and role.",
		Long: clihelpers.LongHelpText(`
		Generate an AWS Console URL for an AWS Account ID and role.

		Enables you to create a link that allows a user to log directly into an
		account with a role to a specific AWS Console URL.

		If you do not specify the profile, account ID, or role, you will be
		interactively prompted to select them from your AWS access.
		`),
		Args: cobra.RangeArgs(0, 2),
		Example: strings.TrimSpace(dedent.Dedent(`
		aws-sso-manager console
		aws-sso-manager console --clipboard=false
		aws-sso-manager console <url>
		aws-sso-manager console <sso-profile>
		aws-sso-manager console <sso-profile> <url>
		`)),
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				consoleURL  string
				profileName string
				startHost   string
				err         error
				accounts    listAccounts

				accountID = fAccountID
				roleName  = fRole
			)

			logger.Info("Passed arguments", "count", len(args))

			if len(args) == 1 {
				switch {
				case strings.Contains(args[0], "://"):
					consoleURL = args[0]
				default:
					profileName = args[0]
				}
			} else if len(args) == 2 {
				profileName = args[0]
				consoleURL = args[1]
			} else {
				profileName = asvConfig.GetString("profile-name")
			}

			var groups []*huh.Group

			if consoleURL == "" {
				logger.Debugf("AWS Console URL is undefined. Collect it from user.")

				if fClipboard {
					// Set the default value to whatever's on the clipboard.
					clipboardBytes, err := nativeclipboard.Text.Read()
					if err != nil {
						logger.Debug("could not read from the clipboard", "error", err)
					}

					clipboardString := string(clipboardBytes)

					// But only if it's a valid console URL.
					if strings.Contains(clipboardString, "console.aws.amazon.com") {
						consoleURL = clipboardString
					}
				}

				groups = append(groups, huh.NewGroup(func() *huh.Input {
					return huh.NewInput().
						Title("The AWS Console URL...").
						Value(&consoleURL)
				}()))
			}

			if profileName == "" {
				logger.Debugf("SSO profile is undefined. Collect it from user.")

				groups = append(groups, huh.NewGroup(func() *huh.Select[string] {
					sections, e := getAllManagedSections()
					if e != nil {
						err = e
					}

					return huh.NewSelect[string]().
						Title("Select an SSO profile...").
						Value(&profileName).
						Height(minMaxRows(sections) + 1).
						Options(huh.NewOptions(sections...)...)
				}()))
			}

			logger.Info("Retrieving SSO session profile", "profile", profileName)

			// profileName should be defined at this point.
			startHost, err = getStartURL(profileName)
			if err != nil {
				return fmt.Errorf("failed to get start URL; re-init to create profile: %w", err)
			}

			// Generate a SSO session profile from the profile name.
			sessionProfile, err := getSsoSession(profileName)
			if err != nil {
				return fmt.Errorf("could not get SSO session: %w", err)
			}

			if fRegion != "" {
				logger.Info(
					"Using explicitly-configured region for console AWS calls",
					"profile",
					profileName,
					"region",
					fRegion,
				)
				sessionProfile.Region = fRegion
			}

			sdkConfig, err := getSDKConfig(cmd.Context(), sessionProfile)
			if err != nil {
				return fmt.Errorf("could not get AWS SDK configuration: %w", err)
			}

			cache, err := getOrRefreshAuthenticatedCache(cmd.Context(), profileName, sessionProfile)
			if err != nil {
				return fmt.Errorf("could not ensure authentication for profile %q: %w", profileName, err)
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
				logger.Fatal(err)
			}

			if accountID == "" {
				logger.Debugf("AWS Account ID is undefined. Collect it from user.")

				groups = append(groups, huh.NewGroup(func() *huh.Select[string] {
					return huh.NewSelect[string]().
						Title("The AWS account...").
						Value(&accountID).
						Height(minMaxRows(accounts.Accounts)+1).
						OptionsFunc(func() []huh.Option[string] {
							var accts []huh.Option[string]

							for _, acct := range accounts.Accounts {
								accts = append(accts, huh.NewOption(
									fmt.Sprintf(
										"%s (%s)",
										acct.Name,
										acct.ID,
									),
									acct.ID,
								))
							}

							return accts
						}, &accountID)
				}()))
			}

			if roleName == "" {
				logger.Debugf("AWS Organizations Role is undefined. Collect it from user.")

				groups = append(groups, huh.NewGroup(func() *huh.Select[string] {
					return huh.NewSelect[string]().
						Title("The AWS role...").
						Value(&roleName).
						Height(minMaxRows(accounts.Accounts)+1).
						OptionsFunc(func() []huh.Option[string] {
							var roles []huh.Option[string]
							roleList := getRolesForAccount(accounts.Accounts, accountID)

							for _, role := range roleList {
								roles = append(roles, huh.NewOption(
									role.Profile,
									role.Name,
								))
							}

							return roles
						}, &roleName)
				}()))
			}

			form := huh.NewForm(groups...)

			err = form.Run()
			if err != nil {
				log.Fatal(err)
			}

			logger.Debugf("Values have been collected: %+v", map[string]string{
				"profileName": profileName,
				"startHost":   startHost,
				"consoleURL":  consoleURL,
				"accountID":   accountID,
				"roleName":    roleName,
			})

			destinationURL := stripAccountFromURL(consoleURL)
			destinationURL = url.QueryEscape(destinationURL)

			finalURL := fmt.Sprintf(
				"https://%s/start/#/console?account_id=%s&role_name=%s&destination=%s",
				startHost,
				accountID,
				roleName,
				destinationURL,
			)

			if fClipboard {
				_, err = nativeclipboard.Text.Write([]byte(finalURL))
				if err != nil {
					logger.Debug("could not write URL to clipboard", "error", err)
				}
			}

			fmt.Println(finalURL)

			return nil
		},
	}
)

func init() {
	consoleCmd.Flags().StringVarP(
		&fAccountID,
		"account-id",
		"a",
		"",
		"The AWS Account ID to authenticate with",
	)

	consoleCmd.Flags().BoolVarP(
		&fClipboard,
		"clipboard",
		"C",
		true,
		"Whether or not to read from, or write to, the clipboard",
	)

	consoleCmd.Flags().StringVarP(
		&fRegion,
		"region",
		"R",
		"",
		"The AWS region to authenticate with",
	)

	consoleCmd.Flags().StringVarP(
		&fRole,
		"role",
		"r",
		"",
		"The AWS role to authenticate with",
	)

	rootCmd.AddCommand(consoleCmd)
}

func minMaxRows[T any](rows []T) int {
	return int(
		math.Min(
			10,
			float64(
				math.Max(
					5,
					float64(
						len(rows),
					),
				),
			),
		),
	)
}

func getRolesForAccount(accounts []listAccount, accountId string) []listRole {
	for _, acct := range accounts {
		if acct.ID == accountId {
			return acct.Roles
		}
	}

	return []listRole{}
}

func getStartURL(profileName string) (string, error) {
	sections, err := configFile.OpenFile(awsConfigFilePath)
	if err != nil {
		return "", err
	}

	if section, ok := sections.GetSection("sso-session " + profileName); ok {
		u, err := url.Parse(section.String("sso_start_url"))
		if err != nil {
			return "", err
		}

		return u.Hostname(), nil
	}

	return "", fmt.Errorf("could not discover 'sso_start_url' for profile '%s'", profileName)
}

func stripAccountFromURL(consoleURL string) string {
	reConsole := regexp.MustCompile(`https://([0-9a-zA-Z-]+)\.([0-9a-zA-Z-]+)\.console\.aws\.amazon\.com`)
	consoleURL = reConsole.ReplaceAllString(consoleURL, `https://${2}.console.aws.amazon.com`)

	return consoleURL
}

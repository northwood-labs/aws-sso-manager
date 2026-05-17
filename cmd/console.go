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
	"log"
	"math"
	"net/url"
	"os"
	"regexp"
	"strings"

	"charm.land/huh/v2"
	"charm.land/huh/v2/spinner"
	nativeclipboard "github.com/aymanbagabas/go-nativeclipboard"
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
		Args: cobra.RangeArgs(0, 2), // lint:allow_raw_number
		Example: strings.TrimSpace(dedent.Dedent(`
		# Prompt for both the AWS console URL and the SSO profile.
		aws-sso-manager console

		# Prompt for whichever is undefined.
		aws-sso-manager console <url>
		aws-sso-manager console <sso-profile>
		aws-sso-manager console <sso-profile> <url>

		# Disable reading from/writing to the clipboard.
		aws-sso-manager console --clipboard=false
		`)),
		RunE: func(cmd *cobra.Command, args []string) error {
			consoleURL := ""
			profileName := ""
			accounts := listAccounts{}
			accountID := fAccountID
			roleName := fRole

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			logger.InfoContext(ctx, "Passed arguments", logKeyCount, len(args))

			if len(args) == 1 {
				switch {
				case strings.Contains(args[0], "://"):
					consoleURL = args[0]
				default:
					profileName = args[0]
				}
			} else if len(args) == 2 { // lint:allow_raw_number
				profileName = args[0]
				consoleURL = args[1]
			} else {
				profileName = asmConfig.GetString("profile-name")
			}

			groups := []*huh.Group{}

			if consoleURL == "" {
				logger.DebugContext(ctx, "AWS Console URL is undefined. Collect it from user.")

				if fClipboard {
					// Set the default value to whatever's on the clipboard.
					clipboardBytes, err := nativeclipboard.Text.Read()
					if err != nil {
						logger.DebugContext(ctx, "Could not read from the clipboard", logKeyErr, err)
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
				logger.DebugContext(ctx, "SSO profile is undefined. Collect it from user.")

				if err := promptProfileSelect(&profileName); err != nil {
					return fmt.Errorf("could not select SSO profile: %w", err)
				}
			}

			logger.InfoContext(ctx, "Retrieving SSO session profile", logKeyProfile, profileName)

			// profileName should be defined at this point.
			startHost, err := getStartURL(profileName)
			if err != nil {
				return fmt.Errorf("failed to get start URL; re-init to create profile: %w", err)
			}

			// Generate a SSO session profile from the profile name.
			sessionProfile, err := getSsoSession(ctx, profileName)
			if err != nil {
				return fmt.Errorf("could not get SSO session: %w", err)
			}

			if fRegion != "" {
				logger.InfoContext(
					ctx,
					"Using explicitly-configured region for console AWS calls",
					logKeyProfile,
					profileName,
					logKeyRegion,
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

			if accountID == "" {
				logger.DebugContext(ctx, "AWS Account ID is undefined. Collect it from user.")

				groups = append(groups, huh.NewGroup(func() *huh.Select[string] {
					return huh.NewSelect[string]().
						Title("The AWS account...").
						Value(&accountID).
						Height(minMaxRows(accounts.Accounts)+1).
						OptionsFunc(func() []huh.Option[string] {
							accts := []huh.Option[string]{}

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
				logger.DebugContext(ctx, "AWS Organizations Role is undefined. Collect it from user.")

				groups = append(groups, huh.NewGroup(func() *huh.Select[string] {
					return huh.NewSelect[string]().
						Title("The AWS role...").
						Value(&roleName).
						Height(minMaxRows(accounts.Accounts)+1).
						OptionsFunc(func() []huh.Option[string] {
							roles := []huh.Option[string]{}

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

			logger.DebugContext(ctx, "Values have been collected",
				logKeyProfileName, profileName,
				logKeyStartHost, startHost,
				logKeyConsoleURL, consoleURL,
				logKeyAccountID, accountID,
				logKeyRoleName, roleName,
			)

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
					logger.DebugContext(ctx, "Could not write URL to clipboard", logKeyErr, err)
				}
			}

			fmt.Println(finalURL)

			return nil
		},
	}
)

func init() { // lint:allow_init
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

// minMaxRows clamps the TUI select list height between 5 and 10 rows. Too few
// rows makes the list feel cramped; too many pushes the prompt off-screen on
// small terminals.
func minMaxRows[T any](rows []T) int {
	return int(
		math.Min(
			10, // lint:allow_raw_number

			math.Max(
				5, // lint:allow_raw_number
				float64(
					len(rows),
				),
			),
		),
	)
}

// getRolesForAccount returns the roles for a specific account ID. This is used
// by the console command's interactive prompt to show only the roles available
// for the selected account.
func getRolesForAccount(accounts []listAccount, accountID string) []listRole {
	for _, acct := range accounts {
		if acct.ID == accountID {
			return acct.Roles
		}
	}

	return []listRole{}
}

// getStartURL extracts the SSO portal hostname from the config. The console
// command needs this to construct the final redirect URL — the hostname is the
// SSO portal that handles the account/role selection.
func getStartURL(profileName string) (string, error) {
	sections, err := configFile.OpenFile(awsConfigFilePath)
	if err != nil {
		return "", fmt.Errorf("could not open AWS config file: %w", err)
	}

	if section, ok := sections.GetSection("sso-session " + profileName); ok {
		u, err := url.Parse(section.String("sso_start_url"))
		if err != nil {
			return "", fmt.Errorf("could not parse sso_start_url: %w", err)
		}

		return u.Hostname(), nil
	}

	return "", fmt.Errorf("%w: %q", ErrStartURLNotFound, profileName)
}

// stripAccountFromURL removes the account-specific subdomain from an AWS
// Console URL. When a user copies a URL from their browser, it often includes
// an account subdomain (e.g., https://123456789012.s3.console.aws.amazon.com).
// The SSO redirect URL needs the generic form without the account subdomain,
// because the account is specified separately via the account_id parameter.
func stripAccountFromURL(consoleURL string) string {
	reConsole := regexp.MustCompile(`https://([0-9a-zA-Z-]+)\.([0-9a-zA-Z-]+)\.console\.aws\.amazon\.com`)

	consoleURL = reConsole.ReplaceAllString(consoleURL, `https://${2}.console.aws.amazon.com`)

	return consoleURL
}

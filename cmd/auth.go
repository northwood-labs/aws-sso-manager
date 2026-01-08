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
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/lithammer/dedent"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

var (
	fBrowser bool

	// authCmd represents the auth command
	authCmd = &cobra.Command{
		Use:   "auth",
		Short: "Authenticates with AWS SSO and retrieves temporary credentials.",
		Long: clihelpers.LongHelpText(`
		Authenticates with AWS SSO and retrieves temporary credentials. This command
		can be used to manually trigger the authentication process and ensure that
		valid credentials are available for AWS CLI and AWS Vault.
		`),
		Args: cobra.RangeArgs(0, 1),
		Example: strings.TrimSpace(dedent.Dedent(`
		aws-sso-vault auth
		aws-sso-vault auth <sso-profile>
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

			// Where does the cache file live?
			cacheFilePath, err := getCacheFilePath(&sessionProfile)
			if err != nil {
				return err
			}

			var cacheData cacheFileData

			// Can we read the cache?
			_, err = cacheData.read(cacheFilePath)
			if err != nil {
				logger.Infof("error reading cache file: %s", err.Error())

				// Generate an AWS SDK config from the SSO session profile.
				sdkConfig, err := getSDKConfig(sessionProfile)
				if err != nil {
					return err
				}

				logger.Infof("Authenticating SSO profile '%s'...", profileName)

				// Perform the SSO authentication flow.
				authURL, registerClient, deviceAuth, err := authenticateSSOProfile(ctx, &sdkConfig, sessionProfile)
				if err != nil {
					return err
				}

				u, err := url.Parse(authURL)
				if err != nil {
					return err
				}

				u, err = url.Parse(u.Fragment)
				if err != nil {
					return err
				}

				fmt.Printf("Ensure the code matches: %s\n", u.Query().Get("user_code"))

				if fBrowser {
					err = browser.OpenURL(authURL)
					cobra.CheckErr(err)
				} else {
					fmt.Println("Confirm: " + authURL + "\n")
				}

				cacheData, err = waitForCustomerToAuthenticate(customerAuthInput{
					ctx:            ctx,
					sdkConfig:      &sdkConfig,
					registerClient: registerClient,
					deviceAuth:     deviceAuth,
					sessionProfile: sessionProfile,
					loginTimeout:   60 * time.Second,
				})
				if err != nil {
					return err
				}

				err = cacheData.save(cacheFilePath)
				if err != nil {
					return err
				}
			}

			fmt.Printf("Successfully authenticated SSO session '%s'.\n", profileName)

			_, err = cacheData.read(cacheFilePath)
			if err != nil {
				return err
			}

			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(authCmd)

	authCmd.Flags().BoolVarP(
		&fBrowser,
		"browser",
		"B",
		true,
		"Open the SSO authentication URL in the default web browser.",
	)
}

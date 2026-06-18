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
	"fmt"
	"net/url"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/lithammer/dedent"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	clihelpers "go.nwlabs.dev/cli-helpers/v2"
)

var (
	fBrowser bool

	// authCmd represents the auth command.
	authCmd = &cobra.Command{
		Use:   "auth [sso-profile-name]",
		Short: "Authenticates with AWS SSO and retrieves temporary credentials.",
		Long: clihelpers.LongHelpText(`
		Authenticates with AWS SSO and retrieves temporary credentials. This command
		can be used to manually trigger the authentication process and ensure that
		valid credentials are available for AWS CLI and AWS Vault.
		`),
		Args:    cobra.RangeArgs(0, 1),
		Aliases: []string{"login"},
		Example: strings.TrimSpace(dedent.Dedent(`
		# Authenticate with the default SSO profile.
		aws-sso-manager auth

		# Authenticate with a specific SSO profile.
		aws-sso-manager auth <sso-profile-name>
		`)),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := ""

			requestCtx := cmd.Context()
			if requestCtx == nil {
				requestCtx = context.Background()
			}

			logger.InfoContext(requestCtx, "Passed arguments", logKeyCount, len(args))

			if len(args) == 1 {
				profileName = args[0]
			} else {
				profileName = asmConfig.GetString("profile-name")
			}

			if profileName == "" {
				err := promptProfileSelect(&profileName)
				if err != nil {
					return fmt.Errorf("could not select SSO profile: %w", err)
				}
			}

			return ensureAuthenticatedSSOSession(requestCtx, profileName)
		},
	}
)

func init() { // lint:allow_init
	rootCmd.AddCommand(authCmd)

	authCmd.Flags().BoolVarP(
		&fBrowser,
		"browser",
		"B",
		true,
		"Open the SSO authentication URL in the default web browser.",
	)
}

func ensureAuthenticatedSSOSession(requestCtx context.Context, profileName string) error {
	logger.InfoContext(requestCtx, "Retrieving SSO session profile", logKeyProfile, profileName)

	// Generate a SSO session profile from the profile name.
	sessionProfile, err := getSsoSession(requestCtx, profileName)
	if err != nil {
		return fmt.Errorf("could not get SSO session: %w", err)
	}

	// Where does the cache file live?
	cacheFilePath, err := getCacheFilePath(requestCtx, &sessionProfile)
	if err != nil {
		return fmt.Errorf("could not get cache file path: %w", err)
	}

	var cacheData cacheFileData

	// Can we read the cache?
	cacheResults, err := cacheData.read(cacheFilePath)
	if err != nil {
		authErr := authenticateAndCacheSSOSession(
			requestCtx,
			profileName,
			sessionProfile,
			cacheFilePath,
		)
		if authErr != nil {
			return fmt.Errorf("could not authenticate SSO session: %w", authErr)
		}
	} else {
		reportValidCachedSSOSession(requestCtx, profileName, cacheFilePath, cacheResults)
	}

	_, err = cacheData.read(cacheFilePath)
	if err != nil {
		return fmt.Errorf("could not read cache after authentication: %w", err)
	}

	return nil
}

func authenticateAndCacheSSOSession(
	requestCtx context.Context,
	profileName string,
	sessionProfile ssoProfile,
	cacheFilePath string,
) error {
	logger.InfoContext(requestCtx, "Authenticating SSO profile", logKeyProfile, profileName)

	// Generate an AWS SDK config from the SSO session profile.
	sdkConfig, err := getSDKConfig(requestCtx, sessionProfile)
	if err != nil {
		return fmt.Errorf("could not get SDK config: %w", err)
	}

	// Perform the SSO authentication flow.
	authURL, registerClient, deviceAuth, err := authenticateSSOProfile(
		requestCtx,
		&sdkConfig,
		sessionProfile,
	)
	if err != nil {
		return fmt.Errorf("could not authenticate SSO profile: %w", err)
	}

	logger.DebugContext(requestCtx, "Authentication URL", logKeyURL, authURL)

	u, err := url.Parse(authURL)
	if err != nil {
		return fmt.Errorf("could not parse auth URL: %w", err)
	}

	u, err = url.Parse(u.Fragment)
	if err != nil {
		return fmt.Errorf("could not parse auth URL fragment: %w", err)
	}

	codeWrapper := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2, 0, 2)

	lipgloss.Println( // lint:allow_unhandled
		codeWrapper.Render(
			lipgloss.Sprintf(
				"Ensure the code matches: %s\n",
				clihelpers.StyleInlineHighlight.Bold(true).Render(u.Query().Get("user_code")),
			),
		),
	)

	if fBrowser {
		err = browser.OpenURL(authURL)
		cobra.CheckErr(err)
	} else {
		fmt.Println("Confirm: " + authURL + "\n")
	}

	cacheData, err := waitForCustomerToAuthenticate(requestCtx, &customerAuthInput{
		sdkConfig:      &sdkConfig,
		registerClient: registerClient,
		deviceAuth:     deviceAuth,
		sessionProfile: sessionProfile,
		loginTimeout:   60 * time.Second, // lint:allow_raw_number
	})
	if err != nil {
		return fmt.Errorf("could not complete authentication: %w", err)
	}

	err = cacheData.save(cacheFilePath)
	if err != nil {
		return fmt.Errorf("could not save cache file: %w", err)
	}

	fmt.Printf("Successfully authenticated SSO session '%s'.\n", profileName)

	return nil
}

func reportValidCachedSSOSession(ctx context.Context, profileName, cacheFilePath string, cacheResults *cacheFileData) {
	logger.InfoContext(ctx, "Cache file is valid; no need to authenticate", logKeyFile, cacheFilePath)

	remaining := time.Until(cacheResults.ExpiresAt)

	fmt.Printf(
		"SSO session '%s' is already authenticated and valid for another %s.\n",
		profileName,
		remaining.Round(time.Second),
	)
}

func getOrRefreshAuthenticatedCache(
	requestCtx context.Context,
	profileName string,
	sessionProfile ssoProfile,
) (*cacheFileData, error) {
	cacheFilePath, err := getCacheFilePath(requestCtx, &sessionProfile)
	if err != nil {
		return nil, fmt.Errorf("could not get cache file path: %w", err)
	}

	var cacheData cacheFileData

	cache, err := cacheData.read(cacheFilePath)
	if err == nil {
		return cache, nil
	}

	logger.InfoContext(
		requestCtx,
		"Session cache is missing or expired; attempting automatic authentication",
		logKeyProfile,
		profileName,
		logKeyErr,
		err,
	)

	authErr := ensureAuthenticatedSSOSession(requestCtx, profileName)
	if authErr != nil {
		return nil, fmt.Errorf("could not ensure authenticated SSO session: %w", authErr)
	}

	cache, err = cacheData.read(cacheFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session cache after authentication: %w", err)
	}

	return cache, nil
}

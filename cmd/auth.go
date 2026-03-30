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
	"context"
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
			var profileName string
			requestCtx := cmd.Context()
			if requestCtx == nil {
				requestCtx = context.Background()
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

			return ensureAuthenticatedSSOSession(requestCtx, profileName)
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

func ensureAuthenticatedSSOSession(requestCtx context.Context, profileName string) error {
	logger.Info("Retrieving SSO session profile", "profile", profileName)

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
	cacheResults, err := cacheData.read(cacheFilePath)
	if err != nil {
		if err := authenticateAndCacheSSOSession(requestCtx, profileName, sessionProfile, cacheFilePath); err != nil {
			return err
		}
	} else {
		reportValidCachedSSOSession(profileName, cacheFilePath, cacheResults)
	}

	_, err = cacheData.read(cacheFilePath)
	if err != nil {
		return err
	}

	return nil
}

func authenticateAndCacheSSOSession(
	requestCtx context.Context,
	profileName string,
	sessionProfile ssoProfile,
	cacheFilePath string,
) error {
	logger.Info("Authenticating SSO profile", "profile", profileName)

	// Generate an AWS SDK config from the SSO session profile.
	sdkConfig, err := getSDKConfig(requestCtx, sessionProfile)
	if err != nil {
		return err
	}

	// Perform the SSO authentication flow.
	authURL, registerClient, deviceAuth, err := authenticateSSOProfile(
		requestCtx,
		&sdkConfig,
		sessionProfile,
	)
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

	cacheData, err := waitForCustomerToAuthenticate(customerAuthInput{
		ctx:            requestCtx,
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

	fmt.Printf("Successfully authenticated SSO session '%s'.\n", profileName)

	return nil
}

func reportValidCachedSSOSession(profileName, cacheFilePath string, cacheResults *cacheFileData) {
	logger.Info("Cache file is valid; no need to authenticate", "file", cacheFilePath)

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
	if requestCtx == nil {
		requestCtx = context.Background()
	}

	cacheFilePath, err := getCacheFilePath(&sessionProfile)
	if err != nil {
		return nil, err
	}

	var cacheData cacheFileData
	cache, err := cacheData.read(cacheFilePath)
	if err == nil {
		return cache, nil
	}

	logger.Info(
		"Session cache is missing or expired; attempting automatic authentication",
		"profile",
		profileName,
		"error",
		err,
	)

	if err := ensureAuthenticatedSSOSession(requestCtx, profileName); err != nil {
		return nil, err
	}

	cache, err = cacheData.read(cacheFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session cache after authentication: %w", err)
	}

	return cache, nil
}

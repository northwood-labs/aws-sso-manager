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
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	clihelpers "go.nwlabs.dev/cli-helpers/v2"
)

// infoCmd represents the console command.
var infoCmd = &cobra.Command{
	Use:   "info [sso-profile-name]",
	Short: "Provides information about the SSO configuration values.",
	Long: clihelpers.LongHelpText(`
	Provides information about the SSO configuration values.
	`),
	Args: cobra.RangeArgs(0, 1),
	Example: strings.TrimSpace(dedent.Dedent(`
	`)),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := ""

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Profile resolution: arg > config > interactive prompt.
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

		logger.InfoContext(ctx, "Retrieving SSO session profile", "profile-name", profileName)

		// Generate a SSO session profile from the profile name.
		sessionProfile, err := getSsoSession(ctx, profileName)
		if err != nil {
			return fmt.Errorf("could not get SSO session: %w", err)
		}

		sdkConfig, err := getSDKConfig(cmd.Context(), sessionProfile)
		if err != nil {
			return fmt.Errorf("could not get AWS SDK config: %w", err)
		}

		cache, err := getOrRefreshAuthenticatedCache(cmd.Context(), profileName, sessionProfile)
		if err != nil {
			return fmt.Errorf("could not ensure authentication for profile %q: %w", profileName, err)
		}

		logger.DebugContext(ctx, "Retrieved SSO session profile", "session-profile", sessionProfile)

		ssoClient := ssoadmin.NewFromConfig(sdkConfig)

		listOut, err := ssoClient.ListInstances(ctx, &ssoadmin.ListInstancesInput{}, func(o *ssoadmin.Options) {
			o.Region = sessionProfile.Region
		})
		if err != nil {
			return fmt.Errorf("could not list SSO instances: %w", err)
		}

		if len(listOut.Instances) == 0 {
			return errors.New("no SSO instances found in this region")
		}

		identityStoreID := aws.ToString(listOut.Instances[0].IdentityStoreId)

		fmt.Println(identityStoreID)

		return nil
	},
}

func init() { // lint:allow_init
	rootCmd.AddCommand(infoCmd)
}

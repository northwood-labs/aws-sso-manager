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
	"strings"

	"github.com/lithammer/dedent"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	clihelpers "go.nwlabs.dev/cli-helpers/v2"
)

// portalCmd represents the console command.
var portalCmd = &cobra.Command{
	Use:   "portal [sso-profile-name]",
	Short: "Opens the AWS Access Portal for an SSO account.",
	Long: clihelpers.LongHelpText(`
	Opens the AWS Access Portal for an SSO account.
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

		// profileName should be defined at this point.
		startHost, err := getStartURL(profileName)
		if err != nil {
			return fmt.Errorf("failed to get start URL; re-init to create profile: %w", err)
		}

		finalURL := fmt.Sprintf("https://%s/start/#/", startHost)

		if fBrowser {
			err = browser.OpenURL(finalURL)
			cobra.CheckErr(err)
		} else {
			fmt.Println(finalURL + "\n")
		}

		return nil
	},
}

func init() { // lint:allow_init
	rootCmd.AddCommand(portalCmd)

	// Variable defined in auth.go
	portalCmd.Flags().BoolVarP(
		&fBrowser,
		"browser",
		"B",
		true,
		"Open the SSO authentication URL in the default web browser.",
	)
}

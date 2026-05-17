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
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// removeFile is a test seam for os.Remove so tests can intercept file
	// deletion without touching the real filesystem.
	removeFile = os.Remove

	// logoutCmd deletes the cached OIDC token for an SSO profile, effectively
	// ending the session. Profile resolution mirrors authCmd: positional arg →
	// Viper config → interactive prompt.
	logoutCmd = &cobra.Command{
		Use:   "logout [sso-profile-name]",
		Short: "Ends an SSO session by removing the cached credentials.",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := ""

			ctx := cmd.Context()

			// Profile resolution: arg > config > interactive prompt.
			if len(args) == 1 {
				profileName = args[0]
			} else {
				profileName = asmConfig.GetString("profile-name")
			}

			if profileName == "" {
				if err := promptProfileSelect(&profileName); err != nil {
					return fmt.Errorf("could not select SSO profile: %w", err)
				}
			}

			logger.InfoContext(ctx, "Logging out SSO session", logKeyProfile, profileName)

			// Resolve the SSO session and cache file path.
			sessionProfile, err := getSsoSession(ctx, profileName)
			if err != nil {
				return fmt.Errorf("could not get SSO session: %w", err)
			}

			cacheFilePath, err := getCacheFilePath(ctx, &sessionProfile)
			if err != nil {
				return fmt.Errorf("could not get cache file path: %w", err)
			}

			// Delete the cache file.
			if err := removeFile(cacheFilePath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					fmt.Printf("No active session found for SSO profile '%s'.\n", profileName)
					return nil
				}

				return fmt.Errorf("failed to remove cache file for profile '%s': %w", profileName, err)
			}

			fmt.Printf("Successfully logged out of SSO profile '%s'.\n", profileName)

			return nil
		},
	}
)

func init() { // lint:allow_init
	rootCmd.AddCommand(logoutCmd)
}

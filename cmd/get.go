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
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

var (
	// getAccountIDPattern enforces the AWS account ID format (exactly 12 digits)
	// so that callers get a clear validation error rather than a confusing
	// "not found" when they pass a malformed ID.
	getAccountIDPattern = regexp.MustCompile(`^\d{12}$`)

	fGetFor     string
	fGetProfile string
	fGetName    bool

	// getCmd is the parent for "get accounts" and "get roles". It exists purely
	// as a namespace — the actual work happens in the subcommands. All output is
	// one-value-per-line so it composes well with Unix pipes and tools like fzf.
	getCmd = &cobra.Command{
		Use:   "get",
		Short: "Returns line-delimited values from local cache for shell piping.",
		Long: clihelpers.LongHelpText(`
		Returns line-delimited values from local cache for shell piping.

		This command reads only from the local lookup cache (or builds it from
		the list cache) and prints one value per line for tools like fzf.
		`),
		Args: cobra.NoArgs,
		Example: strings.TrimSpace(dedent.Dedent(`
		# List all AWS accounts for an SSO profile.
		aws-sso-manager get accounts
		aws-sso-manager get accounts <sso-profile-name> | fzf

		# List all granted roles for an AWS account under an SSO profile.
		aws-sso-manager get roles --for 123456789012
		aws-sso-manager get roles <sso-profile-name> --for 123456789012 | fzf
		`)),
	}

	// getAccountsCmd prints account identifiers from the local lookup cache.
	// By default it prints 12-digit account IDs (sorted numerically) because
	// that's what other commands (get roles --for, console --account-id) expect
	// as input. The --name flag switches to human-readable names for display or
	// interactive selection workflows.
	getAccountsCmd = &cobra.Command{
		Use:   "accounts",
		Short: "Returns linebreak-delimited AWS account IDs (or names with --name).",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, err := resolveGetProfileName()
			if err != nil {
				return fmt.Errorf("could not resolve profile name: %w", err)
			}

			lookupIndex, err := loadOrBuildListAWSAccountsLookupIndex(listAWSAccountsInput{
				Logger:      logger,
				ProfileName: profileName,
			})
			if err != nil {
				return fmt.Errorf("could not load lookup cache: %w", err)
			}

			if fGetName {
				names := getAccountNamesFromLookupIndex(lookupIndex)
				for _, name := range names {
					fmt.Println(name)
				}
			} else {
				accountIDs := getAccountIDsFromLookupIndex(lookupIndex)
				for _, accountID := range accountIDs {
					fmt.Println(accountID)
				}
			}

			return nil
		},
	}

	// getRolesCmd prints role names for a single account. It requires --for
	// because roles are scoped to an account — without it we'd have to dump
	// every role across every account, which is rarely useful for piping.
	getRolesCmd = &cobra.Command{
		Use:   "roles --for [account]",
		Short: "Returns linebreak-delimited role names for one AWS account ID.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(fGetFor) == "" {
				return errors.New("flag --for is required")
			}

			profileName, err := resolveGetProfileName()
			if err != nil {
				return fmt.Errorf("could not resolve profile name: %w", err)
			}

			lookupIndex, err := loadOrBuildListAWSAccountsLookupIndex(listAWSAccountsInput{
				Logger:      logger,
				ProfileName: profileName,
			})
			if err != nil {
				return fmt.Errorf("could not load lookup cache: %w", err)
			}

			roles, err := getRoleNamesForAccountID(lookupIndex, fGetFor)
			if err != nil {
				return fmt.Errorf("could not get roles for account: %w", err)
			}

			for _, roleName := range roles {
				fmt.Println(roleName)
			}

			return nil
		},
	}
)

// resolveGetProfileName determines which SSO profile to use for cache lookups.
// The explicit --profile flag takes priority so that scripts can override the
// default without editing the config file.
func resolveGetProfileName() (string, error) {
	profileName := strings.TrimSpace(fGetProfile)
	if profileName != "" {
		return profileName, nil
	}

	profileName = strings.TrimSpace(asmConfig.GetString("profile-name"))
	if profileName == "" {
		return "", errors.New("no profile configured; set profile-name in config or pass --profile")
	}

	return profileName, nil
}

// getAccountIDsFromLookupIndex extracts all account IDs and returns them in
// sorted order. Sorting numerically (which sort.Strings achieves for
// fixed-width digit strings) gives stable, predictable output for scripts.
func getAccountIDsFromLookupIndex(index listAWSAccountsLookupIndex) []string {
	accountIDs := make([]string, 0, len(index.AccountsByID))
	for accountID := range index.AccountsByID {
		accountIDs = append(accountIDs, accountID)
	}

	sort.Strings(accountIDs)

	return accountIDs
}

// getAccountNamesFromLookupIndex extracts human-readable account names for the
// --name flag. Names are sorted case-insensitively so the output is stable
// regardless of how AWS returns the casing.
func getAccountNamesFromLookupIndex(index listAWSAccountsLookupIndex) []string {
	names := make([]string, 0, len(index.AccountsByID))
	for _, account := range index.AccountsByID {
		if account.Name != "" {
			names = append(names, account.Name)
		}
	}

	sort.SliceStable(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})

	return names
}

// getRoleNamesForAccountID validates the account ID format, looks it up in the
// index, and returns the roles sorted case-insensitively. The format check
// catches typos early — a 12-digit constraint is cheap to enforce and avoids
// confusing "not found" errors when the user accidentally passes a name.
func getRoleNamesForAccountID(index listAWSAccountsLookupIndex, accountID string) ([]string, error) {
	trimmedID := strings.TrimSpace(accountID)
	if !getAccountIDPattern.MatchString(trimmedID) {
		return nil, fmt.Errorf("--for must be a 12-digit AWS account ID, got %q", accountID)
	}

	account, ok := index.AccountsByID[trimmedID]
	if !ok {
		return nil, fmt.Errorf("account ID %q was not found in lookup cache", trimmedID)
	}

	// Copy before sorting so we don't mutate the cached slice.
	roles := append([]string(nil), account.Roles...)
	sort.SliceStable(roles, func(i, j int) bool {
		return strings.ToLower(roles[i]) < strings.ToLower(roles[j])
	})

	return roles, nil
}

func init() {
	rootCmd.AddCommand(getCmd)

	getCmd.PersistentFlags().StringVarP(&fGetProfile, "profile", "p", "", "SSO profile name used for cache lookups")

	getCmd.AddCommand(getAccountsCmd)
	getCmd.AddCommand(getRolesCmd)

	getAccountsCmd.Flags().BoolVarP(&fGetName, "name", "n", false, "print account names instead of account IDs")

	getRolesCmd.Flags().StringVarP(
		&fGetFor,
		"for",
		"f",
		"",
		"12-digit AWS account ID",
	)
}

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
	getAccountIDPattern = regexp.MustCompile(`^\d{12}$`)

	fGetFor     string
	fGetProfile string
	fGetName    bool

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
		aws-sso-manager get accounts
		aws-sso-manager get accounts | fzf
		aws-sso-manager get roles --for 123456789012
		aws-sso-manager get roles --for 123456789012 | fzf
		`)),
	}

	getAccountsCmd = &cobra.Command{
		Use:   "accounts",
		Short: "Returns linebreak-delimited AWS account IDs (or names with --name).",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, err := resolveGetProfileName()
			if err != nil {
				return err
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
				return err
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
				return err
			}

			for _, roleName := range roles {
				fmt.Println(roleName)
			}

			return nil
		},
	}
)

func resolveGetProfileName() (string, error) {
	profileName := strings.TrimSpace(fGetProfile)
	if profileName != "" {
		return profileName, nil
	}

	profileName = strings.TrimSpace(asvConfig.GetString("profile-name"))
	if profileName == "" {
		return "", errors.New("no profile configured; set profile-name in config or pass --profile")
	}

	return profileName, nil
}

func getAccountIDsFromLookupIndex(index listAWSAccountsLookupIndex) []string {
	accountIDs := make([]string, 0, len(index.AccountsByID))
	for accountID := range index.AccountsByID {
		accountIDs = append(accountIDs, accountID)
	}

	sort.Strings(accountIDs)

	return accountIDs
}

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

func getRoleNamesForAccountID(index listAWSAccountsLookupIndex, accountID string) ([]string, error) {
	trimmedID := strings.TrimSpace(accountID)
	if !getAccountIDPattern.MatchString(trimmedID) {
		return nil, fmt.Errorf("--for must be a 12-digit AWS account ID, got %q", accountID)
	}

	account, ok := index.AccountsByID[trimmedID]
	if !ok {
		return nil, fmt.Errorf("account ID %q was not found in lookup cache", trimmedID)
	}

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

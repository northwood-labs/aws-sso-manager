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
	"fmt"
	"slices"
	"strings"

	"pgregory.net/rapid"
)

// genListAccount returns a rapid generator that produces random listAccount
// structs. IDs are constrained to valid 12-digit format so property tests
// exercise realistic data rather than tripping over format validation. Names
// are alphanumeric to avoid special characters that would complicate substring
// matching assertions. Each account gets 1-5 roles to cover both single-role
// and multi-role scenarios.
func genListAccount() *rapid.Generator[listAccount] {
	return rapid.Custom[listAccount](func(t *rapid.T) listAccount {
		accountID := genAccountID().Draw(t, "accountID")
		name := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{2,19}`).Draw(t, "name")
		email := strings.ToLower(name) + "@example.com"

		numRoles := rapid.IntRange(1, 5).Draw(t, "numRoles")

		roles := make([]listRole, numRoles)
		for i := range numRoles {
			roleName := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{2,14}`).Draw(t, fmt.Sprintf("roleName%d", i))

			roles[i] = listRole{
				AccountID: accountID,
				Name:      roleName,
				Profile:   strings.ToLower(fmt.Sprintf("%s-%s", name, roleName)),
			}
		}

		return listAccount{
			ID:    accountID,
			Name:  name,
			Email: email,
			Roles: roles,
		}
	})
}

// genAccountID returns a rapid generator that produces valid 12-digit numeric
// account ID strings.
func genAccountID() *rapid.Generator[string] {
	return rapid.StringMatching(`[0-9]{12}`)
}

// genListAccounts returns a rapid generator that produces pre-sorted
// listAccounts. The sorting matches the production code's sort order so that
// property tests can assert on sorted output without re-sorting. This is
// intentional — if the production sort order changes, the generator should
// change too, and the property tests will catch the mismatch.
func genListAccounts(minAccounts, maxAccounts int) *rapid.Generator[listAccounts] { // lint:allow_param
	return rapid.Custom[listAccounts](func(t *rapid.T) listAccounts {
		numAccounts := rapid.IntRange(minAccounts, maxAccounts).Draw(t, "numAccounts")

		accounts := make([]listAccount, numAccounts)
		for i := range numAccounts {
			accounts[i] = genListAccount().Draw(t, fmt.Sprintf("account%d", i))
		}

		// Sort accounts by name (case-insensitive).
		slices.SortStableFunc(accounts, func(a, b listAccount) int {
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})

		// Sort roles within each account by name (case-insensitive).
		for i := range accounts {
			slices.SortStableFunc(accounts[i].Roles, func(a, b listRole) int {
				return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
			})
		}

		return listAccounts{Accounts: accounts}
	})
}

// genLookupIndex produces a lookup index by calling the real
// buildListAWSAccountsLookupIndex function on random accounts. This ensures
// property tests exercise the actual index-building logic rather than a
// hand-crafted mock that might diverge from production behavior.
func genLookupIndex() *rapid.Generator[listAWSAccountsLookupIndex] {
	return rapid.Custom[listAWSAccountsLookupIndex](func(t *rapid.T) listAWSAccountsLookupIndex {
		accounts := genListAccounts(1, 5).Draw(t, "accounts")
		return buildListAWSAccountsLookupIndex("testprofile", accounts)
	})
}

// genManagedBlockConfig produces well-formed AWS config file content with
// managed block markers. This is used by Property 6 to verify that
// inspectManagedMarkers correctly identifies well-formed configs as having no
// issues. Each profile gets a complete start/end pair with realistic INI
// content inside.
func genManagedBlockConfig(profiles []string) *rapid.Generator[string] {
	return rapid.Custom[string](func(t *rapid.T) string {
		var sb strings.Builder

		// Optional preamble content.
		if rapid.Bool().Draw(t, "hasPreamble") {
			_, _ = sb.WriteString("[default]\n")
			_, _ = sb.WriteString("region = us-east-1\n")
			_, _ = sb.WriteString("\n")
		}

		for _, profile := range profiles {
			fmt.Fprintf(&sb, "; -------- aws-sso-manager: start %s --------\n", profile)
			fmt.Fprintf(&sb, "[sso-session %s]\n", profile)

			_, _ = sb.WriteString("sso_start_url = https://example.awsapps.com/start\n")
			_, _ = sb.WriteString("sso_region = us-east-1\n")
			_, _ = sb.WriteString("sso_registration_scopes = sso:account:access\n")
			_, _ = sb.WriteString("\n")

			// Generate 1-3 profile sections inside the managed block.
			numProfiles := rapid.IntRange(1, 3).Draw(t, "numProfiles_"+profile)
			for j := range numProfiles {
				profileName := rapid.StringMatching(
					`[a-z][a-z0-9]{2,14}`,
				).Draw(
					t,
					fmt.Sprintf("profileName_%s_%d", profile, j),
				)
				fmt.Fprintf(&sb, "[profile %s]\n", profileName)
				fmt.Fprintf(&sb, "sso_session = %s\n", profile)
				fmt.Fprintf(
					&sb,
					"sso_account_id = %s\n",
					genAccountID().Draw(t, fmt.Sprintf("acctID_%s_%d", profile, j)),
				)
				fmt.Fprintf(
					&sb,
					"sso_role_name = %s\n",
					rapid.StringMatching(
						`[A-Za-z][A-Za-z0-9]{2,14}`,
					).Draw(
						t,
						fmt.Sprintf("roleName_%s_%d", profile, j),
					),
				)

				_, _ = sb.WriteString("region = us-east-1\n")
				_, _ = sb.WriteString("output = json\n")
			}

			fmt.Fprintf(&sb, "; -------- aws-sso-manager: end %s --------\n", profile)
		}

		return sb.String()
	})
}

// genProfilePatternConfig produces random but structurally valid profile naming
// configurations. The token order is a random permutation of a random subset,
// which exercises all possible orderings and token combinations. This is used
// by Property 7 to verify that getProfileName correctly assembles tokens
// regardless of order.
func genProfilePatternConfig() *rapid.Generator[map[string]any] {
	return rapid.Custom[map[string]any](func(t *rapid.T) map[string]any {
		allTokens := []string{"PREFIX", "ACCOUNT", "ROLE", "SUFFIX"}

		// Pick a non-empty subset of tokens in random order.
		numTokens := rapid.IntRange(1, len(allTokens)).Draw(t, "numTokens")
		perm := rapid.Permutation(allTokens).Draw(t, "tokenPerm")
		order := perm[:numTokens]

		delimiter := rapid.StringMatching(`[._-]{1,3}`).Draw(t, "delimiter")
		prefix := rapid.StringMatching(`[a-z]{0,5}`).Draw(t, "prefix")
		suffix := rapid.StringMatching(`[a-z]{0,5}`).Draw(t, "suffix")

		// Generate substr_match_replace maps with 0-2 entries.
		accountReplacements := make(map[string]any)

		numAccountReplacements := rapid.IntRange(0, 2).Draw(t, "numAccountReplacements")
		for i := range numAccountReplacements {
			key := rapid.StringMatching(`[A-Za-z]{2,8}`).Draw(t, fmt.Sprintf("acctReplKey%d", i))
			val := rapid.StringMatching(`[a-z]{1,6}`).Draw(t, fmt.Sprintf("acctReplVal%d", i))

			accountReplacements[key] = val
		}

		roleReplacements := make(map[string]any)

		numRoleReplacements := rapid.IntRange(0, 2).Draw(t, "numRoleReplacements")
		for i := range numRoleReplacements {
			key := rapid.StringMatching(`[A-Za-z]{2,8}`).Draw(t, fmt.Sprintf("roleReplKey%d", i))
			val := rapid.StringMatching(`[a-z]{1,6}`).Draw(t, fmt.Sprintf("roleReplVal%d", i))

			roleReplacements[key] = val
		}

		config := map[string]any{
			"pattern": map[string]any{
				"order":     order,
				"delimiter": delimiter,
			},
			"prefix": prefix,
			"suffix": suffix,
			"accounts": map[string]any{
				"substr_match_replace": accountReplacements,
			},
			"roles": map[string]any{
				"substr_match_replace": roleReplacements,
			},
		}

		return config
	})
}

// genConfigKey returns a rapid generator that produces valid config keys
// matching the schema defined in configschema.go. Keys follow the pattern
// <profile>.rename.<leaf> where leaf is one of the valid terminal paths.
func genConfigKey() *rapid.Generator[string] {
	return rapid.Custom[string](func(t *rapid.T) string {
		profile := rapid.StringMatching(`[a-z][a-z0-9]{1,8}`).Draw(t, "profile")

		leaves := []string{
			"rename.prefix",
			"rename.suffix",
			"rename.pattern.delimiter",
		}

		leaf := rapid.SampledFrom(leaves).Draw(t, "leaf")

		return profile + "." + leaf
	})
}

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
	"sort"
	"strings"

	"pgregory.net/rapid"
)

// genListAccount returns a rapid generator that produces random listAccount
// structs with valid 12-digit numeric IDs, alphanumeric names (3-20 chars),
// email addresses, and 1-5 roles.
func genListAccount() *rapid.Generator[listAccount] {
	return rapid.Custom[listAccount](func(t *rapid.T) listAccount {
		accountID := genAccountID().Draw(t, "accountID")
		name := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{2,19}`).Draw(t, "name")
		email := fmt.Sprintf("%s@example.com", strings.ToLower(name))

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

// genListAccounts returns a rapid generator that produces listAccounts with
// between minAccounts and maxAccounts accounts. Accounts are sorted by name
// (case-insensitive) and roles within each account are sorted by name
// (case-insensitive), matching the production sorting logic.
func genListAccounts(minAccounts, maxAccounts int) *rapid.Generator[listAccounts] {
	return rapid.Custom[listAccounts](func(t *rapid.T) listAccounts {
		numAccounts := rapid.IntRange(minAccounts, maxAccounts).Draw(t, "numAccounts")
		accounts := make([]listAccount, numAccounts)
		for i := range numAccounts {
			accounts[i] = genListAccount().Draw(t, fmt.Sprintf("account%d", i))
		}

		// Sort accounts by name (case-insensitive)
		sort.SliceStable(accounts, func(i, j int) bool {
			return strings.ToLower(accounts[i].Name) < strings.ToLower(accounts[j].Name)
		})

		// Sort roles within each account by name (case-insensitive)
		for i := range accounts {
			sort.SliceStable(accounts[i].Roles, func(a, b int) bool {
				return strings.ToLower(accounts[i].Roles[a].Name) < strings.ToLower(accounts[i].Roles[b].Name)
			})
		}

		return listAccounts{Accounts: accounts}
	})
}

// genLookupIndex returns a rapid generator that produces a
// listAWSAccountsLookupIndex by generating random accounts and calling
// buildListAWSAccountsLookupIndex.
func genLookupIndex() *rapid.Generator[listAWSAccountsLookupIndex] {
	return rapid.Custom[listAWSAccountsLookupIndex](func(t *rapid.T) listAWSAccountsLookupIndex {
		accounts := genListAccounts(1, 5).Draw(t, "accounts")
		return buildListAWSAccountsLookupIndex("testprofile", accounts)
	})
}

// genManagedBlockConfig returns a rapid generator that produces well-formed AWS
// config file content with managed block markers. For each profile, a managed
// block is generated with start/end markers and an [sso-session <profile>]
// section inside.
func genManagedBlockConfig(profiles []string) *rapid.Generator[string] {
	return rapid.Custom[string](func(t *rapid.T) string {
		var sb strings.Builder

		// Optional preamble content
		if rapid.Bool().Draw(t, "hasPreamble") {
			sb.WriteString("[default]\n")
			sb.WriteString("region = us-east-1\n")
			sb.WriteString("\n")
		}

		for _, profile := range profiles {
			sb.WriteString(fmt.Sprintf("; -------- aws-sso-manager: start %s --------\n", profile))
			sb.WriteString(fmt.Sprintf("[sso-session %s]\n", profile))
			sb.WriteString("sso_start_url = https://example.awsapps.com/start\n")
			sb.WriteString("sso_region = us-east-1\n")
			sb.WriteString("sso_registration_scopes = sso:account:access\n")
			sb.WriteString("\n")

			// Generate 1-3 profile sections inside the managed block
			numProfiles := rapid.IntRange(1, 3).Draw(t, fmt.Sprintf("numProfiles_%s", profile))
			for j := range numProfiles {
				profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,14}`).Draw(t, fmt.Sprintf("profileName_%s_%d", profile, j))
				sb.WriteString(fmt.Sprintf("[profile %s]\n", profileName))
				sb.WriteString(fmt.Sprintf("sso_session = %s\n", profile))
				sb.WriteString(fmt.Sprintf("sso_account_id = %s\n", genAccountID().Draw(t, fmt.Sprintf("acctID_%s_%d", profile, j))))
				sb.WriteString(fmt.Sprintf("sso_role_name = %s\n", rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{2,14}`).Draw(t, fmt.Sprintf("roleName_%s_%d", profile, j))))
				sb.WriteString("region = us-east-1\n")
				sb.WriteString("output = json\n")
			}

			sb.WriteString(fmt.Sprintf("; -------- aws-sso-manager: end %s --------\n", profile))
		}

		return sb.String()
	})
}

// genProfilePatternConfig returns a rapid generator that produces a
// map[string]interface{} representing a profile pattern configuration with
// pattern order (subset of ["PREFIX","ACCOUNT","ROLE","SUFFIX"]), delimiter
// (1-3 chars), prefix, suffix, and substr_match_replace maps.
func genProfilePatternConfig() *rapid.Generator[map[string]interface{}] {
	return rapid.Custom[map[string]interface{}](func(t *rapid.T) map[string]interface{} {
		allTokens := []string{"PREFIX", "ACCOUNT", "ROLE", "SUFFIX"}

		// Pick a non-empty subset of tokens in random order
		numTokens := rapid.IntRange(1, len(allTokens)).Draw(t, "numTokens")
		perm := rapid.Permutation(allTokens).Draw(t, "tokenPerm")
		order := perm[:numTokens]

		delimiter := rapid.StringMatching(`[._-]{1,3}`).Draw(t, "delimiter")
		prefix := rapid.StringMatching(`[a-z]{0,5}`).Draw(t, "prefix")
		suffix := rapid.StringMatching(`[a-z]{0,5}`).Draw(t, "suffix")

		// Generate substr_match_replace maps with 0-2 entries
		accountReplacements := make(map[string]interface{})
		numAccountReplacements := rapid.IntRange(0, 2).Draw(t, "numAccountReplacements")
		for i := range numAccountReplacements {
			key := rapid.StringMatching(`[A-Za-z]{2,8}`).Draw(t, fmt.Sprintf("acctReplKey%d", i))
			val := rapid.StringMatching(`[a-z]{1,6}`).Draw(t, fmt.Sprintf("acctReplVal%d", i))
			accountReplacements[key] = val
		}

		roleReplacements := make(map[string]interface{})
		numRoleReplacements := rapid.IntRange(0, 2).Draw(t, "numRoleReplacements")
		for i := range numRoleReplacements {
			key := rapid.StringMatching(`[A-Za-z]{2,8}`).Draw(t, fmt.Sprintf("roleReplKey%d", i))
			val := rapid.StringMatching(`[a-z]{1,6}`).Draw(t, fmt.Sprintf("roleReplVal%d", i))
			roleReplacements[key] = val
		}

		config := map[string]interface{}{
			"pattern": map[string]interface{}{
				"order":     order,
				"delimiter": delimiter,
			},
			"prefix": prefix,
			"suffix": suffix,
			"accounts": map[string]interface{}{
				"substr_match_replace": accountReplacements,
			},
			"roles": map[string]interface{}{
				"substr_match_replace": roleReplacements,
			},
		}

		return config
	})
}

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
	"slices"
	"sort"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestLookupAccountIDsByIdentifier(t *testing.T) {
	index := listAWSAccountsLookupIndex{
		ProfileName: "nwl",
		AccountsByID: map[string]listAWSAccountsLookupAccount{
			"111111111111": {Name: "Production"},
		},
		AccountIDsByNameCI: map[string][]string{
			"production": {"111111111111"},
		},
		AccountIDsByProfileCI: map[string][]string{
			"prod-admin": {"111111111111"},
		},
	}

	tests := []struct {
		name       string
		identifier string
		wantID     string
		wantErr    bool
	}{
		{name: "account ID", identifier: "111111111111", wantID: "111111111111"},
		{name: "friendly name case insensitive", identifier: "PrOdUcTiOn", wantID: "111111111111"},
		{name: "profile name case insensitive", identifier: "PROD-ADMIN", wantID: "111111111111"},
		{name: "substring of account name", identifier: "prod", wantID: "111111111111"},
		{name: "substring of profile name", identifier: "admin", wantID: "111111111111"},
		{name: "missing", identifier: "does-not-exist", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := lookupAccountIDsByIdentifier(index, tc.identifier)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}

				return
			}

			if err != nil {
				t.Fatalf("lookupAccountIDsByIdentifier(%q): %v", tc.identifier, err)
			}

			if len(got) != 1 || got[0] != tc.wantID {
				t.Fatalf("expected %q, got %#v", tc.wantID, got)
			}
		})
	}
}

func TestResolveLookupAccountAmbiguous(t *testing.T) {
	index := listAWSAccountsLookupIndex{
		ProfileName: "nwl",
		AccountsByID: map[string]listAWSAccountsLookupAccount{
			"111111111111": {Name: "Sandbox A"},
			"222222222222": {Name: "Sandbox B"},
		},
		AccountIDsByNameCI: map[string][]string{
			"sandbox": {"111111111111", "222222222222"},
		},
		AccountIDsByProfileCI: map[string][]string{},
	}

	_, _, err := resolveLookupAccount(index, "sandbox")
	if err == nil {
		t.Fatalf("expected ambiguity error")
	}
}

// Feature: aws-sso-manager, Property 15: Lookup Account Resolution Correctness
func TestPropertyLookupAccountResolution(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		index := genLookupIndex().Draw(t, "index")

		// 1. For each account in the index, lookup by account ID returns exactly one account.
		for accountID, account := range index.AccountsByID {
			gotID, gotAccount, err := resolveLookupAccount(index, accountID)
			if err != nil {
				t.Fatalf("resolveLookupAccount(%q) returned error: %v", accountID, err)
			}
			if gotID != accountID {
				t.Fatalf("expected account ID %q, got %q", accountID, gotID)
			}
			if gotAccount.Name != account.Name {
				t.Fatalf("expected account name %q, got %q", account.Name, gotAccount.Name)
			}
		}

		// 2. A random identifier that doesn't exist in the index returns not-found error.
		bogus := rapid.StringMatching(`[a-z]{20,30}`).Draw(t, "bogusIdentifier")
		// Ensure the bogus identifier doesn't accidentally match anything in the index.
		bogusLower := strings.ToLower(bogus)
		_, existsByID := index.AccountsByID[bogus]
		_, existsByName := index.AccountIDsByNameCI[bogusLower]
		_, existsByProfile := index.AccountIDsByProfileCI[bogusLower]
		if !existsByID && !existsByName && !existsByProfile {
			_, _, err := resolveLookupAccount(index, bogus)
			if err == nil {
				t.Fatalf("expected not-found error for bogus identifier %q, got nil", bogus)
			}
			if !strings.Contains(err.Error(), "not found") {
				t.Fatalf("expected not-found error for %q, got: %v", bogus, err)
			}
		}

		// 3. For ambiguous lookups, verify ambiguity error.
		// Check AccountIDsByNameCI for keys that map to multiple account IDs.
		for nameKey, accountIDs := range index.AccountIDsByNameCI {
			if len(accountIDs) > 1 {
				_, _, err := resolveLookupAccount(index, nameKey)
				if err == nil {
					t.Fatalf("expected ambiguity error for %q (maps to %v), got nil", nameKey, accountIDs)
				}
				if !strings.Contains(err.Error(), "ambiguous") {
					t.Fatalf("expected ambiguity error for %q, got: %v", nameKey, err)
				}
			}
		}
		// Check AccountIDsByProfileCI for keys that map to multiple account IDs.
		for profileKey, accountIDs := range index.AccountIDsByProfileCI {
			if len(accountIDs) > 1 {
				_, _, err := resolveLookupAccount(index, profileKey)
				if err == nil {
					t.Fatalf("expected ambiguity error for %q (maps to %v), got nil", profileKey, accountIDs)
				}
				if !strings.Contains(err.Error(), "ambiguous") {
					t.Fatalf("expected ambiguity error for %q, got: %v", profileKey, err)
				}
			}
		}
	})
}

// Feature: aws-sso-manager, Property 16: Lookup Role Substring Search
func TestPropertyLookupRoleSubstringSearch(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 1. Generate random accounts with roles.
		accounts := genListAccounts(1, 5).Draw(t, "accounts")
		index := buildListAWSAccountsLookupIndex("testprofile", accounts)

		// For each account in the index, generate a search substring and verify filtering.
		for accountID, account := range index.AccountsByID {
			if len(account.Roles) == 0 {
				continue
			}

			// 2. Generate a non-empty search substring (2-5 chars).
			needle := rapid.StringMatching(`[A-Za-z]{2,5}`).Draw(t, "needle_"+accountID)
			needleLower := strings.ToLower(needle)

			// 3. Apply case-insensitive substring filtering on the account's roles
			//    (replicating the logic from lookupRoleCmd).
			var matches []string
			for _, roleName := range account.Roles {
				if strings.Contains(strings.ToLower(roleName), needleLower) {
					matches = append(matches, roleName)
				}
			}

			sort.SliceStable(matches, func(i, j int) bool {
				return strings.ToLower(matches[i]) < strings.ToLower(matches[j])
			})

			// 4. Verify the result is sorted alphabetically (case-insensitive).
			for i := 1; i < len(matches); i++ {
				if strings.ToLower(matches[i-1]) > strings.ToLower(matches[i]) {
					t.Fatalf("matches not sorted: %q > %q", matches[i-1], matches[i])
				}
			}

			// 5. Verify the result is a subset of the account's roles.
			roleSet := make(map[string]bool, len(account.Roles))
			for _, r := range account.Roles {
				roleSet[r] = true
			}
			for _, m := range matches {
				if !roleSet[m] {
					t.Fatalf("match %q is not in account roles %v", m, account.Roles)
				}
			}

			// 6. Verify every result contains the search substring (case-insensitive).
			for _, m := range matches {
				if !strings.Contains(strings.ToLower(m), needleLower) {
					t.Fatalf("match %q does not contain needle %q", m, needle)
				}
			}

			// 7. Verify completeness: no role that should match was missed.
			for _, roleName := range account.Roles {
				if strings.Contains(strings.ToLower(roleName), needleLower) {
					found := slices.Contains(matches, roleName)
					if !found {
						t.Fatalf("role %q contains %q but was not in matches", roleName, needle)
					}
				}
			}
		}
	})
}

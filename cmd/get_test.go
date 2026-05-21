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
	"reflect"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestGetAccountIDsFromLookupIndexSorted(t *testing.T) {
	index := listAWSAccountsLookupIndex{
		AccountsByID: map[string]listAWSAccountsLookupAccount{
			"222222222222": {Name: "Production"},
			testAccountID:  {Name: "Sandbox"},
		},
	}

	got := getAccountIDsFromLookupIndex(index)
	want := []string{testAccountID, "222222222222"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestGetRoleNamesForAccountID(t *testing.T) {
	index := listAWSAccountsLookupIndex{
		AccountsByID: map[string]listAWSAccountsLookupAccount{
			testAccountID: {
				Name:  "Production",
				Roles: []string{"Viewer", "Admin", "billing"},
			},
		},
	}

	tests := []struct {
		name      string
		accountID string
		want      []string
		wantErr   bool
	}{
		{
			name:      "valid account id returns sorted roles",
			accountID: testAccountID,
			want:      []string{"Admin", "billing", "Viewer"},
		},
		{
			name:      "invalid account id format",
			accountID: "abc",
			wantErr:   true,
		},
		{
			name:      "missing account id",
			accountID: "333333333333",
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getRoleNamesForAccountID(index, tc.accountID)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}

				return
			}

			if err != nil {
				t.Fatalf("getRoleNamesForAccountID(%q): %v", tc.accountID, err)
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expected %#v, got %#v", tc.want, got)
			}
		})
	}
}

// Feature: aws-sso-manager, Property 14: Account ID Validation
// **Validates: Requirements 6.3**.
func TestPropertyAccountIDValidation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Build a lookup index with known accounts.
		index := genLookupIndex().Draw(t, "index")

		// --- Part 1: Invalid account IDs must return an error ---
		// Generate a string that is NOT a valid 12-digit numeric string.
		invalidKind := rapid.IntRange(0, 4).Draw(t, "invalidKind")

		var invalidID string

		switch invalidKind {
		case 0:
			// Empty string.
			invalidID = ""
		case 1:
			// Too short: 1-11 digits.
			length := rapid.IntRange(1, 11).Draw(t, "shortLen")

			invalidID = rapid.StringMatching(fmt.Sprintf(`[0-9]{%d}`, length)).Draw(t, "shortID")
		case 2:
			// Too long: 13-20 digits.
			length := rapid.IntRange(13, 20).Draw(t, "longLen")

			invalidID = rapid.StringMatching(fmt.Sprintf(`[0-9]{%d}`, length)).Draw(t, "longID")
		case 3:
			// Contains letters (12 chars but not all digits).
			invalidID = rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{11}`).Draw(t, "alphaID")
		case 4:
			// Has whitespace or special characters.
			invalidID = rapid.StringMatching(`[!@#$%^& ]{1,12}`).Draw(t, "specialID")
		default:
		}

		_, err := getRoleNamesForAccountID(index, invalidID)
		if err == nil {
			t.Fatalf("expected error for invalid account ID %q, got nil", invalidID)
		}

		// --- Part 2: Valid 12-digit IDs present in the index must return roles ---.
		for accountID, account := range index.AccountsByID {
			roles, err := getRoleNamesForAccountID(index, accountID)
			if err != nil {
				t.Fatalf("unexpected error for valid account ID %q: %v", accountID, err)
			}

			// Roles should be sorted case-insensitively.
			if !slices.IsSortedFunc(roles, func(a, b string) int {
				return strings.Compare(strings.ToLower(a), strings.ToLower(b))
			}) {
				t.Fatalf("roles for account %q are not sorted: %v", accountID, roles)
			}

			// Roles should match the account's roles (sorted).
			expectedRoles := append([]string(nil), account.Roles...)
			slices.SortStableFunc(expectedRoles, func(a, b string) int {
				return strings.Compare(strings.ToLower(a), strings.ToLower(b))
			})

			if !reflect.DeepEqual(roles, expectedRoles) {
				t.Fatalf("roles mismatch for account %q: got %v, want %v", accountID, roles, expectedRoles)
			}
		}
	})
}

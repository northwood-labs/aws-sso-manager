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
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestStripAccountFromURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "URL with 12-digit account subdomain",
			input:    "https://123456789012.s3.console.aws.amazon.com/s3/home",
			expected: "https://s3.console.aws.amazon.com/s3/home",
		},
		{
			name:     "URL with alphanumeric account subdomain",
			input:    "https://myaccount.ec2.console.aws.amazon.com/ec2",
			expected: "https://ec2.console.aws.amazon.com/ec2",
		},
		{
			name:     "URL without account subdomain",
			input:    "https://s3.console.aws.amazon.com/s3/home",
			expected: "https://s3.console.aws.amazon.com/s3/home",
		},
		{
			name:     "non-console URL",
			input:    "https://example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripAccountFromURL(tc.input)
			if got != tc.expected {
				t.Errorf("stripAccountFromURL(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestMinMaxRows(t *testing.T) {
	tests := []struct {
		name     string
		items    []int
		expected int
	}{
		{
			name:     "empty slice returns 5",
			items:    []int{},
			expected: 5,
		},
		{
			name:     "3 items returns 5 (min is 5)",
			items:    make([]int, 3),
			expected: 5,
		},
		{
			name:     "7 items returns 7",
			items:    make([]int, 7),
			expected: 7,
		},
		{
			name:     "15 items returns 10 (max is 10)",
			items:    make([]int, 15),
			expected: 10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := minMaxRows(tc.items)
			if got != tc.expected {
				t.Errorf("minMaxRows(%d items) = %d, want %d", len(tc.items), got, tc.expected)
			}
		})
	}
}

func TestGetRolesForAccount(t *testing.T) {
	accounts := []listAccount{
		{
			ID:   "111111111111",
			Name: "Dev",
			Roles: []listRole{
				{AccountID: "111111111111", Name: "Admin", Profile: "dev-admin"},
				{AccountID: "111111111111", Name: "ReadOnly", Profile: "dev-readonly"},
			},
		},
		{
			ID:   "222222222222",
			Name: "Prod",
			Roles: []listRole{
				{AccountID: "222222222222", Name: "Admin", Profile: "prod-admin"},
			},
		},
	}

	tests := []struct {
		name      string
		accountID string
		wantCount int
	}{
		{
			name:      "matching account returns roles",
			accountID: "111111111111",
			wantCount: 2,
		},
		{
			name:      "missing account returns empty slice",
			accountID: "999999999999",
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			roles := getRolesForAccount(accounts, tc.accountID)
			if len(roles) != tc.wantCount {
				t.Errorf("getRolesForAccount(%q) returned %d roles, want %d", tc.accountID, len(roles), tc.wantCount)
			}
		})
	}
}

// Feature: aws-sso-manager, Property 13: Console URL Account Subdomain Stripping
func TestPropertyConsoleURLAccountSubdomainStripping(t *testing.T) {
	// **Validates: Requirements 5.9**
	t.Run("strips_account_subdomain", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			account := rapid.StringMatching(`[0-9]{12}`).Draw(t, "account")
			service := rapid.StringMatching(`[a-z][a-z0-9-]{1,10}`).Draw(t, "service")
			path := rapid.StringMatching(`/[a-z0-9/]{1,20}`).Draw(t, "path")

			input := "https://" + account + "." + service + ".console.aws.amazon.com" + path
			got := stripAccountFromURL(input)

			expected := "https://" + service + ".console.aws.amazon.com" + path
			if got != expected {
				t.Fatalf("expected %q, got %q", expected, got)
			}
		})
	})

	t.Run("no_account_subdomain_unchanged", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			service := rapid.StringMatching(`[a-z][a-z0-9-]{1,10}`).Draw(t, "service")
			path := rapid.StringMatching(`/[a-z0-9/]{1,20}`).Draw(t, "path")

			input := "https://" + service + ".console.aws.amazon.com" + path
			got := stripAccountFromURL(input)

			if !strings.Contains(got, service+".console.aws.amazon.com") {
				t.Fatalf("expected service subdomain to be preserved, got %q from %q", got, input)
			}

			if got != input {
				t.Fatalf("expected URL to pass through unchanged, got %q from %q", got, input)
			}
		})
	})
}

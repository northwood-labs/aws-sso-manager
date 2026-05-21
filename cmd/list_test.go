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
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestBuildListOutputRows(t *testing.T) {
	accounts := listAccounts{
		Accounts: []listAccount{
			{
				ID:   "111111111111",
				Name: "Production",
				Roles: []listRole{
					{Name: "Admin"},
					{Name: "ReadOnly"},
				},
			},
		},
	}

	got := buildListOutputRows("nwl", accounts)
	want := [][]string{
		{"111111111111", "Production", "Admin", "production-admin"},
		{"111111111111", "Production", "ReadOnly", "production-readonly"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestRenderCSVTable(t *testing.T) {
	headers := []string{"ID", "Account Name", "Role Name", "Profile Name"}
	rows := [][]string{
		{"111111111111", "Prod, Main", `Admin "Power"`, "nwl-prod-admin"},
	}

	got := renderCSVTable(headers, rows)
	want := "\"ID\",\"Account Name\",\"Role Name\",\"Profile Name\"\n" +
		"\"111111111111\",\"Prod, Main\",\"Admin \"\"Power\"\"\",\"nwl-prod-admin\"\n"

	if got != want {
		t.Fatalf("expected:\n%s\ngot:\n%s", want, got)
	}
}

func TestRenderMarkdownTable(t *testing.T) {
	headers := []string{"ID", "Account Name", "Role Name", "Profile Name"}
	rows := [][]string{
		{"111111111111", "Production", "Admin", "nwl-production-admin"},
		{"222222222222", "Sandbox", "ReadOnly", "nwl-sandbox-readonly"},
	}

	got := renderMarkdownTable(headers, rows)
	want := "| ID           | Account Name | Role Name | Profile Name         |\n" +
		"| ------------ | ------------ | --------- | -------------------- |\n" +
		"| 111111111111 | Production   | Admin     | nwl-production-admin |\n" +
		"| 222222222222 | Sandbox      | ReadOnly  | nwl-sandbox-readonly |\n"

	if got != want {
		t.Fatalf("expected:\n%s\ngot:\n%s", want, got)
	}
}

// Feature: aws-sso-manager, Property 3: Output Formats Contain All Data.
func TestPropertyOutputFormatsContainAllData(t *testing.T) { // lint:allow_complexity
	// **Validates: Requirements 3.9, 3.10, 3.11**.
	rapid.Check(t, func(t *rapid.T) {
		profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,9}`).Draw(t, "profileName")
		accounts := genListAccounts(1, 5).Draw(t, "accounts")

		rows := buildListOutputRows(profileName, accounts)

		csvOutput := renderCSVTable(listOutputHeaders, rows)
		mdOutput := renderMarkdownTable(listOutputHeaders, rows)

		jsonBytes, err := json.Marshal(accounts)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}

		jsonOutput := string(jsonBytes)

		// Verify every account ID, account name, and role name appears in all three outputs.
		for _, acct := range accounts.Accounts {
			if !strings.Contains(csvOutput, acct.ID) {
				t.Fatalf("CSV output missing account ID %q", acct.ID)
			}

			if !strings.Contains(mdOutput, acct.ID) {
				t.Fatalf("Markdown output missing account ID %q", acct.ID)
			}

			if !strings.Contains(jsonOutput, acct.ID) {
				t.Fatalf("JSON output missing account ID %q", acct.ID)
			}

			if !strings.Contains(csvOutput, acct.Name) {
				t.Fatalf("CSV output missing account name %q", acct.Name)
			}

			if !strings.Contains(mdOutput, acct.Name) {
				t.Fatalf("Markdown output missing account name %q", acct.Name)
			}

			if !strings.Contains(jsonOutput, acct.Name) {
				t.Fatalf("JSON output missing account name %q", acct.Name)
			}

			for _, role := range acct.Roles {
				if !strings.Contains(csvOutput, role.Name) {
					t.Fatalf("CSV output missing role name %q", role.Name)
				}

				if !strings.Contains(mdOutput, role.Name) {
					t.Fatalf("Markdown output missing role name %q", role.Name)
				}

				if !strings.Contains(jsonOutput, role.Name) {
					t.Fatalf("JSON output missing role name %q", role.Name)
				}
			}
		}

		// Verify every profile name from the rows appears in CSV and markdown output.
		for _, row := range rows {
			profile := row[3] // Profile Name is the 4th column.
			if !strings.Contains(csvOutput, profile) {
				t.Fatalf("CSV output missing profile name %q", profile)
			}

			if !strings.Contains(mdOutput, profile) {
				t.Fatalf("Markdown output missing profile name %q", profile)
			}
		}
	})
}

func TestQuoteCSVCellEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty string", input: "", want: `""`},
		{name: "string with commas", input: "hello, world", want: `"hello, world"`},
		{name: "string with double quotes", input: `say "hello"`, want: `"say ""hello"""`},
		{name: "string with newlines", input: "line1\nline2", want: "\"line1\nline2\""},
		{name: "plain string", input: "plain", want: `"plain"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := quoteCSVCell(tc.input)
			if got != tc.want {
				t.Errorf("quoteCSVCell(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

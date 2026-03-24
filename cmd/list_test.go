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
	"reflect"
	"testing"
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
		{"111111111111", "Production", "Admin", "nwl-production-admin"},
		{"111111111111", "Production", "ReadOnly", "nwl-production-readonly"},
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

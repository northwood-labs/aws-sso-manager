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

func TestGetAccountIDsFromLookupIndexSorted(t *testing.T) {
	index := listAWSAccountsLookupIndex{
		AccountsByID: map[string]listAWSAccountsLookupAccount{
			"222222222222": {Name: "Production"},
			"111111111111": {Name: "Sandbox"},
		},
	}

	got := getAccountIDsFromLookupIndex(index)
	want := []string{"111111111111", "222222222222"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestGetRoleNamesForAccountID(t *testing.T) {
	index := listAWSAccountsLookupIndex{
		AccountsByID: map[string]listAWSAccountsLookupAccount{
			"111111111111": {
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
			accountID: "111111111111",
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
					t.Fatalf("expected error")
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

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

import "testing"

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

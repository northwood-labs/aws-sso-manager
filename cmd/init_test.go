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

func TestNormalizeSSOStartURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "full URL unchanged",
			input: "https://northwood-labs.awsapps.com/start",
			want:  "https://northwood-labs.awsapps.com/start",
		},
		{
			name:  "subdomain becomes full URL",
			input: "northwood-labs",
			want:  "https://northwood-labs.awsapps.com/start",
		},
		{
			name:  "awsapps host without scheme gains https and start",
			input: "northwood-labs.awsapps.com",
			want:  "https://northwood-labs.awsapps.com/start",
		},
		{
			name:  "host and path without scheme gains https",
			input: "northwood-labs.awsapps.com/start",
			want:  "https://northwood-labs.awsapps.com/start",
		},
		{
			name:    "empty rejected",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "malformed full URL rejected",
			input:   "https:///start",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeSSOStartURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("normalizeSSOStartURL(%q): %v", tc.input, err)
			}

			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

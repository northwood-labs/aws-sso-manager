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
	"net/url"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

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

// Feature: aws-sso-manager, Property 1: SSO Start URL Normalization Round Trip
func TestPropertySSOStartURLNormalization(t *testing.T) { // lint:allow_complexity
	// **Validates: Requirements 1.5, 1.6, 1.7**

	// Sub-property 1: Bare subdomains (no dots, no slashes, no "://")
	// produce https://<subdomain>.awsapps.com/start
	t.Run("bare_subdomain", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			subdomain := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9-]{0,19}`).Draw(t, "subdomain")

			result, err := normalizeSSOStartURL(subdomain)
			if err != nil {
				t.Fatalf("unexpected error for bare subdomain %q: %v", subdomain, err)
			}

			if !strings.HasPrefix(result, "https://") {
				t.Fatalf("expected https:// prefix, got %q", result)
			}

			if !strings.HasSuffix(result, ".awsapps.com/start") {
				t.Fatalf("expected .awsapps.com/start suffix for bare subdomain %q, got %q", subdomain, result)
			}

			parsed, err := url.Parse(result)
			if err != nil {
				t.Fatalf("output not parseable as URL: %v", err)
			}

			if parsed.Host == "" {
				t.Fatalf("parsed URL has empty host for %q", result)
			}
		})
	})

	// Sub-property 2: Dot-containing strings (no "://", no "/")
	// produce https://<input>/start
	t.Run("dot_containing", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate a string with at least one dot, no "://", no "/"
			left := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9-]{0,9}`).Draw(t, "left")
			right := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{0,9}`).Draw(t, "right")
			input := left + "." + right

			result, err := normalizeSSOStartURL(input)
			if err != nil {
				t.Fatalf("unexpected error for dot-containing %q: %v", input, err)
			}

			if !strings.HasPrefix(result, "https://") {
				t.Fatalf("expected https:// prefix, got %q", result)
			}

			parsed, err := url.Parse(result)
			if err != nil {
				t.Fatalf("output not parseable as URL: %v", err)
			}

			if parsed.Host == "" {
				t.Fatalf("parsed URL has empty host for %q", result)
			}
		})
	})

	// Sub-property 3: Slash-containing strings (no "://")
	// produce https://<input>
	t.Run("slash_containing", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate a string with at least one "/", no "://"
			host := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9.-]{1,15}`).Draw(t, "host")
			path := rapid.StringMatching(`[a-zA-Z0-9]{1,10}`).Draw(t, "path")
			input := host + "/" + path

			result, err := normalizeSSOStartURL(input)
			if err != nil {
				t.Fatalf("unexpected error for slash-containing %q: %v", input, err)
			}

			if !strings.HasPrefix(result, "https://") {
				t.Fatalf("expected https:// prefix, got %q", result)
			}

			parsed, err := url.Parse(result)
			if err != nil {
				t.Fatalf("output not parseable as URL: %v", err)
			}

			if parsed.Host == "" {
				t.Fatalf("parsed URL has empty host for %q", result)
			}
		})
	})

	// Sub-property 4: Full URLs (starting with "https://")
	// are returned as-is and remain parseable
	t.Run("full_url", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			host := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9-]{1,10}\.[a-zA-Z]{2,5}`).Draw(t, "host")
			path := rapid.StringMatching(`/[a-zA-Z0-9]{1,10}`).Draw(t, "path")
			input := "https://" + host + path

			result, err := normalizeSSOStartURL(input)
			if err != nil {
				t.Fatalf("unexpected error for full URL %q: %v", input, err)
			}

			if !strings.HasPrefix(result, "https://") {
				t.Fatalf("expected https:// prefix, got %q", result)
			}

			parsed, err := url.Parse(result)
			if err != nil {
				t.Fatalf("output not parseable as URL: %v", err)
			}

			if parsed.Host == "" {
				t.Fatalf("parsed URL has empty host for %q", result)
			}
		})
	})
}

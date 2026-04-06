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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"testing"

	"charm.land/log/v2"
	"github.com/spf13/viper"
	"pgregory.net/rapid"
)

// TestPromptProfileSelectReturnsErrorWhenNoProfiles verifies that
// promptProfileSelect returns a descriptive error mentioning "init" when the
// AWS config file contains no [sso-session ...] sections (Requirement 3.3).
func TestPromptProfileSelectReturnsErrorWhenNoProfiles(t *testing.T) {
	logger = slog.New(log.New(io.Discard))

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	// Create a temp file with a [default] section but no sso-session sections.
	tmpFile, err := os.CreateTemp(t.TempDir(), "aws-config-*.ini")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	if _, err := tmpFile.WriteString("[default]\nregion = us-east-1\n"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	awsConfigFilePath = tmpFile.Name()

	var target string
	err = promptProfileSelect(&target)

	if err == nil {
		t.Fatalf("expected error when no SSO profiles exist, got nil")
	}

	if !strings.Contains(err.Error(), "init") {
		t.Fatalf("expected error to mention \"init\", got: %s", err.Error())
	}
}

// TestAuthCommandUsesSelectPrompt verifies that the auth command invokes
// promptProfileSelect when no profile is provided via argument or config
// (Requirement 1.1).
func TestAuthCommandUsesSelectPrompt(t *testing.T) {
	oldPrompt := promptProfileSelect
	t.Cleanup(func() { promptProfileSelect = oldPrompt })

	oldConfig := asmConfig
	t.Cleanup(func() { asmConfig = oldConfig })
	asmConfig = viper.New() // empty config, no profile-name set

	logger = slog.New(log.New(io.Discard))

	called := false
	promptProfileSelect = func(target *string) error {
		called = true
		return errors.New("spy: prompt called")
	}

	_ = authCmd.RunE(authCmd, []string{})

	if !called {
		t.Fatal("expected promptProfileSelect to be called when no profile is provided")
	}
}

// TestListCommandUsesSelectPrompt verifies that the list command invokes
// promptProfileSelect when no profile is provided via argument or config
// (Requirement 1.2).
func TestListCommandUsesSelectPrompt(t *testing.T) {
	oldPrompt := promptProfileSelect
	t.Cleanup(func() { promptProfileSelect = oldPrompt })

	oldConfig := asmConfig
	t.Cleanup(func() { asmConfig = oldConfig })
	asmConfig = viper.New()

	logger = slog.New(log.New(io.Discard))

	called := false
	promptProfileSelect = func(target *string) error {
		called = true
		return errors.New("spy: prompt called")
	}

	_ = listCmd.RunE(listCmd, []string{})

	if !called {
		t.Fatal("expected promptProfileSelect to be called when no profile is provided")
	}
}

// TestUpdateCommandUsesSelectPrompt verifies that the update command invokes
// promptProfileSelect when no profile is provided via argument or config
// (Requirement 1.3).
func TestUpdateCommandUsesSelectPrompt(t *testing.T) {
	oldPrompt := promptProfileSelect
	t.Cleanup(func() { promptProfileSelect = oldPrompt })

	oldConfig := asmConfig
	t.Cleanup(func() { asmConfig = oldConfig })
	asmConfig = viper.New()

	logger = slog.New(log.New(io.Discard))

	called := false
	promptProfileSelect = func(target *string) error {
		called = true
		return errors.New("spy: prompt called")
	}

	_ = updateCmd.RunE(updateCmd, []string{})

	if !called {
		t.Fatal("expected promptProfileSelect to be called when no profile is provided")
	}
}

// TestInitCommandUsesInputPrompt verifies that the init command does NOT invoke
// promptProfileSelect when no profile is provided. Init must use huh.NewInput
// because the profile being created doesn't exist yet (Requirements 2.1, 2.2).
func TestInitCommandUsesInputPrompt(t *testing.T) {
	oldPrompt := promptProfileSelect
	t.Cleanup(func() { promptProfileSelect = oldPrompt })

	oldConfig := asmConfig
	t.Cleanup(func() { asmConfig = oldConfig })
	asmConfig = viper.New()

	logger = slog.New(log.New(io.Discard))

	called := false
	promptProfileSelect = func(target *string) error {
		called = true
		return errors.New("spy: prompt called")
	}

	// init will fail on huh.NewInput().Run() (no TTY) — that's expected.
	// We only care that promptProfileSelect was NOT called.
	_ = initCmd.RunE(initCmd, []string{})

	if called {
		t.Fatal("expected promptProfileSelect NOT to be called for init command")
	}
}

// Feature: sso-profile-select, Property 1: SSO session parsing extracts correct sorted profile names
func TestPropertySSOSessionParsing(t *testing.T) {
	// **Validates: Requirements 3.1, 3.2**

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	logger = slog.New(log.New(io.Discard))

	rapid.Check(t, func(t *rapid.T) {
		// Generate 1–5 unique sso-session names matching [a-z][a-z0-9]{2,10}.
		numSessions := rapid.IntRange(1, 5).Draw(t, "numSessions")
		nameSet := make(map[string]bool)
		var names []string

		for len(names) < numSessions {
			name := rapid.StringMatching(`[a-z][a-z0-9]{2,10}`).Draw(t, fmt.Sprintf("name%d", len(names)))
			if !nameSet[name] {
				nameSet[name] = true
				names = append(names, name)
			}
		}

		// Build AWS config content with optional preamble, sso-session sections,
		// and interleaved profile sections.
		var sb strings.Builder

		if rapid.Bool().Draw(t, "hasPreamble") {
			sb.WriteString("[default]\nregion = us-east-1\n\n")
		}

		for _, name := range names {
			// Optionally interleave a [profile ...] section before the sso-session.
			if rapid.Bool().Draw(t, "interleaveProfile_"+name) {
				profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,10}`).Draw(t, "profileName_"+name)
				sb.WriteString(fmt.Sprintf("[profile %s]\nregion = us-east-1\n\n", profileName))
			}

			sb.WriteString(fmt.Sprintf("[sso-session %s]\n", name))
			sb.WriteString("sso_start_url = https://example.awsapps.com/start\n")
			sb.WriteString("sso_region = us-east-1\n\n")
		}

		// Write to temp file and point awsConfigFilePath at it.
		tmpDir, err := os.MkdirTemp("", "aws-config-test-*")
		if err != nil {
			t.Fatalf("create temp dir: %v", err)
		}

		tmpFile, err := os.CreateTemp(tmpDir, "aws-config-*.ini")
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}

		if _, err := tmpFile.WriteString(sb.String()); err != nil {
			t.Fatalf("write temp file: %v", err)
		}

		if err := tmpFile.Close(); err != nil {
			t.Fatalf("close temp file: %v", err)
		}

		awsConfigFilePath = tmpFile.Name()

		got, err := getAllManagedSections()
		if err != nil {
			t.Fatalf("getAllManagedSections: %v", err)
		}

		// Expected: input names in sorted order.
		expected := make([]string, len(names))
		copy(expected, names)
		slices.Sort(expected)

		if len(got) != len(expected) {
			t.Fatalf("expected %d profiles, got %d: %v", len(expected), len(got), got)
		}

		for i := range expected {
			if got[i] != expected[i] {
				t.Fatalf("index %d: expected %q, got %q", i, expected[i], got[i])
			}
		}
	})
}

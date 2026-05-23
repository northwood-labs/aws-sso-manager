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
	"context"
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

const testSpyPromptCalled = "spy: prompt called"

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

	if _, writeErr := tmpFile.WriteString("[default]\nregion = us-east-1\n"); writeErr != nil {
		t.Fatalf("write temp file: %v", writeErr)
	}

	closeErr := tmpFile.Close()
	if closeErr != nil {
		t.Fatalf("close temp file: %v", closeErr)
	}

	awsConfigFilePath = tmpFile.Name()

	var target string

	err = promptProfileSelect(&target)
	if err == nil {
		t.Fatal("expected error when no SSO profiles exist, got nil")
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

	asmConfig = viper.New() // empty config, no profile-name set.

	logger = slog.New(log.New(io.Discard))

	called := false

	promptProfileSelect = func(_ *string) error {
		called = true
		return errors.New(testSpyPromptCalled) // lint:allow_errorf
	}

	_ = authCmd.RunE(authCmd, nil) // lint:allow_unhandled

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

	promptProfileSelect = func(_ *string) error {
		called = true
		return errors.New(testSpyPromptCalled) // lint:allow_errorf
	}

	_ = listCmd.RunE(listCmd, nil) // lint:allow_unhandled

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

	promptProfileSelect = func(_ *string) error {
		called = true
		return errors.New(testSpyPromptCalled) // lint:allow_errorf
	}

	_ = updateCmd.RunE(updateCmd, nil) // lint:allow_unhandled

	if !called {
		t.Fatal("expected promptProfileSelect to be called when no profile is provided")
	}
}

// TestInitCommandUsesInputPrompt verifies that the init command does NOT invoke
// promptProfileSelect when no profile is provided as an argument. Init resolves
// the profile name from config (or huh.NewInput) — never from promptProfileSelect
// because the profile being created doesn't exist yet (Requirements 2.1, 2.2).
func TestInitCommandUsesInputPrompt(t *testing.T) {
	oldPrompt := promptProfileSelect

	t.Cleanup(func() { promptProfileSelect = oldPrompt })

	oldConfig := asmConfig

	t.Cleanup(func() { asmConfig = oldConfig })

	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	tmpFile, err := os.CreateTemp(t.TempDir(), "aws-config-*.ini")
	if err != nil {
		t.Fatal(err)
	}

	if closeErr := tmpFile.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}

	awsConfigFilePath = tmpFile.Name()

	asmConfig = viper.New()
	asmConfig.Set("profile-name", "test-profile")
	asmConfig.Set("sso-start-url", "https://test.awsapps.com/start")
	asmConfig.Set("sso-region", "us-east-1")
	asmConfig.Set("sso-scopes", "sso:account:access")

	logger = slog.New(log.New(io.Discard))

	called := false

	promptProfileSelect = func(_ *string) error {
		called = true
		return errors.New(testSpyPromptCalled) // lint:allow_errorf
	}

	// Provide all config values so the code skips every huh.NewInput() call
	// (which blocks without a TTY). The assertion validates that
	// promptProfileSelect is never invoked for init.
	initCmd.SetContext(context.Background())

	_ = initCmd.RunE(initCmd, nil) // lint:allow_unhandled

	if called {
		t.Fatal("expected promptProfileSelect NOT to be called for init command")
	}
}

// Feature: sso-profile-select, Property 1: SSO session parsing extracts correct sorted profile names.
func TestPropertySSOSessionParsing(t *testing.T) { // lint:allow_complexity
	// **Validates: Requirements 3.1, 3.2**.
	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	logger = slog.New(log.New(io.Discard))

	tmpDir := t.TempDir()

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
				fmt.Fprintf(&sb, "[profile %s]\nregion = us-east-1\n\n", profileName)
			}

			fmt.Fprintf(&sb, "[sso-session %s]\n", name)
			sb.WriteString("sso_start_url = https://example.awsapps.com/start\n")
			sb.WriteString("sso_region = us-east-1\n\n")
		}

		// Write to temp file and point awsConfigFilePath at it.
		tmpFile, err := os.CreateTemp(tmpDir, "aws-config-*.ini")
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}

		if _, writeErr := tmpFile.WriteString(sb.String()); writeErr != nil {
			t.Fatalf("write temp file: %v", writeErr)
		}

		closeErr := tmpFile.Close()
		if closeErr != nil {
			t.Fatalf("close temp file: %v", closeErr)
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

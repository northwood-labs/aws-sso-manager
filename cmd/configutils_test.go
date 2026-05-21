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
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/log/v2"
	"github.com/spf13/viper"
	"pgregory.net/rapid"
)

const (
	testConfigFile     = "config"
	testWriteConfigErr = "write config: %v"
	testProfileNewOne  = "[profile new-one]"
	testNewline        = "\n"
)

func TestSetManagedSectionReplacesManagedBlockOnce(t *testing.T) {
	t.Helper()

	logger = slog.New(log.New(io.Discard))

	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() {
		awsConfigFilePath = oldConfigPath
	})

	dir := t.TempDir()

	awsConfigFilePath = filepath.Join(dir, testConfigFile)

	configContent := strings.Join([]string{
		"[default]",
		"region = us-east-1",
		"; -------- aws-sso-manager: start abc --------",
		"[profile old-one]",
		"sso_account_id = 111111111111",
		"[profile old-two]",
		"sso_account_id = 222222222222",
		"; -------- aws-sso-manager: end abc --------",
		"[tail]",
		"value = keep",
	}, testNewline) + testNewline

	if err := os.WriteFile(awsConfigFilePath, []byte(configContent), 0o0644); err != nil {
		t.Fatalf(testWriteConfigErr, err)
	}

	tmpFile := filepath.Join(dir, "managed.ini")
	replacement := testProfileNewOne + testNewline + "sso_account_id = 333333333333" + testNewline

	if err := os.WriteFile(tmpFile, []byte(replacement), 0o0644); err != nil {
		t.Fatalf("write tmp file: %v", err)
	}

	backupName, err := setManagedSection(tmpFile, "abc")
	if err != nil {
		t.Fatalf("setManagedSection: %v", err)
	}

	content, err := os.ReadFile(backupName) // lint:allow_dynamic_filename
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}

	got := string(content)

	if strings.Count(got, testProfileNewOne) != 1 {
		t.Fatalf(
			"expected replacement to be injected once, got %d occurrences\n%s",
			strings.Count(got, testProfileNewOne),
			got,
		)
	}

	if strings.Contains(got, "[profile old-one]") || strings.Contains(got, "[profile old-two]") {
		t.Fatalf("expected old managed block contents to be removed\n%s", got)
	}

	if !strings.Contains(got, "; -------- aws-sso-manager: start abc --------") ||
		!strings.Contains(got, "; -------- aws-sso-manager: end abc --------") {
		t.Fatalf("expected managed block markers to be preserved\n%s", got)
	}

	if !strings.Contains(got, "[tail]\nvalue = keep\n") {
		t.Fatalf("expected unmanaged tail content to be preserved\n%s", got)
	}
}

func TestGetAllMarkedProfiles(t *testing.T) {
	logger = slog.New(log.New(io.Discard))

	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()

	awsConfigFilePath = filepath.Join(dir, testConfigFile)

	config := strings.Join([]string{
		"[default]",
		"region = us-east-1",
		"; -------- aws-sso-manager: start alpha --------",
		"[profile alpha-role]",
		"; -------- aws-sso-manager: end alpha --------",
		"; -------- aws-sso-manager: start beta --------",
		"[profile beta-role]",
		"; -------- aws-sso-manager: end beta --------",
	}, testNewline) + testNewline

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf(testWriteConfigErr, err)
	}

	profiles, err := getAllMarkedProfiles()
	if err != nil {
		t.Fatalf("getAllMarkedProfiles: %v", err)
	}

	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d: %v", len(profiles), profiles)
	}

	if profiles[0] != "alpha" || profiles[1] != "beta" {
		t.Fatalf("expected [alpha beta], got %v", profiles)
	}
}

func TestGetAllMarkedProfilesDeduplicates(t *testing.T) {
	logger = slog.New(log.New(io.Discard))

	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()

	awsConfigFilePath = filepath.Join(dir, testConfigFile)

	// Duplicate start markers for the same profile.
	config := strings.Join([]string{
		"; -------- aws-sso-manager: start alpha --------",
		"; -------- aws-sso-manager: end alpha --------",
		"; -------- aws-sso-manager: start alpha --------",
		"; -------- aws-sso-manager: end alpha --------",
	}, testNewline) + testNewline

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf(testWriteConfigErr, err)
	}

	profiles, err := getAllMarkedProfiles()
	if err != nil {
		t.Fatalf("getAllMarkedProfiles: %v", err)
	}

	if len(profiles) != 1 || profiles[0] != "alpha" {
		t.Fatalf("expected deduplication to produce [alpha], got %v", profiles)
	}
}

func TestValidateMarkersOK(t *testing.T) {
	logger = slog.New(log.New(io.Discard))

	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()

	awsConfigFilePath = filepath.Join(dir, testConfigFile)

	config := strings.Join([]string{
		"; -------- aws-sso-manager: start myprofile --------",
		"[profile some-role]",
		"; -------- aws-sso-manager: end myprofile --------",
	}, testNewline) + testNewline

	err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644)
	if err != nil {
		t.Fatalf(testWriteConfigErr, err)
	}

	err = validateMarkers("myprofile")
	if err != nil {
		t.Fatalf("expected no error for valid markers, got: %v", err)
	}
}

func TestValidateMarkersMismatched(t *testing.T) {
	logger = slog.New(log.New(io.Discard))

	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()

	awsConfigFilePath = filepath.Join(dir, testConfigFile)

	// Missing end marker.
	config := "; -------- aws-sso-manager: start myprofile --------\n"

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf(testWriteConfigErr, err)
	}

	err := validateMarkers("myprofile")
	if err == nil {
		t.Fatal("expected error for missing end marker")
	}
}

func TestValidateMarkersDuplicate(t *testing.T) {
	logger = slog.New(log.New(io.Discard))

	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()

	awsConfigFilePath = filepath.Join(dir, testConfigFile)

	config := strings.Join([]string{
		"; -------- aws-sso-manager: start myprofile --------",
		"; -------- aws-sso-manager: end myprofile --------",
		"; -------- aws-sso-manager: start myprofile --------",
		"; -------- aws-sso-manager: end myprofile --------",
	}, testNewline) + testNewline

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf(testWriteConfigErr, err)
	}

	err := validateMarkers("myprofile")
	if err == nil {
		t.Fatal("expected error for duplicate markers")
	}
}

func TestValidateMarkersOverlappingProfiles(t *testing.T) {
	logger = slog.New(log.New(io.Discard))

	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()

	awsConfigFilePath = filepath.Join(dir, testConfigFile)

	config := strings.Join([]string{
		"; -------- aws-sso-manager: start alpha --------",
		"; -------- aws-sso-manager: start beta --------",
		"; -------- aws-sso-manager: end alpha --------",
		"; -------- aws-sso-manager: end beta --------",
	}, testNewline) + testNewline

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf(testWriteConfigErr, err)
	}

	err := validateManagedMarkers()
	if err == nil {
		t.Fatal("expected error for overlapping managed block markers")
	}

	if !strings.Contains(err.Error(), "overlapping managed block markers") {
		t.Fatalf("expected overlapping marker error, got: %v", err)
	}

	if !strings.Contains(err.Error(), "alpha") || !strings.Contains(err.Error(), "beta") {
		t.Fatalf("expected overlapping error to mention both profiles, got: %v", err)
	}
}

func TestValidateManagedMarkersUnmatchedEnd(t *testing.T) {
	logger = slog.New(log.New(io.Discard))

	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()

	awsConfigFilePath = filepath.Join(dir, testConfigFile)

	config := "; -------- aws-sso-manager: end orphan --------\n"

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf(testWriteConfigErr, err)
	}

	err := validateManagedMarkers()
	if err == nil {
		t.Fatal("expected error for unmatched end marker")
	}

	if !strings.Contains(err.Error(), "unmatched managed block end marker") {
		t.Fatalf("expected unmatched end marker error, got: %v", err)
	}
}

func TestGetAllMarkedProfilesIncludesEndOnlyProfiles(t *testing.T) {
	logger = slog.New(log.New(io.Discard))

	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()

	awsConfigFilePath = filepath.Join(dir, testConfigFile)

	config := strings.Join([]string{
		"; -------- aws-sso-manager: end orphan --------",
		"; -------- aws-sso-manager: start alpha --------",
		"; -------- aws-sso-manager: end alpha --------",
	}, testNewline) + testNewline

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf(testWriteConfigErr, err)
	}

	profiles, err := getAllMarkedProfiles()
	if err != nil {
		t.Fatalf("getAllMarkedProfiles: %v", err)
	}

	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d: %v", len(profiles), profiles)
	}

	if profiles[0] != "alpha" || profiles[1] != "orphan" {
		t.Fatalf("expected [alpha orphan], got %v", profiles)
	}
}

func TestSetManagedSectionReplacesEachMatchingBlockDeterministically(t *testing.T) {
	t.Helper()

	logger = slog.New(log.New(io.Discard))

	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() {
		awsConfigFilePath = oldConfigPath
	})

	dir := t.TempDir()

	awsConfigFilePath = filepath.Join(dir, testConfigFile)

	configContent := strings.Join([]string{
		"; -------- aws-sso-manager: start abc --------",
		"[profile old-one]",
		"; -------- aws-sso-manager: end abc --------",
		"[middle]",
		"value = keep",
		"; -------- aws-sso-manager: start abc --------",
		"[profile old-two]",
		"; -------- aws-sso-manager: end abc --------",
	}, testNewline) + testNewline

	if err := os.WriteFile(awsConfigFilePath, []byte(configContent), 0o0644); err != nil {
		t.Fatalf(testWriteConfigErr, err)
	}

	tmpFile := filepath.Join(dir, "managed.ini")
	replacement := testProfileNewOne + testNewline

	if err := os.WriteFile(tmpFile, []byte(replacement), 0o0644); err != nil {
		t.Fatalf("write tmp file: %v", err)
	}

	backupName, err := setManagedSection(tmpFile, "abc")
	if err != nil {
		t.Fatalf("setManagedSection: %v", err)
	}

	content, err := os.ReadFile(backupName) // lint:allow_dynamic_filename
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}

	got := string(content)

	if strings.Count(got, testProfileNewOne) != 2 {
		t.Fatalf(
			"expected replacement to be applied once per matching managed block, got %d occurrences\n%s",
			strings.Count(got, testProfileNewOne),
			got,
		)
	}

	if strings.Contains(got, "[profile old-one]") || strings.Contains(got, "[profile old-two]") {
		t.Fatalf("expected old managed block contents to be removed\n%s", got)
	}
}

func TestAcquireAWSConfigLockCreatesMissingDirectory(t *testing.T) {
	logger = slog.New(log.New(io.Discard))

	oldHomeDir := userHomeDir

	t.Cleanup(func() { userHomeDir = oldHomeDir })

	dir := t.TempDir()

	userHomeDir = dir

	lock, err := acquireAWSConfigLock(context.Background())
	if err != nil {
		t.Fatalf("acquireAWSConfigLock: %v", err)
	}

	t.Cleanup(func() {
		releaseErr := lock.Release()
		if releaseErr != nil {
			t.Fatalf("release lock: %v", releaseErr)
		}
	})

	lockDir := filepath.Join(dir, ".config", ".aws-sso-manager")
	lockPath := filepath.Join(lockDir, ".config.lock")

	// Verify the lock file exists at the new path.
	lockInfo, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("expected lock file at %s: %v", lockPath, err)
	}

	// Verify lock file permissions are 0600.
	if perm := lockInfo.Mode().Perm(); perm != 0o0600 {
		t.Fatalf("expected lock file permissions 0600, got %04o", perm)
	}

	// Verify lock directory permissions are 0755.
	dirInfo, err := os.Stat(lockDir)
	if err != nil {
		t.Fatalf("expected lock directory at %s: %v", lockDir, err)
	}

	if perm := dirInfo.Mode().Perm(); perm != 0o0755 {
		t.Fatalf("expected lock directory permissions 0755, got %04o", perm)
	}
}

func TestCreateAWSConfigFileDoesNotOverwriteExistingFile(t *testing.T) {
	logger = slog.New(log.New(io.Discard))

	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()

	awsConfigFilePath = filepath.Join(dir, ".aws", testConfigFile)

	if err := os.MkdirAll(filepath.Dir(awsConfigFilePath), 0o0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	original := []byte("[default]\nregion = us-east-1\n")
	if err := os.WriteFile(awsConfigFilePath, original, 0o0644); err != nil {
		t.Fatalf(testWriteConfigErr, err)
	}

	gotPath := createAWSConfigFile(context.Background())
	if gotPath != awsConfigFilePath {
		t.Fatalf("expected path %q, got %q", awsConfigFilePath, gotPath)
	}

	content, err := os.ReadFile(awsConfigFilePath) // lint:allow_dynamic_filename
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	if !bytes.Equal(content, original) {
		t.Fatalf("expected existing config to remain unchanged, got %q", string(content))
	}
}

func TestGetProfileNameFallsBackWhenRenameConfigMissing(t *testing.T) {
	oldConfig := asmConfig

	asmConfig = viper.New()

	t.Cleanup(func() { asmConfig = oldConfig })

	got := getProfileName("nwl2", "Sandbox", "ReadOnlyAccess")
	if got != "sandbox-readonlyaccess" {
		t.Fatalf("expected fallback profile name sandbox-readonlyaccess, got %q", got)
	}
}

func TestGetProfileNameFallsBackWhenConfiguredPatternIsEmpty(t *testing.T) {
	oldConfig := asmConfig

	asmConfig = viper.New()

	t.Cleanup(func() { asmConfig = oldConfig })

	asmConfig.Set("nwl2.rename.pattern.order", []string{"PREFIX", "SUFFIX"})
	asmConfig.Set("nwl2.rename.prefix", "")
	asmConfig.Set("nwl2.rename.suffix", "")

	got := getProfileName("nwl2", "Prod Account", "AdministratorAccess")
	if got != "prod-account-administratoraccess" {
		t.Fatalf("expected fallback profile name prod-account-administratoraccess, got %q", got)
	}
}

// Feature: aws-sso-manager, Property 7: Profile Name Generation with Pattern
// **Validates: Requirements 9.1, 9.2, 9.3, 9.4, 9.5, 9.12**.
func TestPropertyProfileNameGenerationWithPattern(t *testing.T) { // lint:allow_complexity
	rapid.Check(t, func(rt *rapid.T) {
		oldConfig := asmConfig

		asmConfig = viper.New()
		defer func() { asmConfig = oldConfig }()

		profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,10}`).Draw(rt, "profileName")
		account := rapid.StringMatching(`[A-Za-z][A-Za-z0-9 ]{2,19}`).Draw(rt, "account")
		role := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{2,14}`).Draw(rt, "role")

		config := genProfilePatternConfig().Draw(rt, "patternConfig")

		// Extract pattern config values.
		patternMap, ok := config["pattern"].(map[string]any)
		if !ok {
			return
		}

		order, ok := patternMap["order"].([]string)
		if !ok {
			return
		}

		delimiter, ok := patternMap["delimiter"].(string)
		if !ok {
			delimiter = "-"
		}

		prefix, ok := config["prefix"].(string)
		if !ok {
			prefix = ""
		}

		suffix, ok := config["suffix"].(string)
		if !ok {
			suffix = ""
		}

		// Set up asvConfig with the pattern config (skip substr_match_replace for this test).
		asmConfig.Set(profileName+".rename.pattern.order", order)
		asmConfig.Set(profileName+".rename.pattern.delimiter", delimiter)
		asmConfig.Set(profileName+".rename.prefix", prefix)
		asmConfig.Set(profileName+".rename.suffix", suffix)

		got := getProfileName(profileName, account, role)

		// Property: output should be lowercased.
		if got != strings.ToLower(got) {
			rt.Fatalf("expected lowercased output, got %q", got)
		}

		// Property: output should not be empty.
		if got == "" {
			rt.Fatal("expected non-empty output")
		}

		// Build expected tokens to verify the output matches the expected
		// pattern-based join. Empty prefix/suffix are omitted from the output.
		// The final result is TrimSpace'd by getProfileName.
		var expectedTokens []string

		for _, token := range order {
			switch strings.ToLower(token) {
			case "prefix":
				if prefix != "" {
					expectedTokens = append(expectedTokens, strings.ToLower(prefix))
				}
			case "suffix":
				if suffix != "" {
					expectedTokens = append(expectedTokens, strings.ToLower(suffix))
				}
			case "account":
				expectedTokens = append(expectedTokens, strings.ToLower(account))
			case "role":
				expectedTokens = append(expectedTokens, strings.ToLower(role))
			default:
			}
		}

		if len(expectedTokens) == 0 {
			return
		}

		expected := strings.TrimSpace(strings.Join(expectedTokens, delimiter))
		if expected == "" {
			// When all tokens produce empty after trim, getProfileName falls back
			// to buildDefaultProfileName — just verify it's non-empty and lowercased.
			return
		}

		if got != expected {
			rt.Fatalf("expected %q, got %q (order=%v, delimiter=%q, prefix=%q, suffix=%q, account=%q, role=%q)",
				expected, got, order, delimiter, prefix, suffix, account, role)
		}
	})
}

// Feature: aws-sso-manager, Property 6: Managed Block Marker Validation.
func TestPropertyManagedBlockMarkerValidation(t *testing.T) { // lint:allow_complexity
	t.Run("well-formed configs produce no issues", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			logger = slog.New(log.New(io.Discard))

			oldConfigPath := awsConfigFilePath
			defer func() { awsConfigFilePath = oldConfigPath }()

			// Generate 1-3 unique profile names.
			numProfiles := rapid.IntRange(1, 3).Draw(rt, "numProfiles")
			profiles := make([]string, numProfiles)
			seen := make(map[string]bool)

			for i := range numProfiles {
				for {
					p := rapid.StringMatching(`[a-z][a-z0-9]{2,10}`).Draw(rt, fmt.Sprintf("profile%d", i))
					if seen[p] {
						continue
					}

					seen[p] = true
					profiles[i] = p

					break
				}
			}

			content := genManagedBlockConfig(profiles).Draw(rt, "config")

			dir := t.TempDir()

			tmpFile := filepath.Join(dir, testConfigFile)
			if err := os.WriteFile(tmpFile, []byte(content), 0o0644); err != nil {
				rt.Fatalf(testWriteConfigErr, err)
			}

			awsConfigFilePath = tmpFile

			report, err := inspectManagedMarkers()
			if err != nil {
				rt.Fatalf("inspectManagedMarkers: %v", err)
			}

			if len(report.issues) != 0 {
				rt.Fatalf("expected no issues for well-formed config, got %v", report.issues)
			}
		})
	})

	// **Validates: Requirements 8.3, 8.4, 8.5, 8.6, 8.7**.
	t.Run("malformed configs produce issues", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			logger = slog.New(log.New(io.Discard))

			oldConfigPath := awsConfigFilePath
			defer func() { awsConfigFilePath = oldConfigPath }()

			profile := rapid.StringMatching(`[a-z][a-z0-9]{2,10}`).Draw(rt, "profile")

			// Pick an anomaly type: 0=missing end (unclosed), 1=extra end (unmatched), 2=duplicate start.
			anomaly := rapid.IntRange(0, 2).Draw(rt, "anomaly")

			var sb strings.Builder

			switch anomaly {
			case 0: // Missing end marker (unclosed block).
				fmt.Fprintf(&sb, "; -------- aws-sso-manager: start %s --------\n", profile)
				fmt.Fprintf(&sb, "[sso-session %s]\n", profile)
				sb.WriteString("sso_start_url = https://example.awsapps.com/start\n")
			case 1: // Extra end marker (unmatched end).
				fmt.Fprintf(&sb, "; -------- aws-sso-manager: end %s --------\n", profile)
			case 2: // Duplicate start markers.
				fmt.Fprintf(&sb, "; -------- aws-sso-manager: start %s --------\n", profile)
				fmt.Fprintf(&sb, "[sso-session %s]\n", profile)
				fmt.Fprintf(&sb, "; -------- aws-sso-manager: end %s --------\n", profile)
				fmt.Fprintf(&sb, "; -------- aws-sso-manager: start %s --------\n", profile)
				fmt.Fprintf(&sb, "[sso-session %s]\n", profile)
				fmt.Fprintf(&sb, "; -------- aws-sso-manager: end %s --------\n", profile)
			default:
			}

			dir := t.TempDir()

			tmpFile := filepath.Join(dir, testConfigFile)
			if err := os.WriteFile(tmpFile, []byte(sb.String()), 0o0644); err != nil {
				rt.Fatalf(testWriteConfigErr, err)
			}

			awsConfigFilePath = tmpFile

			report, err := inspectManagedMarkers()
			if err != nil {
				rt.Fatalf("inspectManagedMarkers: %v", err)
			}

			if len(report.issues) == 0 {
				rt.Fatalf("expected issues for malformed config (anomaly=%d), got none", anomaly)
			}

			// Verify the profile with the anomaly has issues.
			if _, ok := report.issues[profile]; !ok {
				rt.Fatalf("expected issues for profile %q, got issues for: %v", profile, report.issues)
			}
		})
	})
}

// Feature: aws-sso-manager, Property 8: Substring Match Replacement in Profile Names
// **Validates: Requirements 9.6, 9.8**.
func TestPropertySubstringMatchReplacement(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		oldConfig := asmConfig

		asmConfig = viper.New()
		defer func() { asmConfig = oldConfig }()

		profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,10}`).Draw(rt, "profileName")

		// Generate a random account name (3-15 alphanumeric chars).
		account := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{2,14}`).Draw(rt, "account")

		// Pick a random substring of the account name as the match key.
		// We need at least 1 char for the substring.
		accountLower := strings.ToLower(account)
		startIdx := rapid.IntRange(0, len(accountLower)-1).Draw(rt, "startIdx")
		endIdx := rapid.IntRange(startIdx+1, len(accountLower)).Draw(rt, "endIdx")
		matchKey := accountLower[startIdx:endIdx]

		// Generate a random replacement value (1-6 lowercase chars).
		replacement := rapid.StringMatching(`[a-z]{1,6}`).Draw(rt, "replacement")

		role := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{2,14}`).Draw(rt, "role")

		// Set up asvConfig with pattern order ["ACCOUNT", "ROLE"], delimiter "-",
		// and the substr_match_replace map.
		asmConfig.Set(profileName+".rename.pattern.order", []string{"ACCOUNT", "ROLE"})
		asmConfig.Set(profileName+".rename.pattern.delimiter", "-")
		asmConfig.Set(profileName+".rename.accounts.substr_match_replace", map[string]any{
			matchKey: replacement,
		})

		got := getProfileName(profileName, account, role)

		// The output should contain the lowercased replacement value instead of
		// the account name, joined with the role by "-".
		expected := strings.ToLower(replacement) + "-" + strings.ToLower(role)
		if got != expected {
			rt.Fatalf(
				"expected %q, got %q (account=%q, matchKey=%q, replacement=%q, role=%q)",
				expected, got, account, matchKey, replacement, role,
			)
		}
	})
}

// Feature: aws-sso-manager, Property 9: Default Profile Name Generation.
func TestPropertyDefaultProfileNameGeneration(t *testing.T) {
	// **Validates: Requirements 9.10**.
	t.Run("toProfileToken_idempotence", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			input := rapid.String().Draw(t, "input")
			once := toProfileToken(input)

			twice := toProfileToken(once)
			if once != twice {
				t.Fatalf("toProfileToken is not idempotent: toProfileToken(%q)=%q, toProfileToken(%q)=%q",
					input, once, once, twice)
			}
		})
	})

	t.Run("buildDefaultProfileName_lowercased", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			account := rapid.StringMatching(`[A-Za-z0-9 _.-]{1,20}`).Draw(t, "account")
			role := rapid.StringMatching(`[A-Za-z0-9 _.-]{1,20}`).Draw(t, "role")

			got := buildDefaultProfileName(account, role)

			if got != strings.ToLower(got) {
				t.Fatalf("expected lowercased output, got %q", got)
			}

			if got == "" {
				t.Fatal("expected non-empty output")
			}
		})
	})
}

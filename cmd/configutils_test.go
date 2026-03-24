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
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
)

func TestSetManagedSectionReplacesManagedBlockOnce(t *testing.T) {
	t.Helper()

	logger = log.New(io.Discard)

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() {
		awsConfigFilePath = oldConfigPath
	})

	dir := t.TempDir()
	awsConfigFilePath = filepath.Join(dir, "config")

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
	}, "\n") + "\n"

	if err := os.WriteFile(awsConfigFilePath, []byte(configContent), 0o0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	tmpFile := filepath.Join(dir, "managed.ini")
	replacement := strings.Join([]string{
		"[profile new-one]",
		"sso_account_id = 333333333333",
	}, "\n") + "\n"

	if err := os.WriteFile(tmpFile, []byte(replacement), 0o0644); err != nil {
		t.Fatalf("write tmp file: %v", err)
	}

	backupName, err := setManagedSection(tmpFile, "abc")
	if err != nil {
		t.Fatalf("setManagedSection: %v", err)
	}

	content, err := os.ReadFile(backupName)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}

	got := string(content)

	if strings.Count(got, "[profile new-one]") != 1 {
		t.Fatalf(
			"expected replacement to be injected once, got %d occurrences\n%s",
			strings.Count(got, "[profile new-one]"),
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
	logger = log.New(io.Discard)

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()
	awsConfigFilePath = filepath.Join(dir, "config")

	config := strings.Join([]string{
		"[default]",
		"region = us-east-1",
		"; -------- aws-sso-manager: start alpha --------",
		"[profile alpha-role]",
		"; -------- aws-sso-manager: end alpha --------",
		"; -------- aws-sso-manager: start beta --------",
		"[profile beta-role]",
		"; -------- aws-sso-manager: end beta --------",
	}, "\n") + "\n"

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf("write config: %v", err)
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
	logger = log.New(io.Discard)

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()
	awsConfigFilePath = filepath.Join(dir, "config")

	// Duplicate start markers for the same profile.
	config := strings.Join([]string{
		"; -------- aws-sso-manager: start alpha --------",
		"; -------- aws-sso-manager: end alpha --------",
		"; -------- aws-sso-manager: start alpha --------",
		"; -------- aws-sso-manager: end alpha --------",
	}, "\n") + "\n"

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf("write config: %v", err)
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
	logger = log.New(io.Discard)

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()
	awsConfigFilePath = filepath.Join(dir, "config")

	config := strings.Join([]string{
		"; -------- aws-sso-manager: start myprofile --------",
		"[profile some-role]",
		"; -------- aws-sso-manager: end myprofile --------",
	}, "\n") + "\n"

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := validateMarkers("myprofile"); err != nil {
		t.Fatalf("expected no error for valid markers, got: %v", err)
	}
}

func TestValidateMarkersMismatched(t *testing.T) {
	logger = log.New(io.Discard)

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()
	awsConfigFilePath = filepath.Join(dir, "config")

	// Missing end marker.
	config := "; -------- aws-sso-manager: start myprofile --------\n"

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := validateMarkers("myprofile")
	if err == nil {
		t.Fatal("expected error for missing end marker")
	}
}

func TestValidateMarkersDuplicate(t *testing.T) {
	logger = log.New(io.Discard)

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()
	awsConfigFilePath = filepath.Join(dir, "config")

	config := strings.Join([]string{
		"; -------- aws-sso-manager: start myprofile --------",
		"; -------- aws-sso-manager: end myprofile --------",
		"; -------- aws-sso-manager: start myprofile --------",
		"; -------- aws-sso-manager: end myprofile --------",
	}, "\n") + "\n"

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := validateMarkers("myprofile")
	if err == nil {
		t.Fatal("expected error for duplicate markers")
	}
}

func TestValidateMarkersOverlappingProfiles(t *testing.T) {
	logger = log.New(io.Discard)

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()
	awsConfigFilePath = filepath.Join(dir, "config")

	config := strings.Join([]string{
		"; -------- aws-sso-manager: start alpha --------",
		"; -------- aws-sso-manager: start beta --------",
		"; -------- aws-sso-manager: end alpha --------",
		"; -------- aws-sso-manager: end beta --------",
	}, "\n") + "\n"

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf("write config: %v", err)
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
	logger = log.New(io.Discard)

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()
	awsConfigFilePath = filepath.Join(dir, "config")

	config := "; -------- aws-sso-manager: end orphan --------\n"

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf("write config: %v", err)
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
	logger = log.New(io.Discard)

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()
	awsConfigFilePath = filepath.Join(dir, "config")

	config := strings.Join([]string{
		"; -------- aws-sso-manager: end orphan --------",
		"; -------- aws-sso-manager: start alpha --------",
		"; -------- aws-sso-manager: end alpha --------",
	}, "\n") + "\n"

	if err := os.WriteFile(awsConfigFilePath, []byte(config), 0o0644); err != nil {
		t.Fatalf("write config: %v", err)
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

	logger = log.New(io.Discard)

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() {
		awsConfigFilePath = oldConfigPath
	})

	dir := t.TempDir()
	awsConfigFilePath = filepath.Join(dir, "config")

	configContent := strings.Join([]string{
		"; -------- aws-sso-manager: start abc --------",
		"[profile old-one]",
		"; -------- aws-sso-manager: end abc --------",
		"[middle]",
		"value = keep",
		"; -------- aws-sso-manager: start abc --------",
		"[profile old-two]",
		"; -------- aws-sso-manager: end abc --------",
	}, "\n") + "\n"

	if err := os.WriteFile(awsConfigFilePath, []byte(configContent), 0o0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	tmpFile := filepath.Join(dir, "managed.ini")
	replacement := "[profile new-one]\n"

	if err := os.WriteFile(tmpFile, []byte(replacement), 0o0644); err != nil {
		t.Fatalf("write tmp file: %v", err)
	}

	backupName, err := setManagedSection(tmpFile, "abc")
	if err != nil {
		t.Fatalf("setManagedSection: %v", err)
	}

	content, err := os.ReadFile(backupName)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}

	got := string(content)

	if strings.Count(got, "[profile new-one]") != 2 {
		t.Fatalf(
			"expected replacement to be applied once per matching managed block, got %d occurrences\n%s",
			strings.Count(got, "[profile new-one]"),
			got,
		)
	}

	if strings.Contains(got, "[profile old-one]") || strings.Contains(got, "[profile old-two]") {
		t.Fatalf("expected old managed block contents to be removed\n%s", got)
	}
}

func TestAcquireAWSConfigLockCreatesMissingDirectory(t *testing.T) {
	logger = log.New(io.Discard)

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()
	awsConfigFilePath = filepath.Join(dir, ".aws", "config")

	lock, err := acquireAWSConfigLock(context.Background())
	if err != nil {
		t.Fatalf("acquireAWSConfigLock: %v", err)
	}
	t.Cleanup(func() {
		if err := lock.Release(); err != nil {
			t.Fatalf("release lock: %v", err)
		}
	})

	lockPath := filepath.Join(filepath.Dir(awsConfigFilePath), ".aws-sso-manager.config.lock")
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lock file at %s: %v", lockPath, err)
	}
}

func TestCreateAWSConfigFileDoesNotOverwriteExistingFile(t *testing.T) {
	logger = log.New(io.Discard)

	oldConfigPath := awsConfigFilePath
	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()
	awsConfigFilePath = filepath.Join(dir, ".aws", "config")

	if err := os.MkdirAll(filepath.Dir(awsConfigFilePath), 0o0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	original := []byte("[default]\nregion = us-east-1\n")
	if err := os.WriteFile(awsConfigFilePath, original, 0o0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	gotPath := createAWSConfigFile()
	if gotPath != awsConfigFilePath {
		t.Fatalf("expected path %q, got %q", awsConfigFilePath, gotPath)
	}

	content, err := os.ReadFile(awsConfigFilePath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	if string(content) != string(original) {
		t.Fatalf("expected existing config to remain unchanged, got %q", string(content))
	}
}

func TestGetProfileNameFallsBackWhenRenameConfigMissing(t *testing.T) {
	oldConfig := asvConfig
	asvConfig = viper.New()
	t.Cleanup(func() { asvConfig = oldConfig })

	got := getProfileName("nwl2", "Sandbox", "ReadOnlyAccess")
	if got != "sandbox-readonlyaccess" {
		t.Fatalf("expected fallback profile name sandbox-readonlyaccess, got %q", got)
	}
}

func TestGetProfileNameFallsBackWhenConfiguredPatternIsEmpty(t *testing.T) {
	oldConfig := asvConfig
	asvConfig = viper.New()
	t.Cleanup(func() { asvConfig = oldConfig })

	asvConfig.Set("nwl2.rename.pattern.order", []string{"PREFIX", "SUFFIX"})
	asvConfig.Set("nwl2.rename.prefix", "")
	asvConfig.Set("nwl2.rename.suffix", "")

	got := getProfileName("nwl2", "Prod Account", "AdministratorAccess")
	if got != "prod-account-administratoraccess" {
		t.Fatalf("expected fallback profile name prod-account-administratoraccess, got %q", got)
	}
}

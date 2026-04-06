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
	"strings"
	"testing"

	"charm.land/log/v2"
	"github.com/spf13/viper"
	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Unit tests: command registration (Task 3.1)
// ---------------------------------------------------------------------------

func TestLogoutCommandRegistration(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "logoutCmd is registered on rootCmd",
			fn: func(t *testing.T) {
				found := false
				for _, cmd := range rootCmd.Commands() {
					if cmd == logoutCmd {
						found = true
						break
					}
				}
				if !found {
					t.Fatal("logoutCmd not found in rootCmd.Commands()")
				}
			},
		},
		{
			name: "Use field is correct",
			fn: func(t *testing.T) {
				if logoutCmd.Use != "logout [sso-profile-name]" {
					t.Fatalf("expected Use %q, got %q", "logout [sso-profile-name]", logoutCmd.Use)
				}
			},
		},
		{
			name: "accepts zero or one args",
			fn: func(t *testing.T) {
				if err := logoutCmd.Args(logoutCmd, []string{}); err != nil {
					t.Fatalf("expected 0 args to be valid: %v", err)
				}
				if err := logoutCmd.Args(logoutCmd, []string{"profile"}); err != nil {
					t.Fatalf("expected 1 arg to be valid: %v", err)
				}
				if err := logoutCmd.Args(logoutCmd, []string{"a", "b"}); err == nil {
					t.Fatal("expected 2 args to be rejected")
				}
			},
		},
		{
			name: "RunE is non-nil",
			fn: func(t *testing.T) {
				if logoutCmd.RunE == nil {
					t.Fatal("expected RunE to be non-nil")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.fn)
	}
}

// ---------------------------------------------------------------------------
// Unit tests: profile resolution (Task 3.2)
// ---------------------------------------------------------------------------

func TestLogoutProfileResolution(t *testing.T) {
	t.Run("arg provided uses arg", func(t *testing.T) {
		oldPrompt := promptProfileSelect
		oldConfig := asmConfig
		oldConfigPath := awsConfigFilePath
		oldRemove := removeFile
		t.Cleanup(func() {
			promptProfileSelect = oldPrompt
			asmConfig = oldConfig
			awsConfigFilePath = oldConfigPath
			removeFile = oldRemove
		})

		logger = slog.New(log.New(io.Discard))
		asmConfig = viper.New()

		// Create a temp AWS config with a valid sso-session.
		tmpDir := t.TempDir()
		tmpFile := tmpDir + "/config"
		content := "[sso-session myprofile]\nsso_start_url = https://example.awsapps.com/start\nsso_region = us-east-1\n"
		if err := os.WriteFile(tmpFile, []byte(content), 0o600); err != nil {
			t.Fatalf("write temp config: %v", err)
		}
		awsConfigFilePath = tmpFile

		// Stub removeFile to track what path was deleted.
		removeFile = func(path string) error { return nil }

		promptCalled := false
		promptProfileSelect = func(target *string) error {
			promptCalled = true
			return errors.New("should not be called")
		}

		err := logoutCmd.RunE(logoutCmd, []string{"myprofile"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if promptCalled {
			t.Fatal("promptProfileSelect should not be called when arg is provided")
		}
	})

	t.Run("no arg with Viper config uses config value", func(t *testing.T) {
		oldPrompt := promptProfileSelect
		oldConfig := asmConfig
		oldConfigPath := awsConfigFilePath
		oldRemove := removeFile
		t.Cleanup(func() {
			promptProfileSelect = oldPrompt
			asmConfig = oldConfig
			awsConfigFilePath = oldConfigPath
			removeFile = oldRemove
		})

		logger = slog.New(log.New(io.Discard))

		asmConfig = viper.New()
		asmConfig.Set("profile-name", "configured-profile")

		tmpDir := t.TempDir()
		tmpFile := tmpDir + "/config"
		content := "[sso-session configured-profile]\nsso_start_url = https://example.awsapps.com/start\nsso_region = us-east-1\n"
		if err := os.WriteFile(tmpFile, []byte(content), 0o600); err != nil {
			t.Fatalf("write temp config: %v", err)
		}
		awsConfigFilePath = tmpFile

		removeFile = func(path string) error { return nil }

		promptCalled := false
		promptProfileSelect = func(target *string) error {
			promptCalled = true
			return errors.New("should not be called")
		}

		err := logoutCmd.RunE(logoutCmd, []string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if promptCalled {
			t.Fatal("promptProfileSelect should not be called when config is set")
		}
	})

	t.Run("no arg and no config calls promptProfileSelect", func(t *testing.T) {
		oldPrompt := promptProfileSelect
		oldConfig := asmConfig
		t.Cleanup(func() {
			promptProfileSelect = oldPrompt
			asmConfig = oldConfig
		})

		logger = slog.New(log.New(io.Discard))
		asmConfig = viper.New()

		called := false
		promptProfileSelect = func(target *string) error {
			called = true
			return errors.New("spy: prompt called")
		}

		_ = logoutCmd.RunE(logoutCmd, []string{})

		if !called {
			t.Fatal("expected promptProfileSelect to be called when no profile is provided")
		}
	})
}

// ---------------------------------------------------------------------------
// Unit test: permission error wrapping (Task 3.3)
// ---------------------------------------------------------------------------

func TestLogoutPermissionErrorWrapping(t *testing.T) {
	oldRemove := removeFile
	oldConfigPath := awsConfigFilePath
	oldConfig := asmConfig
	t.Cleanup(func() {
		removeFile = oldRemove
		awsConfigFilePath = oldConfigPath
		asmConfig = oldConfig
	})

	logger = slog.New(log.New(io.Discard))
	asmConfig = viper.New()

	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/config"
	content := "[sso-session testprofile]\nsso_start_url = https://example.awsapps.com/start\nsso_region = us-east-1\n"
	if err := os.WriteFile(tmpFile, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	awsConfigFilePath = tmpFile

	permErr := os.ErrPermission
	removeFile = func(path string) error { return permErr }

	err := logoutCmd.RunE(logoutCmd, []string{"testprofile"})
	if err == nil {
		t.Fatal("expected error when removeFile returns permission error")
	}
	if !errors.Is(err, permErr) {
		t.Fatalf("expected error to wrap os.ErrPermission, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Property-based tests (Task 4)
// ---------------------------------------------------------------------------

// writeTestAWSConfig creates a temp AWS config file with a single sso-session
// section and returns the file path. Helper for property tests.
func writeTestAWSConfig(dir, profileName string) (string, error) {
	configPath := dir + "/config"
	content := fmt.Sprintf(
		"[sso-session %s]\nsso_start_url = https://example.awsapps.com/start\nsso_region = us-east-1\n",
		profileName,
	)

	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write temp config: %w", err)
	}

	return configPath, nil
}

// Feature: logout-command, Property 1: Cache file deletion
func TestPropertyLogoutDeletesCacheFile(t *testing.T) {
	// **Validates: Requirements 4.1**

	rapid.Check(t, func(t *rapid.T) {
		profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,10}`).Draw(t, "profileName")

		oldConfigPath := awsConfigFilePath
		oldRemove := removeFile
		oldConfig := asmConfig
		defer func() {
			awsConfigFilePath = oldConfigPath
			removeFile = oldRemove
			asmConfig = oldConfig
		}()

		logger = slog.New(log.New(io.Discard))
		asmConfig = viper.New()

		tmpDir, err := os.MkdirTemp("", "logout-prop1-*")
		if err != nil {
			t.Fatalf("create temp dir: %v", err)
		}

		configFile, err := writeTestAWSConfig(tmpDir, profileName)
		if err != nil {
			t.Fatalf("writeTestAWSConfig: %v", err)
		}
		awsConfigFilePath = configFile

		// Resolve the cache file path the same way the command does.
		sessionProfile, err := getSsoSession(profileName)
		if err != nil {
			t.Fatalf("getSsoSession: %v", err)
		}

		cacheFilePath, err := getCacheFilePath(&sessionProfile)
		if err != nil {
			t.Fatalf("getCacheFilePath: %v", err)
		}

		// Create the cache file so there's something to delete.
		cacheDir := cacheFilePath[:strings.LastIndex(cacheFilePath, "/")]
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			t.Fatalf("create cache dir: %v", err)
		}

		if err := os.WriteFile(cacheFilePath, []byte(`{"accessToken":"test"}`), 0o600); err != nil {
			t.Fatalf("write cache file: %v", err)
		}

		// Use real os.Remove for this property test.
		removeFile = os.Remove

		err = logoutCmd.RunE(logoutCmd, []string{profileName})
		if err != nil {
			t.Fatalf("logout returned error: %v", err)
		}

		// Assert the cache file no longer exists.
		if _, statErr := os.Stat(cacheFilePath); !os.IsNotExist(statErr) {
			t.Fatalf("expected cache file to be deleted, but it still exists")
		}
	})
}

// Feature: logout-command, Property 2: Missing cache file is not an error
func TestPropertyLogoutMissingFileNoError(t *testing.T) {
	// **Validates: Requirements 4.3**

	rapid.Check(t, func(t *rapid.T) {
		profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,10}`).Draw(t, "profileName")

		oldConfigPath := awsConfigFilePath
		oldRemove := removeFile
		oldConfig := asmConfig
		defer func() {
			awsConfigFilePath = oldConfigPath
			removeFile = oldRemove
			asmConfig = oldConfig
		}()

		logger = slog.New(log.New(io.Discard))
		asmConfig = viper.New()

		tmpDir, err := os.MkdirTemp("", "logout-prop2-*")
		if err != nil {
			t.Fatalf("create temp dir: %v", err)
		}

		configFile, err := writeTestAWSConfig(tmpDir, profileName)
		if err != nil {
			t.Fatalf("writeTestAWSConfig: %v", err)
		}
		awsConfigFilePath = configFile

		// Stub removeFile to return os.ErrNotExist (no cache file on disk).
		removeFile = func(path string) error { return os.ErrNotExist }

		err = logoutCmd.RunE(logoutCmd, []string{profileName})
		if err != nil {
			t.Fatalf("expected nil error for missing cache file, got: %v", err)
		}
	})
}

// Feature: logout-command, Property 3: Output always contains the profile name
func TestPropertyLogoutOutputContainsProfileName(t *testing.T) {
	// **Validates: Requirements 4.2, 4.3**

	rapid.Check(t, func(t *rapid.T) {
		profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,10}`).Draw(t, "profileName")
		cacheExists := rapid.Bool().Draw(t, "cacheExists")

		oldConfigPath := awsConfigFilePath
		oldRemove := removeFile
		oldConfig := asmConfig
		oldStdout := os.Stdout
		defer func() {
			awsConfigFilePath = oldConfigPath
			removeFile = oldRemove
			asmConfig = oldConfig
			os.Stdout = oldStdout
		}()

		logger = slog.New(log.New(io.Discard))
		asmConfig = viper.New()

		tmpDir, err := os.MkdirTemp("", "logout-prop3-*")
		if err != nil {
			t.Fatalf("create temp dir: %v", err)
		}

		configFile, err := writeTestAWSConfig(tmpDir, profileName)
		if err != nil {
			t.Fatalf("writeTestAWSConfig: %v", err)
		}
		awsConfigFilePath = configFile

		if cacheExists {
			removeFile = func(path string) error { return nil }
		} else {
			removeFile = func(path string) error { return os.ErrNotExist }
		}

		// Capture stdout.
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("create pipe: %v", err)
		}
		os.Stdout = w

		_ = logoutCmd.RunE(logoutCmd, []string{profileName})

		w.Close()
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		if !strings.Contains(output, profileName) {
			t.Fatalf("expected output to contain profile name %q, got: %q", profileName, output)
		}
	})
}

// Feature: logout-command, Property 4: getSsoSession error propagation
func TestPropertyLogoutInvalidProfileReturnsError(t *testing.T) {
	// **Validates: Requirements 3.3**

	rapid.Check(t, func(t *rapid.T) {
		// Generate a profile name that won't match any sso-session in the config.
		invalidProfile := rapid.StringMatching(`[a-z][a-z0-9]{2,10}`).Draw(t, "invalidProfile")

		oldConfigPath := awsConfigFilePath
		oldRemove := removeFile
		oldConfig := asmConfig
		defer func() {
			awsConfigFilePath = oldConfigPath
			removeFile = oldRemove
			asmConfig = oldConfig
		}()

		logger = slog.New(log.New(io.Discard))
		asmConfig = viper.New()

		// Create a config with a DIFFERENT sso-session name so the generated
		// profile will never match.
		tmpDir, err := os.MkdirTemp("", "logout-prop4-*")
		if err != nil {
			t.Fatalf("create temp dir: %v", err)
		}

		configPath := tmpDir + "/config"
		content := "[sso-session zzz-never-match-this-profile]\nsso_start_url = https://example.awsapps.com/start\nsso_region = us-east-1\n"
		if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
			t.Fatalf("write temp config: %v", err)
		}
		awsConfigFilePath = configPath

		removeCalled := false
		removeFile = func(path string) error {
			removeCalled = true
			return nil
		}

		err = logoutCmd.RunE(logoutCmd, []string{invalidProfile})
		if err == nil {
			t.Fatalf("expected error for invalid profile %q, got nil", invalidProfile)
		}
		if removeCalled {
			t.Fatalf("removeFile should not be called when getSsoSession fails")
		}
	})
}

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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"pgregory.net/rapid"
)

func TestParseCacheDurationFlag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{name: "go duration", input: "24h", expected: 24 * time.Hour},
		{name: "days only", input: "1d", expected: 24 * time.Hour},
		{name: "days and hours and minutes", input: "1d6h30m", expected: 30*time.Hour + 30*time.Minute},
		{name: "invalid", input: "abc", wantErr: true},
		{name: "zero", input: "0h", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
		{name: "negative duration", input: "-1h", wantErr: true},
		{name: "multi-day token", input: "2d12h", expected: 60 * time.Hour},
		{name: "whitespace only", input: "   ", wantErr: true},
		{name: "uppercase D suffix", input: "1D", expected: 24 * time.Hour},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCacheDurationFlag(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.input)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseCacheDurationFlag(%q): %v", tc.input, err)
			}

			if got != tc.expected {
				t.Fatalf("expected %s for %q, got %s", tc.expected, tc.input, got)
			}
		})
	}
}

func TestExecutePassesFangNotifyOption(t *testing.T) {
	oldRunRootCommand := runRootCommand
	oldOSExit := osExit
	t.Cleanup(func() {
		runRootCommand = oldRunRootCommand
		osExit = oldOSExit
	})

	called := false
	runRootCommand = func(ctx context.Context, cmd *cobra.Command, signals ...os.Signal) error {
		called = true

		if ctx == nil {
			t.Fatal("expected non-nil context passed to runRootCommand")
		}
		if cmd != rootCmd {
			t.Fatal("expected rootCmd to be passed to runRootCommand")
		}
		if len(signals) != 2 {
			t.Fatalf("expected exactly 2 notify signals, got %d", len(signals))
		}
		if signals[0] != syscall.SIGINT || signals[1] != syscall.SIGTERM {
			t.Fatalf("expected notify signals [SIGINT SIGTERM], got %v", signals)
		}

		return nil
	}

	exitCalled := false
	osExit = func(code int) {
		exitCalled = true
		t.Fatalf("did not expect osExit to be called, got code %d", code)
	}

	Execute()

	if !called {
		t.Fatal("expected runRootCommand to be called")
	}
	if exitCalled {
		t.Fatal("did not expect osExit to be called")
	}
}

func TestExecuteExitsOnFangError(t *testing.T) {
	oldRunRootCommand := runRootCommand
	oldOSExit := osExit
	t.Cleanup(func() {
		runRootCommand = oldRunRootCommand
		osExit = oldOSExit
	})

	runRootCommand = func(context.Context, *cobra.Command, ...os.Signal) error {
		return errors.New("boom")
	}

	exitCode := -1
	osExit = func(code int) {
		exitCode = code
	}

	Execute()

	if exitCode != 1 {
		t.Fatalf("expected osExit to be called with code 1, got %d", exitCode)
	}
}

func TestRunRootCommandUsesFangExecute(t *testing.T) {
	oldFangExecute := fangExecute
	t.Cleanup(func() {
		fangExecute = oldFangExecute
	})

	called := false
	fangExecute = func(ctx context.Context, cmd *cobra.Command, options ...fang.Option) error {
		called = true

		if ctx == nil {
			t.Fatal("expected non-nil context passed to fangExecute")
		}
		if cmd != rootCmd {
			t.Fatal("expected rootCmd to be passed to fangExecute")
		}
		if len(options) != 1 {
			t.Fatalf("expected exactly one Fang option, got %d", len(options))
		}

		return nil
	}

	if err := runRootCommand(context.Background(), rootCmd, syscall.SIGINT, syscall.SIGTERM); err != nil {
		t.Fatalf("runRootCommand returned error: %v", err)
	}

	if !called {
		t.Fatal("expected fangExecute to be called")
	}
}

func TestVerboseLevels(t *testing.T) {
	oldLogger := logger
	oldVerbose := fVerbose
	oldCacheDuration := fCacheDuration
	oldConfigFile := fConfigFile
	oldConfig := asvConfig
	t.Cleanup(func() {
		logger = oldLogger
		fVerbose = oldVerbose
		fCacheDuration = oldCacheDuration
		fConfigFile = oldConfigFile
		asvConfig = oldConfig
	})

	tmpDir := t.TempDir()
	tmpConfig := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(tmpConfig, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	fConfigFile = tmpConfig
	fCacheDuration = "24h"

	tests := []struct {
		name          string
		verbose       int
		expectedLevel log.Level
		reportCaller  bool
	}{
		{name: "no verbose", verbose: 0, expectedLevel: log.WarnLevel, reportCaller: false},
		{name: "-v", verbose: 1, expectedLevel: log.InfoLevel, reportCaller: false},
		{name: "-vv", verbose: 2, expectedLevel: log.DebugLevel, reportCaller: false},
		{name: "-vvv", verbose: 3, expectedLevel: log.DebugLevel, reportCaller: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asvConfig = viper.New()

			var buf bytes.Buffer
			logger = log.NewWithOptions(&buf, log.Options{
				ReportTimestamp: false,
			})
			fVerbose = tc.verbose

			cmd := &cobra.Command{Use: "test"}
			err := rootCmd.PersistentPreRunE(cmd, nil)
			if err != nil {
				t.Fatalf("PersistentPreRunE: %v", err)
			}

			if logger.GetLevel() != tc.expectedLevel {
				t.Errorf("expected level %v, got %v", tc.expectedLevel, logger.GetLevel())
			}

			// Test ReportCaller indirectly by emitting a log line and
			// checking whether the output contains a caller reference
			// (e.g. "root_test.go").
			buf.Reset()
			logger.Debug("probe")

			output := buf.String()
			hasCaller := strings.Contains(output, ".go:")

			if tc.reportCaller && !hasCaller {
				t.Errorf("expected ReportCaller=true output to contain caller info, got %q", output)
			}
			if !tc.reportCaller && hasCaller {
				t.Errorf("expected ReportCaller=false output to NOT contain caller info, got %q", output)
			}
		})
	}
}

func TestEnvPrefixASM(t *testing.T) {
	// Save and restore the global asvConfig on cleanup.
	oldConfig := asvConfig
	oldConfigFile := fConfigFile
	t.Cleanup(func() {
		asvConfig = oldConfig
		fConfigFile = oldConfigFile
	})

	// Create a minimal temp config file so initializeConfig doesn't fail.
	tmpDir := t.TempDir()
	tmpConfig := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(tmpConfig, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	t.Run("ASM prefix is read", func(t *testing.T) {
		asvConfig = viper.New()
		fConfigFile = tmpConfig

		// The env key replacer maps "-" to "_", so for key "profile-name"
		// Viper looks up env var ASM_PROFILE_NAME.
		t.Setenv("ASM_PROFILE_NAME", "test-profile-from-env")

		cmd := &cobra.Command{Use: "test"}
		if err := initializeConfig(cmd); err != nil {
			t.Fatalf("initializeConfig: %v", err)
		}

		got := asvConfig.GetString("profile-name")
		if got != "test-profile-from-env" {
			t.Fatalf("expected %q, got %q", "test-profile-from-env", got)
		}
	})

	t.Run("old ASV prefix is not read", func(t *testing.T) {
		asvConfig = viper.New()
		fConfigFile = tmpConfig

		// Set the old ASV_ prefix env var — should NOT be picked up.
		t.Setenv("ASV_PROFILE_NAME", "old-prefix-value")

		cmd := &cobra.Command{Use: "test"}
		if err := initializeConfig(cmd); err != nil {
			t.Fatalf("initializeConfig: %v", err)
		}

		got := asvConfig.GetString("profile-name")
		if got == "old-prefix-value" {
			t.Fatal("ASV_ prefix should no longer be read, but got old-prefix-value")
		}
	})
}

// Feature: aws-sso-manager, Property 12: Cache Duration Parsing
func TestPropertyCacheDurationParsing(t *testing.T) {
	// **Validates: Requirements 10.6, 10.8**

	t.Run("valid_go_durations", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			hours := rapid.IntRange(1, 100).Draw(t, "hours")
			minutes := rapid.IntRange(0, 59).Draw(t, "minutes")
			input := fmt.Sprintf("%dh%dm", hours, minutes)

			got, err := parseCacheDurationFlag(input)
			if err != nil {
				t.Fatalf("parseCacheDurationFlag(%q): %v", input, err)
			}
			if got <= 0 {
				t.Fatalf("expected positive duration for %q, got %s", input, got)
			}
		})
	})

	t.Run("day_tokens", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			days := rapid.IntRange(1, 30).Draw(t, "days")
			extraHours := rapid.IntRange(0, 23).Draw(t, "extraHours")

			var input string
			if extraHours > 0 {
				input = fmt.Sprintf("%dd%dh", days, extraHours)
			} else {
				input = fmt.Sprintf("%dd", days)
			}

			got, err := parseCacheDurationFlag(input)
			if err != nil {
				t.Fatalf("parseCacheDurationFlag(%q): %v", input, err)
			}

			expected := time.Duration(days*24+extraHours) * time.Hour
			if got != expected {
				t.Fatalf("expected %s for %q, got %s", expected, input, got)
			}
		})
	})

	t.Run("invalid_inputs", func(t *testing.T) {
		// Empty string
		if _, err := parseCacheDurationFlag(""); err == nil {
			t.Fatal("expected error for empty string")
		}
		// Zero
		if _, err := parseCacheDurationFlag("0h"); err == nil {
			t.Fatal("expected error for zero duration")
		}
		// Negative
		if _, err := parseCacheDurationFlag("-1h"); err == nil {
			t.Fatal("expected error for negative duration")
		}
	})
}

func TestCommandAliases(t *testing.T) {
	tests := []struct {
		name    string
		cmd     *cobra.Command
		aliases []string
	}{
		{name: "auth has login alias", cmd: authCmd, aliases: []string{"login"}},
		{name: "list has ls alias", cmd: listCmd, aliases: []string{"ls"}},
		{name: "update has upgrade and sync aliases", cmd: updateCmd, aliases: []string{"upgrade", "sync"}},
		{name: "validate has check and lint aliases", cmd: validateCmd, aliases: []string{"check", "lint"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, alias := range tc.aliases {
				found := false
				for _, a := range tc.cmd.Aliases {
					if a == alias {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected alias %q on command %q, got aliases %v", alias, tc.cmd.Use, tc.cmd.Aliases)
				}
			}
		})
	}
}

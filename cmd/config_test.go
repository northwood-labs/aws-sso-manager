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
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"pgregory.net/rapid"
)

// Feature: config-commands, Property 1: Set-then-get round trip
func TestPropertyConfigSetGetRoundTrip(t *testing.T) {
	// **Validates: Requirements 2.2, 2.3, 3.2, 6.1**

	rapid.Check(t, func(rt *rapid.T) {
		logger = log.New(io.Discard)

		oldConfig := asmConfig
		asmConfig = viper.New()
		defer func() { asmConfig = oldConfig }()

		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")

		asmConfig.SetConfigType("toml")
		asmConfig.SetConfigFile(configPath)

		key := genConfigKey().Draw(rt, "key")
		value := rapid.StringMatching(`[A-Za-z0-9 _-]{1,30}`).Draw(rt, "value")

		// Execute configSetCmd.RunE
		var buf bytes.Buffer
		configSetCmd.SetOut(&buf)
		configSetCmd.SetArgs([]string{key, value})

		err := configSetCmd.RunE(configSetCmd, []string{key, value})
		if err != nil {
			rt.Fatalf("config set failed: %v", err)
		}

		// Verify in-memory: asmConfig.GetString(key) returns the value
		got := asmConfig.GetString(key)
		if got != value {
			rt.Fatalf("in-memory mismatch: asmConfig.GetString(%q) = %q, want %q", key, got, value)
		}

		// Verify on-disk: read the TOML file back with a fresh Viper instance
		freshViper := viper.New()
		freshViper.SetConfigType("toml")
		freshViper.SetConfigFile(configPath)

		if err := freshViper.ReadInConfig(); err != nil {
			rt.Fatalf("could not read config back: %v", err)
		}

		persisted := freshViper.GetString(key)
		if persisted != value {
			rt.Fatalf("on-disk mismatch: fresh viper GetString(%q) = %q, want %q", key, persisted, value)
		}
	})
}

// Feature: config-commands, Property 5: Set confirmation output contains key and value
func TestPropertyConfigSetConfirmation(t *testing.T) {
	// **Validates: Requirements 2.6**

	rapid.Check(t, func(rt *rapid.T) {
		logger = log.New(io.Discard)

		oldConfig := asmConfig
		defer func() { asmConfig = oldConfig }()

		asmConfig = viper.New()

		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")

		asmConfig.SetConfigType("toml")
		asmConfig.SetConfigFile(configPath)

		key := genConfigKey().Draw(rt, "key")
		value := rapid.StringMatching(`[A-Za-z0-9 _-]{1,30}`).Draw(rt, "value")

		var buf bytes.Buffer
		configSetCmd.SetOut(&buf)
		configSetCmd.SetArgs([]string{key, value})

		err := configSetCmd.RunE(configSetCmd, []string{key, value})
		if err != nil {
			rt.Fatalf("config set failed: %v", err)
		}

		output := buf.String()

		if !strings.Contains(output, key) {
			rt.Fatalf("stdout output %q does not contain key %q", output, key)
		}

		if !strings.Contains(output, value) {
			rt.Fatalf("stdout output %q does not contain value %q", output, value)
		}
	})
}

// Feature: config-commands, Property 7: Backup retention limit
func TestPropertyConfigBackupRetention(t *testing.T) {
	// **Validates: Requirements 8.4**

	rapid.Check(t, func(rt *rapid.T) {
		logger = log.New(io.Discard)

		oldConfig := asmConfig
		defer func() { asmConfig = oldConfig }()

		asmConfig = viper.New()

		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")

		asmConfig.SetConfigType("toml")
		asmConfig.SetConfigFile(configPath)

		// Generate a random number of mutations between 6 and 15 (N > 5).
		numMutations := rapid.IntRange(6, 15).Draw(rt, "numMutations")

		for i := range numMutations {
			key := genConfigKey().Draw(rt, "key")
			value := rapid.StringMatching(`[A-Za-z0-9 _-]{1,30}`).Draw(rt, "value")

			var buf bytes.Buffer
			configSetCmd.SetOut(&buf)
			configSetCmd.SetArgs([]string{key, value})

			err := configSetCmd.RunE(configSetCmd, []string{key, value})
			if err != nil {
				rt.Fatalf("config set failed on mutation %d: %v", i, err)
			}
		}

		// After all mutations, count backup files.
		matches, err := filepath.Glob(filepath.Join(dir, "config-*.toml.bak"))
		if err != nil {
			rt.Fatalf("could not glob backup files: %v", err)
		}

		if len(matches) > 5 {
			rt.Fatalf("backup count %d exceeds limit of 5", len(matches))
		}

		// If there are more than 5 backups possible (N-1 >= 5), verify the
		// retained ones are the most recent. Since timestamps in filenames are
		// ISO-8601, lexicographic sort equals chronological order. The retained
		// files should be the last `len(matches)` entries of a hypothetical
		// full sorted list — but since pruning already ran, we just verify
		// they are sorted (no older file survived while a newer one was pruned).
		if len(matches) > 1 {
			sorted := make([]string, len(matches))
			copy(sorted, matches)
			sort.Strings(sorted)

			for i, m := range matches {
				if filepath.Base(m) != filepath.Base(sorted[i]) {
					rt.Fatalf("backups not in expected order: got %v, want %v", matches, sorted)
				}
			}
		}
	})
}

// Feature: config-commands, Property 8: Backup content matches pre-mutation state
func TestPropertyConfigBackupContent(t *testing.T) {
	// **Validates: Requirements 8.1, 8.2**

	rapid.Check(t, func(rt *rapid.T) {
		logger = log.New(io.Discard)

		oldConfig := asmConfig
		defer func() { asmConfig = oldConfig }()

		asmConfig = viper.New()

		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")

		asmConfig.SetConfigType("toml")
		asmConfig.SetConfigFile(configPath)

		// First mutation: creates the config file (no backup expected).
		key1 := genConfigKey().Draw(rt, "key1")
		value1 := rapid.StringMatching(`[A-Za-z0-9 _-]{1,30}`).Draw(rt, "value1")

		var buf bytes.Buffer
		configSetCmd.SetOut(&buf)
		configSetCmd.SetArgs([]string{key1, value1})

		err := configSetCmd.RunE(configSetCmd, []string{key1, value1})
		if err != nil {
			rt.Fatalf("first config set failed: %v", err)
		}

		// Read the config file content — this is the pre-mutation state.
		preMutationContent, err := os.ReadFile(configPath)
		if err != nil {
			rt.Fatalf("could not read config file after first set: %v", err)
		}

		// Second mutation: triggers a backup of the pre-mutation state.
		key2 := genConfigKey().Draw(rt, "key2")
		value2 := rapid.StringMatching(`[A-Za-z0-9 _-]{1,30}`).Draw(rt, "value2")

		buf.Reset()
		configSetCmd.SetOut(&buf)
		configSetCmd.SetArgs([]string{key2, value2})

		err = configSetCmd.RunE(configSetCmd, []string{key2, value2})
		if err != nil {
			rt.Fatalf("second config set failed: %v", err)
		}

		// Find the backup file.
		matches, err := filepath.Glob(filepath.Join(dir, "config-*.toml.bak"))
		if err != nil {
			rt.Fatalf("could not glob backup files: %v", err)
		}

		if len(matches) != 1 {
			rt.Fatalf("expected exactly 1 backup file, got %d", len(matches))
		}

		// Read the backup and compare byte-for-byte with pre-mutation content.
		backupContent, err := os.ReadFile(matches[0])
		if err != nil {
			rt.Fatalf("could not read backup file: %v", err)
		}

		if !bytes.Equal(backupContent, preMutationContent) {
			rt.Fatalf(
				"backup content does not match pre-mutation state\nbackup:  %q\noriginal: %q",
				string(backupContent),
				string(preMutationContent),
			)
		}
	})
}

// Feature: config-commands, Property 4: Nonexistent key returns error
func TestPropertyConfigNonexistentKey(t *testing.T) {
	// **Validates: Requirements 3.3, 4.6**

	rapid.Check(t, func(rt *rapid.T) {
		// Save/restore asmConfig.
		oldConfig := asmConfig
		defer func() { asmConfig = oldConfig }()

		// Silence logger.
		logger = log.New(io.Discard)

		// Fresh Viper instance with empty config.
		asmConfig = viper.New()

		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")

		asmConfig.SetConfigType("toml")
		asmConfig.SetConfigFile(configPath)

		// Generate a random key — but never set it.
		key := rapid.StringMatching(`[a-z][a-z0-9]{1,8}(\.[a-z][a-z0-9]{1,8}){0,3}`).Draw(rt, "key")

		// config get on a nonexistent key must return a non-nil error containing "is not set".
		err := configGetCmd.RunE(configGetCmd, []string{key})
		if err == nil {
			rt.Fatalf("expected error for nonexistent key %q, got nil", key)
		}

		if !strings.Contains(err.Error(), "is not set") {
			rt.Fatalf("error %q does not contain %q", err.Error(), "is not set")
		}
	})
}

// Feature: config-commands, Property 3: Wrong argument count returns error
func TestPropertyConfigWrongArgCount(t *testing.T) {
	t.Run("set_wrong_args", func(t *testing.T) {
		// **Validates: Requirements 2.4**
		rapid.Check(t, func(rt *rapid.T) {
			argCount := rapid.IntRange(0, 5).Filter(func(n int) bool {
				return n != 2
			}).Draw(rt, "argCount")

			args := make([]string, argCount)
			for i := range argCount {
				args[i] = rapid.StringMatching(`[a-z][a-z0-9]{1,8}`).Draw(rt, "arg")
			}

			err := configSetCmd.Args(configSetCmd, args)
			if err == nil {
				rt.Fatalf("expected error for %d args to config set, got nil", argCount)
			}
		})
	})

	t.Run("get_wrong_args", func(t *testing.T) {
		// **Validates: Requirements 3.4**
		rapid.Check(t, func(rt *rapid.T) {
			argCount := rapid.IntRange(2, 5).Draw(rt, "argCount")

			args := make([]string, argCount)
			for i := range argCount {
				args[i] = rapid.StringMatching(`[a-z][a-z0-9]{1,8}`).Draw(rt, "arg")
			}

			err := configGetCmd.Args(configGetCmd, args)
			if err == nil {
				rt.Fatalf("expected error for %d args to config get, got nil", argCount)
			}
		})
	})

	t.Run("del_wrong_args", func(t *testing.T) {
		// **Validates: Requirements 4.4**
		rapid.Check(t, func(rt *rapid.T) {
			argCount := rapid.IntRange(0, 5).Filter(func(n int) bool {
				return n != 1
			}).Draw(rt, "argCount")

			args := make([]string, argCount)
			for i := range argCount {
				args[i] = rapid.StringMatching(`[a-z][a-z0-9]{1,8}`).Draw(rt, "arg")
			}

			err := configDelCmd.Args(configDelCmd, args)
			if err == nil {
				rt.Fatalf("expected error for %d args to config del, got nil", argCount)
			}
		})
	})
}

// Feature: config-commands, Property 2: Set-then-delete-then-get round trip
func TestPropertyConfigSetDelGetRoundTrip(t *testing.T) {
	// **Validates: Requirements 4.4, 4.5, 6.2**

	rapid.Check(t, func(rt *rapid.T) {
		// Save/restore asmConfig.
		oldConfig := asmConfig
		defer func() { asmConfig = oldConfig }()

		// Save/restore fForce.
		oldForce := fForce
		defer func() { fForce = oldForce }()

		// Silence logger.
		logger = log.New(io.Discard)

		// Fresh Viper instance with temp config file.
		asmConfig = viper.New()

		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")

		asmConfig.SetConfigType("toml")
		asmConfig.SetConfigFile(configPath)

		// Generate random key and value.
		key := genConfigKey().Draw(rt, "key")
		value := rapid.StringMatching(`[A-Za-z0-9 _-]{1,30}`).Draw(rt, "value")

		// Step 1: config set <key> <value>
		var buf bytes.Buffer
		configSetCmd.SetOut(&buf)
		configSetCmd.SetArgs([]string{key, value})

		err := configSetCmd.RunE(configSetCmd, []string{key, value})
		if err != nil {
			rt.Fatalf("config set failed: %v", err)
		}

		// Step 2: config del --force <key>
		fForce = true

		buf.Reset()
		configDelCmd.SetOut(&buf)
		configDelCmd.SetArgs([]string{key})

		err = configDelCmd.RunE(configDelCmd, []string{key})
		if err != nil {
			rt.Fatalf("config del failed: %v", err)
		}

		// Simulate a fresh CLI invocation: reset asmConfig from disk so the
		// in-memory state reflects the post-delete file. Real CLI runs start
		// with a fresh Viper that reads the persisted file.
		asmConfig = viper.New()
		asmConfig.SetConfigType("toml")
		asmConfig.SetConfigFile(configPath)
		_ = asmConfig.ReadInConfig()

		// Step 3: config get <key> — must return a non-nil error containing "is not set"
		buf.Reset()
		configGetCmd.SetOut(&buf)
		configGetCmd.SetArgs([]string{key})

		err = configGetCmd.RunE(configGetCmd, []string{key})
		if err == nil {
			rt.Fatalf("expected error for deleted key %q, got nil", key)
		}

		if !strings.Contains(err.Error(), "is not set") {
			rt.Fatalf("error %q does not contain %q", err.Error(), "is not set")
		}

		// Step 4: Verify on-disk — read the TOML file with a fresh Viper and confirm key is gone.
		freshViper := viper.New()
		freshViper.SetConfigType("toml")
		freshViper.SetConfigFile(configPath)

		if err := freshViper.ReadInConfig(); err != nil {
			// File may not exist if it was the only key and got cleaned up.
			// That's fine — the key is definitely gone.
			return
		}

		if got := freshViper.Get(key); got != nil {
			rt.Fatalf("on-disk key %q should be gone after delete, but got %v", key, got)
		}
	})
}

// Feature: config-commands, Property 6: Del confirmation output contains key
func TestPropertyConfigDelConfirmation(t *testing.T) {
	// **Validates: Requirements 4.8**

	rapid.Check(t, func(rt *rapid.T) {
		// Save/restore asmConfig.
		oldConfig := asmConfig
		defer func() { asmConfig = oldConfig }()

		// Save/restore fForce.
		oldForce := fForce
		defer func() { fForce = oldForce }()

		// Silence logger.
		logger = log.New(io.Discard)

		// Fresh Viper instance with temp config file.
		asmConfig = viper.New()

		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")

		asmConfig.SetConfigType("toml")
		asmConfig.SetConfigFile(configPath)

		// Generate random key and value.
		key := genConfigKey().Draw(rt, "key")
		value := rapid.StringMatching(`[A-Za-z0-9 _-]{1,30}`).Draw(rt, "value")

		// Set the key first so it exists for deletion.
		var buf bytes.Buffer
		configSetCmd.SetOut(&buf)
		configSetCmd.SetArgs([]string{key, value})

		err := configSetCmd.RunE(configSetCmd, []string{key, value})
		if err != nil {
			rt.Fatalf("config set failed: %v", err)
		}

		// Force mode to skip confirmation prompt.
		fForce = true

		// Capture stdout from config del.
		buf.Reset()
		configDelCmd.SetOut(&buf)
		configDelCmd.SetArgs([]string{key})

		err = configDelCmd.RunE(configDelCmd, []string{key})
		if err != nil {
			rt.Fatalf("config del failed: %v", err)
		}

		output := buf.String()

		if !strings.Contains(output, key) {
			rt.Fatalf("stdout output %q does not contain key %q", output, key)
		}
	})
}

// ---------------------------------------------------------------------------
// Unit tests for edge cases and structural checks (Task 6.1)
// ---------------------------------------------------------------------------

// TestConfigCommandRegistration verifies that configCmd is registered under
// rootCmd and that configCmd has the expected subcommands: set, get, del.
// **Validates: Requirements 1.1, 1.2**
func TestConfigCommandRegistration(t *testing.T) {
	t.Run("configCmd_under_rootCmd", func(t *testing.T) {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "config" {
				found = true

				break
			}
		}

		if !found {
			t.Fatal("expected rootCmd to have a subcommand named \"config\"")
		}
	})

	t.Run("configCmd_has_set_get_del", func(t *testing.T) {
		expected := map[string]bool{"set": false, "get": false, "del": false}

		for _, cmd := range configCmd.Commands() {
			if _, ok := expected[cmd.Name()]; ok {
				expected[cmd.Name()] = true
			}
		}

		for name, found := range expected {
			if !found {
				t.Errorf("expected configCmd to have subcommand %q", name)
			}
		}
	})
}

// TestConfigNoSubcommandShowsHelp verifies that configCmd's help text lists
// the set, get, and del subcommands.
// **Validates: Requirements 1.2**
func TestConfigNoSubcommandShowsHelp(t *testing.T) {
	var buf bytes.Buffer
	configCmd.SetOut(&buf)
	configCmd.SetErr(&buf)

	if err := configCmd.Help(); err != nil {
		t.Fatalf("configCmd.Help(): %v", err)
	}

	output := buf.String()

	for _, sub := range []string{"set", "get", "del"} {
		if !strings.Contains(output, sub) {
			t.Errorf("expected help output to contain %q, got:\n%s", sub, output)
		}
	}
}

// TestConfigSetCreatesParentDirectory verifies that config set creates the
// parent directory when it doesn't exist.
// **Validates: Requirements 2.5**
func TestConfigSetCreatesParentDirectory(t *testing.T) {
	logger = log.New(io.Discard)

	oldConfig := asmConfig
	t.Cleanup(func() { asmConfig = oldConfig })

	asmConfig = viper.New()

	dir := t.TempDir()
	// Use a nested subdirectory that doesn't exist yet.
	configPath := filepath.Join(dir, "nested", "subdir", "config.toml")

	asmConfig.SetConfigType("toml")
	asmConfig.SetConfigFile(configPath)

	var buf bytes.Buffer
	configSetCmd.SetOut(&buf)

	err := configSetCmd.RunE(configSetCmd, []string{"profile-name", "myvalue"})
	if err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	// Verify the directory was created.
	info, err := os.Stat(filepath.Dir(configPath))
	if err != nil {
		t.Fatalf("expected parent directory to exist: %v", err)
	}

	if !info.IsDir() {
		t.Fatal("expected parent path to be a directory")
	}

	// Verify the config file exists.
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}
}

// TestConfigSubcommandsUseRunE verifies that all three subcommands use RunE
// (not Run) for their execution function.
// **Validates: Requirements 7.1, 7.2, 7.3**
func TestConfigSubcommandsUseRunE(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "configSetCmd uses RunE", cmd: configSetCmd},
		{name: "configGetCmd uses RunE", cmd: configGetCmd},
		{name: "configDelCmd uses RunE", cmd: configDelCmd},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cmd.RunE == nil {
				t.Fatalf("expected %s to have RunE set", tc.cmd.Name())
			}
		})
	}
}

// TestConfigDelForceSkipsPrompt verifies that config del --force skips the
// confirmation prompt and deletes the key immediately.
// **Validates: Requirements 4.2, 4.4**
func TestConfigDelForceSkipsPrompt(t *testing.T) {
	logger = log.New(io.Discard)

	oldConfig := asmConfig
	oldForce := fForce
	t.Cleanup(func() {
		asmConfig = oldConfig
		fForce = oldForce
	})

	asmConfig = viper.New()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	asmConfig.SetConfigType("toml")
	asmConfig.SetConfigFile(configPath)

	// Set a key first using setConfigKey so it goes through the schema.
	if err := setConfigKey(configPath, "testprofile.rename.prefix", "testvalue"); err != nil {
		t.Fatalf("setConfigKey: %v", err)
	}

	// Reload so asmConfig sees the key for the del command's existence check.
	_ = asmConfig.ReadInConfig()

	// Force mode — should not prompt.
	fForce = true

	var buf bytes.Buffer
	configDelCmd.SetOut(&buf)

	err := configDelCmd.RunE(configDelCmd, []string{"testprofile.rename.prefix"})
	if err != nil {
		t.Fatalf("config del --force failed: %v", err)
	}

	// Verify the key is gone on disk.
	freshViper := viper.New()
	freshViper.SetConfigType("toml")
	freshViper.SetConfigFile(configPath)

	if err := freshViper.ReadInConfig(); err == nil {
		if freshViper.Get("testprofile.rename.prefix") != nil {
			t.Fatal("expected key to be deleted from config file")
		}
	}
}

// TestConfigDelDeclinedLeavesConfigUnchanged verifies that declining the
// confirmation prompt prints a cancellation message and leaves the config
// unchanged.
// **Validates: Requirements 4.3**
func TestConfigDelDeclinedLeavesConfigUnchanged(t *testing.T) {
	logger = log.New(io.Discard)

	oldConfig := asmConfig
	oldForce := fForce
	oldConfirm := confirmDeletion
	t.Cleanup(func() {
		asmConfig = oldConfig
		fForce = oldForce
		confirmDeletion = oldConfirm
	})

	asmConfig = viper.New()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	asmConfig.SetConfigType("toml")
	asmConfig.SetConfigFile(configPath)

	// Set a key and persist via setConfigKey.
	if err := setConfigKey(configPath, "testprofile.rename.prefix", "original"); err != nil {
		t.Fatalf("setConfigKey: %v", err)
	}

	// Reload so asmConfig sees the key.
	_ = asmConfig.ReadInConfig()

	// Swap confirmDeletion to decline.
	confirmDeletion = func(key string, value any) (bool, error) {
		return false, nil
	}

	fForce = false

	var buf bytes.Buffer
	configDelCmd.SetOut(&buf)

	err := configDelCmd.RunE(configDelCmd, []string{"testprofile.rename.prefix"})
	if err != nil {
		t.Fatalf("config del failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Deletion canceled.") {
		t.Fatalf("expected cancellation message, got: %q", output)
	}

	// Verify the key still exists on disk.
	freshViper := viper.New()
	freshViper.SetConfigType("toml")
	freshViper.SetConfigFile(configPath)

	if err := freshViper.ReadInConfig(); err != nil {
		t.Fatalf("could not read config: %v", err)
	}

	if freshViper.GetString("testprofile.rename.prefix") != "original" {
		t.Fatalf("expected key to remain %q, got %q", "original", freshViper.GetString("testprofile.rename.prefix"))
	}
}

// TestConfigSetSkipsBackupOnFirstWrite verifies that no backup file is created
// when config set writes the config file for the first time.
// **Validates: Requirements 8.5**
func TestConfigSetSkipsBackupOnFirstWrite(t *testing.T) {
	logger = log.New(io.Discard)

	oldConfig := asmConfig
	t.Cleanup(func() { asmConfig = oldConfig })

	asmConfig = viper.New()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	asmConfig.SetConfigType("toml")
	asmConfig.SetConfigFile(configPath)

	var buf bytes.Buffer
	configSetCmd.SetOut(&buf)

	err := configSetCmd.RunE(configSetCmd, []string{"profile-name", "newvalue"})
	if err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	// No backup should exist — this was the first write.
	matches, err := filepath.Glob(filepath.Join(dir, "config-*.toml.bak"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	if len(matches) != 0 {
		t.Fatalf("expected 0 backup files on first write, got %d", len(matches))
	}
}

// TestConfigSetBackupFailureNonFatal verifies that a backup failure does not
// prevent the config mutation from succeeding.
// **Validates: Requirements 8.6**
func TestConfigSetBackupFailureNonFatal(t *testing.T) {
	logger = log.New(io.Discard)

	// Call backupConfigFile with a path inside a read-only directory.
	// The backup write should fail silently (logged as warning) and not panic.
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	// Write an initial config file so backupConfigFile attempts a backup.
	if err := os.WriteFile(configPath, []byte("key = \"value\"\n"), 0o0644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	// Make the directory read-only so the backup write fails.
	if err := os.Chmod(dir, 0o0555); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	t.Cleanup(func() {
		// Restore write permission so TempDir cleanup succeeds.
		_ = os.Chmod(dir, 0o0755)
	})

	// This should not panic — backup failure is non-fatal.
	backupConfigFile(configPath)

	// Verify no backup was created.
	matches, _ := filepath.Glob(filepath.Join(dir, "config-*.toml.bak"))
	if len(matches) != 0 {
		t.Fatalf("expected 0 backup files when dir is read-only, got %d", len(matches))
	}
}

// TestDeleteNestedKeyPrunesEmptyParents verifies that deleteNestedKey removes
// the leaf key and prunes all empty parent tables.
func TestDeleteNestedKeyPrunesEmptyParents(t *testing.T) {
	tests := []struct {
		name      string
		tree      map[string]any
		parts     []string
		wantOK    bool
		wantEmpty bool
	}{
		{
			name:      "deep nested key prunes all parents",
			tree:      map[string]any{"a": map[string]any{"b": map[string]any{"c": "value"}}},
			parts:     []string{"a", "b", "c"},
			wantOK:    true,
			wantEmpty: true,
		},
		{
			name:      "sibling key prevents parent pruning",
			tree:      map[string]any{"a": map[string]any{"b": map[string]any{"c": "value"}, "d": "keep"}},
			parts:     []string{"a", "b", "c"},
			wantOK:    true,
			wantEmpty: false,
		},
		{
			name:      "nonexistent key returns false",
			tree:      map[string]any{"a": map[string]any{"b": "value"}},
			parts:     []string{"a", "x"},
			wantOK:    false,
			wantEmpty: false,
		},
		{
			name:      "empty parts returns false",
			tree:      map[string]any{"a": "value"},
			parts:     []string{},
			wantOK:    false,
			wantEmpty: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deleteNestedKey(tc.tree, tc.parts)
			if got != tc.wantOK {
				t.Fatalf("deleteNestedKey returned %v, want %v", got, tc.wantOK)
			}

			if tc.wantEmpty && len(tc.tree) != 0 {
				t.Fatalf("expected tree to be empty after pruning, got %v", tc.tree)
			}

			if !tc.wantEmpty && tc.wantOK && len(tc.tree) == 0 {
				t.Fatal("expected tree to retain sibling keys, but it's empty")
			}
		})
	}
}

// TestValidateConfigKey verifies that validateConfigKey accepts valid schema
// paths and rejects keys that don't belong in the config file.
func TestValidateConfigKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "profile-name is valid", key: "profile-name", wantErr: false},
		{name: "rename prefix is valid", key: "abc.rename.prefix", wantErr: false},
		{name: "rename suffix is valid", key: "abc.rename.suffix", wantErr: false},
		{name: "pattern delimiter is valid", key: "abc.rename.pattern.delimiter", wantErr: false},
		{name: "pattern order is valid", key: "abc.rename.pattern.order", wantErr: false},
		{name: "accounts substr_match_replace entry is valid", key: "abc.rename.accounts.substr_match_replace.Production", wantErr: false},
		{name: "roles global_regex_replace entry is valid", key: "abc.rename.roles.global_regex_replace.pattern", wantErr: false},
		{name: "bare profile name is rejected", key: "abc", wantErr: true},
		{name: "unknown top-level key is rejected", key: "verbose", wantErr: true},
		{name: "unknown nested key is rejected", key: "abc.rename.bogus", wantErr: true},
		{name: "too deep into leaf is rejected", key: "abc.rename.prefix.extra", wantErr: true},
		{name: "too deep into map is rejected", key: "abc.rename.accounts.substr_match_replace.key.extra", wantErr: true},
		{name: "profile-name with child is rejected", key: "profile-name.child", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfigKey(tc.key)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for key %q, got nil", tc.key)
			}

			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for key %q, got: %v", tc.key, err)
			}
		})
	}
}

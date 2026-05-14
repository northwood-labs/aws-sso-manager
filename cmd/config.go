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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/lithammer/dedent"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

var (
	fForce bool

	configCmd = &cobra.Command{
		Use:   "config",
		Short: "Manage configuration values.",
		Long:  "Read, write, and delete individual keys in the configuration file.",
		Example: strings.TrimSpace(dedent.Dedent(`
		# Add a new config entry.
		aws-sso-manager config set <key> <value>

		# Read a config entry.
		aws-sso-manager config get <key>

		# Delete a config entry.
		aws-sso-manager config del <key>
		`)),
	}

	configSetCmd = &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value.",
		Args:  cobra.ExactArgs(2),
		Example: strings.TrimSpace(dedent.Dedent(`
		# Add a new config entry.
		aws-sso-manager config set <key> <value>
		`)),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			if err := validateConfigKey(key); err != nil {
				return fmt.Errorf("invalid config key: %w", err)
			}

			configPath := asmConfig.ConfigFileUsed()

			if err := os.MkdirAll(filepath.Dir(configPath), 0o0755); err != nil {
				return fmt.Errorf("could not create config directory: %w", err)
			}

			backupConfigFile(configPath)

			if err := setConfigKey(configPath, key, value); err != nil {
				return fmt.Errorf("could not set config key: %w", err)
			}

			// Update the in-memory Viper state so subsequent reads in the
			// same process see the new value.
			asmConfig.Set(key, value)

			fmt.Fprintf(cmd.OutOrStdout(), "Set %q to %q\n", key, value)

			return nil
		},
	}

	configGetCmd = &cobra.Command{
		Use:   "get [key]",
		Short: "Get a configuration value.",
		Args:  cobra.RangeArgs(0, 1),
		Example: strings.TrimSpace(dedent.Dedent(`
		# List all config entries.
		aws-sso-manager config get

		# Read a single config entry.
		aws-sso-manager config get <key>
		`)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return printAllConfigKeys(cmd)
			}

			key := args[0]
			value := asmConfig.Get(key)

			if value == nil {
				return fmt.Errorf("key %q is not set", key)
			}

			fmt.Fprintln(cmd.OutOrStdout(), value)

			return nil
		},
	}

	configDelCmd = &cobra.Command{
		Use:   "del <key>",
		Short: "Delete a configuration value.",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(dedent.Dedent(`
		# Delete a config entry.
		aws-sso-manager config del <key>

		# Delete a config entry without confirming.
		aws-sso-manager config del --force <key>
		`)),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := asmConfig.Get(key)

			if value == nil {
				return fmt.Errorf("key %q is not set", key)
			}

			if !fForce {
				confirmed, err := confirmDeletion(key, value)
				if err != nil {
					return fmt.Errorf("confirmation prompt failed: %w", err)
				}

				if !confirmed {
					fmt.Fprintln(cmd.OutOrStdout(), "Deletion canceled.")

					return nil
				}
			}

			configPath := asmConfig.ConfigFileUsed()

			backupConfigFile(configPath)

			if err := deleteConfigKey(configPath, key); err != nil {
				return fmt.Errorf("could not delete config key: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %q\n", key)

			return nil
		},
	}

	// confirmDeletion is a test seam for the deletion confirmation prompt.
	// Tests swap this to avoid TUI interaction.
	confirmDeletion = func(key string, value any) (bool, error) {
		var confirmed bool

		err := huh.NewConfirm().
			Title(fmt.Sprintf("Delete %q (current value: %v)?", key, value)).
			Value(&confirmed).
			Run()
		if err != nil {
			return confirmed, fmt.Errorf("could not run confirmation prompt: %w", err)
		}

		return confirmed, nil
	}
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configDelCmd)

	configDelCmd.Flags().BoolVarP(&fForce, "force", "f", false, "skip confirmation prompt")
}

// printAllConfigKeys reads the persisted TOML config file, flattens it into
// dot-delimited keys, and renders a lipgloss table so the user can see all
// current settings at a glance.
func printAllConfigKeys(cmd *cobra.Command) error {
	configPath := asmConfig.ConfigFileUsed()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(cmd.OutOrStdout(), "No configuration values set.")

			return nil
		}

		return fmt.Errorf("could not read config file: %w", err)
	}

	var tree map[string]any
	if err := toml.Unmarshal(data, &tree); err != nil {
		return fmt.Errorf("could not parse config file: %w", err)
	}

	pairs := flattenConfig(tree, "")
	if len(pairs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No configuration values set.")

		return nil
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		Headers("Key", "Value").
		StyleFunc(func(row, _ int) lipgloss.Style {
			switch row {
			case table.HeaderRow:
				return headerStyle
			default:
				return cellStyle
			}
		})

	for _, kv := range pairs {
		t.Row(kv[0], kv[1])
	}

	_, err = lipgloss.Println(t)
	if err != nil {
		return fmt.Errorf("could not print table: %w", err)
	}

	return nil
}

// flattenConfig recursively walks a TOML tree and produces sorted
// dot-delimited key/value pairs. Nested tables become dotted prefixes
// (e.g., {"a": {"b": "c"}} → [["a.b", "c"]]). This gives the user a flat
// view of the entire config regardless of nesting depth. String values are
// quoted to distinguish them from other types.
func flattenConfig(tree map[string]any, prefix string) [][2]string {
	var pairs [][2]string

	for k, v := range tree {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]any:
			pairs = append(pairs, flattenConfig(val, fullKey)...)
		case string:
			pairs = append(pairs, [2]string{fullKey, fmt.Sprintf("%q", val)})
		default:
			pairs = append(pairs, [2]string{fullKey, fmt.Sprintf("%v", val)})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i][0] < pairs[j][0]
	})

	return pairs
}

// backupConfigFile copies the current config to a timestamped .bak file before
// a mutation so the user can recover from accidental changes. Failures are
// logged as warnings because atomic writes already protect against corruption.
func backupConfigFile(configPath string) {
	ctx := context.Background()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return
	}

	dir := filepath.Dir(configPath)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	backupPath := filepath.Join(dir, fmt.Sprintf("config-%s.toml.bak", timestamp))

	data, err := os.ReadFile(configPath)
	if err != nil {
		logger.WarnContext(ctx, "Could not read config for backup", logKeyErr, err)

		return
	}

	if err := os.WriteFile(backupPath, data, 0o0644); err != nil {
		logger.WarnContext(ctx, "Could not write config backup", logKeyErr, err)

		return
	}

	pruneConfigBackups(dir, 5)
}

// pruneConfigBackups keeps only the most recent `keep` backup files in dir,
// removing the oldest ones. The ISO-8601 timestamp in the filename ensures
// lexicographic sort matches chronological order.
func pruneConfigBackups(dir string, keep int) {
	ctx := context.Background()

	matches, err := filepath.Glob(filepath.Join(dir, "config-*.toml.bak"))
	if err != nil {
		logger.WarnContext(ctx, "Could not list config backups", logKeyErr, err)

		return
	}

	sort.Strings(matches)

	if len(matches) <= keep {
		return
	}

	for _, old := range matches[:len(matches)-keep] {
		if err := os.Remove(old); err != nil {
			logger.WarnContext(ctx, "Could not remove old config backup", logKeyFile, old, logKeyErr, err)
		}
	}
}

// setConfigKey writes a single key-value pair into the persisted TOML config
// file without touching any other state. This avoids the Viper WriteConfigAs
// problem where bound flags (cache-duration, verbose, etc.) leak into the
// file. We read the existing TOML, set the key in the raw tree, and write
// back atomically.
func setConfigKey(configPath, key, value string) error {
	var tree map[string]any

	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("could not read config file: %w", err)
		}

		tree = make(map[string]any)
	} else {
		if err := toml.Unmarshal(data, &tree); err != nil {
			return fmt.Errorf("could not parse config file: %w", err)
		}
	}

	setNestedKey(tree, strings.Split(key, "."), value)

	// Filter the tree through the typed schema so only valid keys are
	// persisted. This strips any stale flag keys (cache-duration, verbose,
	// etc.) that a previous Viper WriteConfigAs may have leaked.
	tree = filterConfigTree(tree)

	out, err := toml.Marshal(tree)
	if err != nil {
		return fmt.Errorf("could not marshal config: %w", err)
	}

	if err := writeAtomicBytes(configPath, out); err != nil {
		return fmt.Errorf("could not write config file: %w", err)
	}

	return nil
}

// setNestedKey walks a dot-split key path through a map tree, creating
// intermediate tables as needed, and sets the leaf value.
func setNestedKey(tree map[string]any, parts []string, value string) {
	if len(parts) == 1 {
		tree[parts[0]] = value

		return
	}

	child, ok := tree[parts[0]].(map[string]any)
	if !ok {
		child = make(map[string]any)
		tree[parts[0]] = child
	}

	setNestedKey(child, parts[1:], value)
}

// filterConfigTree removes top-level keys that don't belong in the config
// file (e.g., flag keys like "cache-duration", "verbose" that Viper's
// WriteConfigAs may have leaked in a previous run). It keeps "profile-name"
// and any key whose value is a map (SSO profile tables). This is a shallow
// filter — it doesn't recurse into SSO profiles because validateConfigKey
// already guards what can be set.
func filterConfigTree(tree map[string]any) map[string]any {
	filtered := make(map[string]any, len(tree))

	for k, v := range tree {
		if k == "profile-name" {
			filtered[k] = v

			continue
		}

		// SSO profile entries are always tables (maps). Scalar top-level
		// keys like "cache-duration" or "verbose" are flag leaks.
		if _, isMap := v.(map[string]any); isMap {
			filtered[k] = v
		}
	}

	return filtered
}

// writeAtomicBytes writes raw bytes to configPath using the temp-file-then-
// rename pattern. Extracted from writeConfigAtomic so both set and delete can
// share the same atomic write logic without going through Viper.
func writeAtomicBytes(configPath string, data []byte) error {
	dir := filepath.Dir(configPath)

	tmp, err := os.CreateTemp(dir, ".config-*.toml")
	if err != nil {
		return fmt.Errorf("could not create temporary config file: %w", err)
	}

	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)

		return fmt.Errorf("could not write config: %w", err)
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)

		return fmt.Errorf("could not close temporary config file: %w", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		_ = os.Remove(tmpPath)

		return fmt.Errorf("could not replace config file: %w", err)
	}

	return nil
}

// deleteConfigKey removes a dot-delimited key from the persisted TOML config
// file. Viper has no Delete method, so we parse the raw TOML into a map, remove
// the key, and write back atomically.
func deleteConfigKey(configPath, key string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("could not read config file: %w", err)
	}

	var tree map[string]any
	if err := toml.Unmarshal(data, &tree); err != nil {
		return fmt.Errorf("could not parse config file: %w", err)
	}

	if !deleteNestedKey(tree, strings.Split(key, ".")) {
		return fmt.Errorf("key %q not found in config file", key)
	}

	tree = filterConfigTree(tree)

	out, err := toml.Marshal(tree)
	if err != nil {
		return fmt.Errorf("could not marshal config: %w", err)
	}

	if err := writeAtomicBytes(configPath, out); err != nil {
		return fmt.Errorf("could not write config file: %w", err)
	}

	return nil
}

// deleteNestedKey walks a dot-split key path through a TOML tree and deletes
// the leaf entry. Empty parent tables are pruned after deletion so the file
// stays clean.
func deleteNestedKey(tree map[string]any, parts []string) bool {
	if len(parts) == 0 {
		return false
	}

	if len(parts) == 1 {
		if _, ok := tree[parts[0]]; !ok {
			return false
		}

		delete(tree, parts[0])

		return true
	}

	child, ok := tree[parts[0]].(map[string]any)
	if !ok {
		return false
	}

	if !deleteNestedKey(child, parts[1:]) {
		return false
	}

	// Prune empty parent tables after deletion.
	if len(child) == 0 {
		delete(tree, parts[0])
	}

	return true
}

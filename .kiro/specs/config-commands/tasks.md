# Implementation Plan: config-commands

## Overview

Implement `config set`, `config get`, and `config del` subcommands in a single file `cmd/config.go`, along with helpers for atomic writes, rolling backups, and TOML-level key deletion. Tests go in `cmd/config_test.go`. All code follows existing project conventions: flat `cmd/` package, `RunE` pattern, `fmt.Errorf` with `%w`, atomic write-to-temp-then-rename, and `pgregory.net/rapid` for property-based tests.

## Tasks

* [x] 1. Implement core command structure and `config set`
  * [x] 1.1 Create `cmd/config.go` with `configCmd` parent, `configSetCmd`, `writeConfigAtomic` helper, `backupConfigFile`, and `pruneConfigBackups`
    * Define `configCmd` (Use: `"config"`, no `RunE`, groups subcommands)
    * Define `configSetCmd` (Use: `"set <key> <value>"`, `cobra.ExactArgs(2)`, `RunE`)
    * `RunE` calls `asmConfig.Set(key, value)`, ensures parent dir via `os.MkdirAll`, calls `backupConfigFile`, then `writeConfigAtomic`
    * Prints `Set "%s" to "%s"\n` on success
    * Implement `writeConfigAtomic(configPath string) error` — temp file in same dir, `asmConfig.WriteConfigAs`, rename over target, cleanup on error
    * Implement `backupConfigFile(configPath string)` — skip if file doesn't exist, copy to `config-<20060102T150405Z>.toml.bak`, call `pruneConfigBackups`
    * Implement `pruneConfigBackups(dir string, keep int)` — glob `config-*.toml.bak`, sort, remove oldest beyond `keep`
    * Register `configCmd` under `rootCmd`, `configSetCmd` under `configCmd` in `init()`
    * _Requirements: 1.1, 1.2, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 5.1, 5.2, 5.3, 7.1, 8.1, 8.2, 8.3, 8.4, 8.5, 8.6_

  * [x] 1.2 Write property test: Set-then-get round trip
    * **Property 1: Set-then-get round trip**
    * **Validates: Requirements 2.2, 2.3, 3.2, 6.1.**

  * [x] 1.3 Write property test: Set confirmation output contains key and value
    * **Property 5: Set confirmation output contains key and value**
    * **Validates: Requirements 2.6.**

  * [x] 1.4 Write property test: Backup retention limit
    * **Property 7: Backup retention limit**
    * **Validates: Requirements 8.4.**

  * [x] 1.5 Write property test: Backup content matches pre-mutation state
    * **Property 8: Backup content matches pre-mutation state**
    * **Validates: Requirements 8.1, 8.2.**

* [x] 2. Checkpoint
  * Ensure all tests pass, ask the user if questions arise.

* [x] 3. Implement `config get`
  * [x] 3.1 Add `configGetCmd` to `cmd/config.go`
    * Define `configGetCmd` (Use: `"get <key>"`, `cobra.ExactArgs(1)`, `RunE`)
    * `RunE` reads `asmConfig.Get(key)`, returns `fmt.Errorf("key %q is not set", key)` when nil
    * Prints value via `fmt.Fprintln(cmd.OutOrStdout(), value)`
    * Register `configGetCmd` under `configCmd` in `init()`
    * _Requirements: 3.1, 3.2, 3.3, 3.4, 7.2_

  * [x] 3.2 Write property test: Nonexistent key returns error
    * **Property 4: Nonexistent key returns error**
    * **Validates: Requirements 3.3, 4.6.**

  * [x] 3.3 Write property test: Wrong argument count returns error
    * **Property 3: Wrong argument count returns error**
    * **Validates: Requirements 2.4, 3.4, 4.4.**

* [x] 4. Implement `config del` with TOML-level deletion
  * [x] 4.1 Add `configDelCmd`, `deleteConfigKey`, and `deleteNestedKey` to `cmd/config.go`
    * Define `fForce` bool and `configDelCmd` (Use: `"del <key>"`, `cobra.ExactArgs(1)`, `RunE`)
    * `RunE` checks `asmConfig.Get(key)` — returns error if nil
    * When `--force` is not set, prompt via `huh.NewConfirm()` showing current value; if declined, print `"Deletion canceled.\n"` and return nil
    * When `--force` is set, skip prompt
    * Calls `backupConfigFile`, then `deleteConfigKey`
    * Prints `Deleted "%s"\n` on success
    * Register `--force` / `-f` flag on `configDelCmd` in `init()`
    * Register `configDelCmd` under `configCmd` in `init()`
    * Implement `deleteConfigKey(configPath, key string) error` — read file, `toml.Unmarshal` into `map[string]any`, call `deleteNestedKey`, `toml.Marshal`, write atomically
    * Implement `deleteNestedKey(tree map[string]any, parts []string) bool` — recursive walk, delete leaf, prune empty parent tables
    * _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 4.8, 5.1, 5.2, 5.3, 7.3_

  * [x] 4.2 Write property test: Set-then-delete-then-get round trip
    * **Property 2: Set-then-delete-then-get round trip**
    * **Validates: Requirements 4.4, 4.5, 6.2.**

  * [x] 4.3 Write property test: Del confirmation output contains key
    * **Property 6: Del confirmation output contains key**
    * **Validates: Requirements 4.8.**

* [x] 5. Checkpoint
  * Ensure all tests pass, ask the user if questions arise.

* [x] 6. Unit tests for edge cases and structural checks
  * [x] 6.1 Write unit tests in `cmd/config_test.go`
    * Test command registration: `configCmd` under `rootCmd` with subcommands `set`, `get`, `del`
    * Test `config` with no subcommand produces help listing `set`, `get`, `del`
    * Test `config set` creates parent directory when it doesn't exist
    * Test all three subcommands use `RunE` (not `Run`)
    * Test `config del --force` skips prompt and deletes immediately
    * Test declining `config del` prompt prints cancellation message and leaves config unchanged
    * Test backup is skipped on first-time `config set` (no existing file)
    * Test backup failure is non-fatal (mutation still succeeds)
    * Test `deleteNestedKey` prunes empty parent tables
    * _Requirements: 1.1, 1.2, 2.5, 4.2, 4.3, 4.4, 7.1, 7.2, 7.3, 8.5, 8.6_

* [x] 7. Final checkpoint
  * Ensure all tests pass, ask the user if questions arise.

## Notes

* Tasks marked with `*` are optional and can be skipped for faster MVP
* All code goes in `cmd/config.go` (production) and `cmd/config_test.go` (tests)
* New shared generators for config keys/values should be added to `cmd/testhelpers_test.go`
* Property tests use `pgregory.net/rapid` with save/restore of `asmConfig` via `t.Cleanup`
* Each property test operates against a fresh Viper instance and `t.TempDir()` for the config file
* The `huh.NewConfirm` call in `config del` needs a test seam (package-level function variable) for unit testing the confirmation prompt

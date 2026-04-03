# Requirements Document

## Introduction

Add `config set`, `config get`, and `config del` subcommands to the `aws-sso-manager` CLI. These commands provide a programmatic interface for reading, writing, and deleting individual keys in the TOML configuration file at `~/.config/aws-sso-manager/config.toml`, removing the need for users to hand-edit TOML. All mutations go through the existing Viper-based config system (`asmConfig`) and persist changes atomically (write-to-temp-then-rename).

## Glossary

* **CLI**: The `aws-sso-manager` command-line application.
* **Config_File**: The TOML file at `~/.config/aws-sso-manager/config.toml` (overridable via `--config` flag).
* **Config_Key**: A dot-delimited string identifying a value in the TOML hierarchy (e.g., `profile-name`, `abc.rename.prefix`).
* **Config_Value**: The string representation of a value to store for a given Config_Key.
* **Viper_Instance**: The package-level `asmConfig` variable that merges TOML config, environment variables, and CLI flags.
* **Atomic_Write**: A write strategy where content is written to a temporary file in the same directory, then renamed over the target to prevent partial writes.
* **Config_Parent_Command**: The `config` Cobra command that groups `set`, `get`, and `del` as subcommands.
* **Config_Backup**: A copy of the Config_File created before a mutation, named `config-<ISO-8601-timestamp>.toml.bak` in the same directory as the Config_File. At most 5 backups are retained; the oldest are pruned after each mutation.

## Requirements

### Requirement 1: Config parent command

**User Story:** As a CLI user, I want a `config` parent command that groups configuration subcommands, so that the CLI namespace is organized and discoverable.

#### Acceptance Criteria

1. THE CLI SHALL register a `config` command under the root command with the use-string `config`.
2. WHEN `config` is invoked without a subcommand, THE CLI SHALL display the help text listing available subcommands (`set`, `get`, `del`).

### Requirement 2: Set a configuration value

**User Story:** As a CLI user, I want to run `config set <key> <value>` to persist a configuration value, so that I can modify my config without editing TOML by hand.

#### Acceptance Criteria

1. THE `config set` command SHALL accept exactly two positional arguments: a Config_Key and a Config_Value.
2. WHEN `config set <key> <value>` is invoked, THE CLI SHALL store the Config_Value under the Config_Key in the Viper_Instance.
3. WHEN `config set <key> <value>` is invoked, THE CLI SHALL persist the updated configuration to the Config_File using Atomic_Write.
4. IF fewer than two or more than two positional arguments are provided, THEN THE CLI SHALL return a descriptive error indicating the expected usage.
5. WHEN the Config_File parent directory does not exist, THE CLI SHALL create the directory with mode 0755 before writing.
6. WHEN `config set` completes successfully, THE CLI SHALL print a confirmation message to stdout that includes the Config_Key and Config_Value that were set.

### Requirement 3: Get a configuration value

**User Story:** As a CLI user, I want to run `config get <key>` to retrieve a configuration value, so that I can inspect my current settings from scripts or the terminal.

#### Acceptance Criteria

1. THE `config get` command SHALL accept exactly one positional argument: a Config_Key.
2. WHEN `config get <key>` is invoked and the Config_Key exists, THE CLI SHALL print the value to stdout followed by a newline.
3. WHEN `config get <key>` is invoked and the Config_Key does not exist in the Viper_Instance, THE CLI SHALL return an error indicating the key is not set.
4. IF fewer than one or more than one positional argument is provided, THEN THE CLI SHALL return a descriptive error indicating the expected usage.

### Requirement 4: Delete a configuration value

**User Story:** As a CLI user, I want to run `config del <key>` to remove a configuration value, so that I can reset a setting to its default or clean up stale entries.

#### Acceptance Criteria

1. THE `config del` command SHALL accept exactly one positional argument: a Config_Key.
2. WHEN `config del <key>` is invoked and the Config_Key exists and the `--force` flag is not set, THE CLI SHALL prompt the user for confirmation before deleting using a TUI confirm form.
3. IF the user declines the confirmation prompt, THEN THE CLI SHALL abort the deletion and print a cancellation message.
4. WHEN `config del <key>` is invoked with the `--force` flag, THE CLI SHALL skip the confirmation prompt and proceed with deletion immediately.
5. WHEN the deletion proceeds (via confirmation or `--force`), THE CLI SHALL remove the Config_Key from the persisted Config_File using Atomic_Write.
6. WHEN `config del <key>` is invoked and the Config_Key does not exist, THE CLI SHALL return an error indicating the key is not set.
7. IF fewer than one or more than one positional argument is provided, THEN THE CLI SHALL return a descriptive error indicating the expected usage.
8. WHEN `config del` completes successfully, THE CLI SHALL print a confirmation message to stdout that includes the Config_Key that was deleted.

### Requirement 5: Atomic write for config mutations

**User Story:** As a CLI user, I want config file writes to be atomic, so that an interrupted write does not leave a corrupt or empty config file.

#### Acceptance Criteria

1. WHEN the CLI persists changes to the Config_File, THE CLI SHALL write the full content to a temporary file in the same directory as the Config_File.
2. WHEN the temporary file is fully written, THE CLI SHALL rename the temporary file over the Config_File in a single OS operation.
3. IF the temporary file write fails, THEN THE CLI SHALL return an error and leave the original Config_File unchanged.

### Requirement 6: TOML round-trip fidelity

**User Story:** As a CLI user, I want `config set` followed by `config get` on the same key to return the value I set, so that I can trust the config commands are consistent.

#### Acceptance Criteria

1. FOR ALL valid Config_Key and Config_Value pairs, setting a value with `config set` then reading it with `config get` SHALL return the same Config_Value.
2. FOR ALL valid Config_Key values that exist, deleting with `config del` then reading with `config get` SHALL return an error indicating the key is not set.

### Requirement 7: Command uses RunE pattern

**User Story:** As a developer, I want all config subcommands to use the `RunE` pattern, so that errors propagate correctly through Cobra.

#### Acceptance Criteria

1. THE `config set` command SHALL use `RunE` (not `Run`) for its execution function.
2. THE `config get` command SHALL use `RunE` (not `Run`) for its execution function.
3. THE `config del` command SHALL use `RunE` (not `Run`) for its execution function.

### Requirement 8: Config file backups

**User Story:** As a CLI user, I want the tool to keep backups of my config file before mutations, so that I can recover from accidental changes.

#### Acceptance Criteria

1. BEFORE persisting a mutation to the Config_File (`config set` or `config del`), THE CLI SHALL create a Config_Backup of the current Config_File.
2. THE Config_Backup filename SHALL follow the pattern `config-<timestamp>.toml.bak` where `<timestamp>` is an ISO-8601 formatted timestamp (e.g., `config-2026-04-02T143022Z.toml.bak`).
3. THE Config_Backup SHALL be stored in the same directory as the Config_File.
4. AFTER creating a new Config_Backup, THE CLI SHALL retain at most 5 backup files, deleting the oldest backups that exceed this limit.
5. IF the Config_File does not yet exist (first-time `config set`), THEN THE CLI SHALL skip backup creation.
6. IF backup creation fails, THE CLI SHALL log a warning but SHALL NOT abort the mutation.

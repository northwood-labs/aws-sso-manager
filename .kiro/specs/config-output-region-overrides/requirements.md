# Requirements Document

## Introduction

The `update` command currently hardcodes `output = json` and `region = <sso_region>` for every generated `[profile ...]` block in `~/.aws/config`. This feature makes those values configurable per SSO profile via the existing `%.settings.output` and `%.settings.region` keys in the TOML config file, while preserving the current defaults when no override is set.

## Glossary

* **Update_Command**: The `aws-sso-manager update` CLI subcommand that regenerates managed `[profile ...]` blocks in `~/.aws/config`.
* **Config_File**: The TOML configuration file at `~/.config/aws-sso-manager/config.toml`, read via the `asmConfig` Viper instance.
* **SSO_Profile**: A user-defined identifier (e.g., `nwl`, `abc`) that corresponds to an `[sso-session ...]` block and a top-level TOML key.
* **Managed_Section**: The region of `~/.aws/config` between start/end markers that the Update_Command owns and regenerates on every run.
* **SettingsConfig**: The Go struct (`cmd/configschema.go`) that models the `%.settings` TOML table, containing `Region` and `Output` fields.

## Requirements

### Requirement 1: Region Override from Config File

**User Story:** As a user managing multiple AWS Organizations, I want to set a default region per SSO profile in the TOML config file, so that generated profiles use my preferred region instead of always inheriting `sso_region`.

#### Acceptance Criteria

1. WHEN the Config_File contains a non-empty value for `<sso-profile>.settings.region`, THE Update_Command SHALL use that value as the `region` field in every generated `[profile ...]` block for that SSO_Profile.
2. WHEN the Config_File does not contain a value for `<sso-profile>.settings.region`, THE Update_Command SHALL fall back to the `sso_region` value from the `[sso-session ...]` section.
3. WHEN the Config_File contains an empty string for `<sso-profile>.settings.region`, THE Update_Command SHALL fall back to the `sso_region` value from the `[sso-session ...]` section.

### Requirement 2: Output Override from Config File

**User Story:** As a user who prefers a specific AWS CLI output format, I want to set a default output format per SSO profile in the TOML config file, so that generated profiles use my preferred format instead of always using `json`.

#### Acceptance Criteria

1. WHEN the Config_File contains a non-empty value for `<sso-profile>.settings.output`, THE Update_Command SHALL use that value as the `output` field in every generated `[profile ...]` block for that SSO_Profile.
2. WHEN the Config_File does not contain a value for `<sso-profile>.settings.output`, THE Update_Command SHALL use `json` as the `output` field.
3. WHEN the Config_File contains an empty string for `<sso-profile>.settings.output`, THE Update_Command SHALL use `json` as the `output` field.

### Requirement 3: Config Key Validation for Settings Keys

**User Story:** As a user, I want the `config set` command to accept `%.settings.region` and `%.settings.output` as valid keys, so that I can configure overrides without errors.

#### Acceptance Criteria

1. WHEN a user calls `config set` with a key matching `<profile>.settings.region`, THE Config_File validator SHALL accept the key as valid.
2. WHEN a user calls `config set` with a key matching `<profile>.settings.output`, THE Config_File validator SHALL accept the key as valid.
3. WHEN a user calls `config set` with a key matching `<profile>.settings.<unknown>`, THE Config_File validator SHALL reject the key with a descriptive error.

### Requirement 4: Override Consistency Across All Profiles in a Managed Section

**User Story:** As a user, I want the region and output overrides to apply uniformly to every generated profile within a single SSO profile's managed section, so that all profiles under one organization share the same defaults.

#### Acceptance Criteria

1. WHILE the Update_Command is generating profiles for a single SSO_Profile, THE Update_Command SHALL apply the same resolved `region` value to every generated `[profile ...]` block in that Managed_Section.
2. WHILE the Update_Command is generating profiles for a single SSO_Profile, THE Update_Command SHALL apply the same resolved `output` value to every generated `[profile ...]` block in that Managed_Section.

### Requirement 5: Round-Trip Stability of Settings Overrides

**User Story:** As a user, I want repeated `update` runs with the same config to produce identical output, so that my config file remains stable and predictable.

#### Acceptance Criteria

1. FOR ALL valid Config_File settings, running the Update_Command twice with the same inputs SHALL produce identical Managed_Section content (idempotence).

### Requirement 6: Documentation of Settings Configuration Keys

**User Story:** As a user, I want the `docs/config_file.md` documentation to describe the `%.settings.region` and `%.settings.output` configuration keys, so that I can discover and understand the available overrides without reading source code.

#### Acceptance Criteria

1. THE Documentation SHALL include a `%.settings.region` entry in the TOML key tree diagram under the `%.` node, as a sibling of `rename.`.
2. THE Documentation SHALL include a `%.settings.output` entry in the TOML key tree diagram under the `%.` node, as a sibling of `rename.`.
3. THE Documentation SHALL contain a `%.settings.region` section describing the key as a per-SSO-profile override for the `region` field in generated `[profile ...]` blocks, with the fallback behavior to `sso_region` when the value is empty or absent.
4. THE Documentation SHALL contain a `%.settings.output` section describing the key as a per-SSO-profile override for the `output` field in generated `[profile ...]` blocks, with the fallback behavior to `json` when the value is empty or absent.
5. THE Documentation SHALL include a `[%.settings]` table in the sample config that demonstrates setting `region` and `output` values.

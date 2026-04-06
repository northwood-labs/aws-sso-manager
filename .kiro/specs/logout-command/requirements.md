# Requirements Document

## Introduction

The CLI currently supports authenticating with AWS SSO via the `auth` command, which creates an on-disk cache file containing OIDC credentials. There is no way to explicitly end a session by removing those cached credentials. This feature adds a `logout` command that resolves the SSO profile (via argument, config, or interactive prompt) and deletes the corresponding cache file from disk.

## Glossary

* **CLI**: The `aws-sso-manager` command-line application.
* **Logout_Command**: The new `logout` Cobra subcommand registered on the root command.
* **SSO_Profile_Name**: The identifier for an `[sso-session <name>]` section in the AWS_Config_File, provided as a positional argument, via Viper config, or via the interactive prompt.
* **SSO_Session_Profile**: The `ssoProfile` struct resolved from the SSO_Profile_Name via `getSsoSession`.
* **Cache_File**: The JSON file on disk (path resolved via `getCacheFilePath`) containing `cacheFileData` credentials for an authenticated SSO session.
* **AWS_Config_File**: The AWS shared configuration file, typically located at `~/.aws/config`.
* **Profile_Prompt**: The `promptProfileSelect` interactive TUI element that collects the SSO_Profile_Name when one is not provided via argument or configuration.

## Requirements

### Requirement 1: Register the logout command

**User Story:** As a CLI user, I want a `logout` command available in the CLI, so that I can explicitly end an SSO session.

#### Acceptance Criteria

1. THE CLI SHALL register a `logout` command on the root command with the usage pattern `logout [sso-profile-name]`.
2. THE Logout_Command SHALL accept zero or one positional arguments.
3. THE Logout_Command SHALL use `RunE` to return errors to the Cobra framework.

### Requirement 2: Resolve the SSO profile name

**User Story:** As a CLI user, I want the logout command to resolve the SSO profile the same way the auth command does, so that the experience is consistent across commands.

#### Acceptance Criteria

1. WHEN one positional argument is provided, THE Logout_Command SHALL use that argument as the SSO_Profile_Name.
2. WHEN no positional argument is provided and a profile name is set in the Viper configuration, THE Logout_Command SHALL use the configured value as the SSO_Profile_Name.
3. WHEN no positional argument is provided and no profile name is configured, THE Logout_Command SHALL invoke the Profile_Prompt to collect the SSO_Profile_Name interactively.

### Requirement 3: Resolve the cache file path

**User Story:** As a CLI user, I want the logout command to locate the correct cache file for my SSO profile, so that the right credentials are removed.

#### Acceptance Criteria

1. WHEN a valid SSO_Profile_Name is resolved, THE Logout_Command SHALL call `getSsoSession` to obtain the SSO_Session_Profile.
2. WHEN the SSO_Session_Profile is obtained, THE Logout_Command SHALL call `getCacheFilePath` to determine the Cache_File location.
3. IF `getSsoSession` returns an error, THEN THE Logout_Command SHALL return that error without attempting file deletion.
4. IF `getCacheFilePath` returns an error, THEN THE Logout_Command SHALL return that error without attempting file deletion.

### Requirement 4: Delete the cache file

**User Story:** As a CLI user, I want the logout command to delete the cached credentials file, so that my SSO session is effectively ended.

#### Acceptance Criteria

1. WHEN the Cache_File path is resolved, THE Logout_Command SHALL delete the Cache_File from disk.
2. WHEN the Cache_File is deleted, THE Logout_Command SHALL print a confirmation message that includes the SSO_Profile_Name.
3. IF the Cache_File does not exist on disk, THEN THE Logout_Command SHALL print a message indicating no active session was found for the SSO_Profile_Name and return without error.
4. IF the file system returns a permission or I/O error during deletion, THEN THE Logout_Command SHALL return a descriptive error wrapping the underlying cause.

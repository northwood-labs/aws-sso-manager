# Requirements Document

## Introduction

AWS SSO Manager is a Go CLI tool that manages AWS Identity Center (formerly AWS SSO) profiles stored in `~/.aws/config`. It automates the setup, authentication, listing, updating, and validation of SSO-based AWS CLI profiles, enabling seamless use with `$AWS_PROFILE`, AWS Vault, and AWS SDKs. The tool supports configurable profile naming, multi-layer caching, file locking for concurrent access safety, and interactive TUI prompts.

## Glossary

* **CLI**: The `aws-sso-manager` command-line interface application
* **AWS_Config_File**: The AWS shared configuration file at `~/.aws/config`
* **SSO_Profile**: A user-defined short identifier for an AWS Organization's SSO session (e.g., `nwl`, `abc`)
* **SSO_Session_Section**: An INI section in the AWS_Config_File with the header `[sso-session <SSO_Profile>]`
* **Managed_Block**: A region of the AWS_Config_File delimited by start and end comment markers of the form `; -------- aws-sso-manager: start <SSO_Profile> --------` and `; -------- aws-sso-manager: end <SSO_Profile> --------`
* **Config_File**: The TOML configuration file at `~/.config/aws-sso-manager/config.toml` controlling profile naming and default settings
* **Auth_Cache**: The SSO access token cache stored in the standard AWS SDK cache location, keyed by SSO_Profile name
* **Accounts_Cache**: A JSON file in `~/.config/aws-sso-manager/cache/` storing the list of AWS accounts and roles for a given SSO_Profile
* **Lookup_Index**: A JSON file derived from the Accounts_Cache providing case-insensitive lookups by account ID, account name, and profile name
* **Profile_Name_Pattern**: A configurable naming pattern for generated AWS CLI profile names, composed of ordered tokens (PREFIX, ACCOUNT, ROLE, SUFFIX) joined by a delimiter
* **Device_Authorization_Flow**: The OAuth 2.0 device authorization grant flow used by AWS SSO OIDC for browser-based authentication
* **TUI**: Terminal User Interface components (forms, spinners, tables) provided by Charmbracelet libraries
* **File_Lock**: An exclusive advisory lock (`flock`) on `~/.aws/.aws-sso-manager.config.lock` used to prevent concurrent modifications to the AWS_Config_File

## Requirements

### Requirement 1: SSO profile initialization

**User Story:** As a developer, I want to initialize an SSO configuration for an AWS Organization, so that I can set up my AWS CLI config for SSO-based authentication.

#### Acceptance criteria

1. WHEN the user runs `init` with an SSO_Profile argument, THE CLI SHALL use that argument as the SSO_Profile name
2. WHEN the user runs `init` without an SSO_Profile argument and a `profile-name` value exists in the Config_File, THE CLI SHALL use the configured default as the SSO_Profile name
3. WHEN the user runs `init` without an SSO_Profile argument and no default is configured, THE CLI SHALL prompt the user interactively for the SSO_Profile name using a TUI input form
4. THE CLI SHALL collect the SSO start URL, SSO region, and SSO registration scopes for the SSO_Profile
5. WHEN the SSO start URL is a bare subdomain (no dots, no slashes), THE CLI SHALL normalize the SSO start URL to `https://<subdomain>.awsapps.com/start`
6. WHEN the SSO start URL contains a dot but no scheme, THE CLI SHALL prepend `https://` and append `/start`
7. WHEN the SSO start URL contains a slash but no scheme, THE CLI SHALL prepend `https://`
8. IF the AWS_Config_File already contains an SSO_Session_Section for the given SSO_Profile, THEN THE CLI SHALL return an error instructing the user to delete the existing section
9. IF the AWS_Config_File already contains Managed_Block markers for the given SSO_Profile without a corresponding SSO_Session_Section, THEN THE CLI SHALL return an error about orphaned markers
10. THE CLI SHALL acquire a File_Lock before modifying the AWS_Config_File
11. THE CLI SHALL write the new SSO_Session_Section inside a Managed_Block appended to the AWS_Config_File using an atomic file operation (write to temp file, then rename)
12. THE CLI SHALL release the File_Lock after the write operation completes

### Requirement 2: SSO authentication

**User Story:** As a developer, I want to authenticate with AWS Identity Center, so that I can obtain temporary credentials for AWS CLI and SDK usage.

#### Acceptance criteria

1. WHEN the user runs `auth` with an SSO_Profile argument, THE CLI SHALL use that argument as the SSO_Profile name
2. WHEN the user runs `auth` without an SSO_Profile argument and a default is configured, THE CLI SHALL use the configured default
3. WHEN the user runs `auth` without an SSO_Profile argument and no default is configured, THE CLI SHALL prompt the user interactively for the SSO_Profile name
4. THE CLI SHALL support the alias `login` for the `auth` command
5. THE CLI SHALL read the SSO_Session_Section from the AWS_Config_File for the given SSO_Profile
6. IF a valid (non-expired) Auth_Cache exists for the SSO_Profile, THEN THE CLI SHALL report the remaining validity duration and skip re-authentication
7. WHEN no valid Auth_Cache exists, THE CLI SHALL initiate the Device_Authorization_Flow by registering a public OIDC client and starting device authorization
8. WHEN the `--browser` flag is true (default), THE CLI SHALL open the verification URL in the default web browser
9. WHEN the `--browser` flag is false, THE CLI SHALL print the verification URL to stdout
10. THE CLI SHALL display the user verification code so the user can confirm the code matches the browser prompt
11. THE CLI SHALL poll the OIDC token endpoint with a 2-second interval for up to 60 seconds waiting for user approval
12. IF the user denies the authorization request, THEN THE CLI SHALL return an authentication denied error
13. IF the token expires before user approval, THEN THE CLI SHALL return a token expired error
14. WHEN the user approves the authorization, THE CLI SHALL save the access token, client credentials, and expiration timestamps to the Auth_Cache
15. WHEN another command (list, update, console) requires authentication and the Auth_Cache is missing or expired, THE CLI SHALL automatically trigger the authentication flow before proceeding

### Requirement 3: account and role listing

**User Story:** As a developer, I want to list all AWS accounts and roles available through my SSO session, so that I can see what access I have.

#### Acceptance criteria

1. WHEN the user runs `list` with an SSO_Profile argument, THE CLI SHALL use that argument as the SSO_Profile name
2. WHEN the user runs `list` without an SSO_Profile argument and a default is configured, THE CLI SHALL use the configured default
3. WHEN the user runs `list` without an SSO_Profile argument and no default is configured, THE CLI SHALL prompt the user interactively for the SSO_Profile name
4. THE CLI SHALL support the alias `ls` for the `list` command
5. THE CLI SHALL display a TUI spinner on stderr while fetching accounts and roles from the AWS SSO API
6. THE CLI SHALL paginate through all accounts and all roles per account using the AWS SSO ListAccounts and ListAccountRoles APIs
7. THE CLI SHALL sort accounts alphabetically by name (case-insensitive) and roles alphabetically by name (case-insensitive) within each account
8. WHEN no output format flag is specified, THE CLI SHALL render results in a styled TUI table with rounded borders showing columns: ID, Account Name, Role Name, Profile Name
9. WHEN the `--json` flag is set, THE CLI SHALL output the accounts and roles as a JSON object
10. WHEN the `--csv` flag is set, THE CLI SHALL output the accounts and roles as a CSV table with quoted fields
11. WHEN the `--markdown` flag is set, THE CLI SHALL output the accounts and roles as a GitHub-Flavored Markdown table
12. IF more than one output format flag is set, THEN THE CLI SHALL return an error stating only one format may be chosen
13. WHEN the `--accounts` flag is set with a substring, THE CLI SHALL filter accounts to those whose name contains the substring (case-insensitive)
14. WHEN the `--roles` flag is set with a substring, THE CLI SHALL filter roles to those whose name contains the substring (case-insensitive)
15. WHEN no accounts are assigned to the user, THE CLI SHALL print "No AWS accounts are assigned to this user." and exit with code 0
16. THE CLI SHALL cache the fetched account and role data in the Accounts_Cache with a timestamp
17. WHEN a valid (non-expired) Accounts_Cache exists, THE CLI SHALL use the cached data instead of calling the AWS SSO API
18. WHEN the `--no-cache` flag is set, THE CLI SHALL fetch fresh data (to ensure it is available) before deleting the existing Accounts_Cache and Lookup_Index
19. THE CLI SHALL build and persist a Lookup_Index alongside the Accounts_Cache when no account or role filters are applied

### Requirement 4: AWS config update

**User Story:** As a developer, I want to synchronize my AWS CLI config with the current set of accounts and roles from AWS Identity Center, so that my local profiles stay up to date.

#### Acceptance criteria

1. WHEN the user runs `update` with an SSO_Profile argument, THE CLI SHALL use that argument as the SSO_Profile name
2. WHEN the user runs `update` without an SSO_Profile argument and a default is configured, THE CLI SHALL use the configured default
3. WHEN the user runs `update` without an SSO_Profile argument and no default is configured, THE CLI SHALL prompt the user interactively for the SSO_Profile name
4. THE CLI SHALL support the aliases `upgrade` and `sync` for the `update` command
5. THE CLI SHALL acquire a File_Lock before modifying the AWS_Config_File
6. THE CLI SHALL extract the existing Managed_Block content for the SSO_Profile from the AWS_Config_File
7. THE CLI SHALL validate that the Managed_Block markers are well-formed before proceeding
8. THE CLI SHALL fetch the current accounts and roles from the AWS SSO API (with caching)
9. THE CLI SHALL rebuild the Managed_Block from scratch, generating a `[profile <name>]` section for each account-role combination with keys: `sso_session`, `sso_account_id`, `sso_role_name`, `region`, and `output`
10. THE CLI SHALL generate profile names using the Profile_Name_Pattern from the Config_File
11. THE CLI SHALL replace the old Managed_Block content in the AWS_Config_File with the newly generated content using an atomic file operation (write to temp file, set permissions to 0644, then rename)
12. THE CLI SHALL release the File_Lock after the write operation completes
13. THE CLI SHALL print the count of updated profiles and the SSO_Profile name upon completion

### Requirement 5: console URL generation

**User Story:** As a developer, I want to generate AWS Console URLs with a pre-selected account and role, so that I can quickly access the AWS Console for a specific context.

#### Acceptance criteria

1. WHEN the user runs `console` with two arguments, THE CLI SHALL treat the first as the SSO_Profile and the second as the console URL
2. WHEN the user runs `console` with one argument that contains `://`, THE CLI SHALL treat the argument as the console URL
3. WHEN the user runs `console` with one argument that does not contain `://`, THE CLI SHALL treat the argument as the SSO_Profile
4. WHEN the user runs `console` with no arguments, THE CLI SHALL use the configured default SSO_Profile if available
5. WHEN the console URL is not provided and `--clipboard` is true (default), THE CLI SHALL attempt to read the console URL from the system clipboard if the clipboard content contains `console.aws.amazon.com`
6. WHEN the SSO_Profile is not determined, THE CLI SHALL prompt the user to select from available managed SSO profiles using a TUI select form
7. WHEN the account ID is not provided via `--account-id`, THE CLI SHALL prompt the user to select from available accounts using a TUI select form
8. WHEN the role is not provided via `--role`, THE CLI SHALL prompt the user to select from available roles for the chosen account using a TUI select form
9. THE CLI SHALL strip the account ID subdomain from the console URL before encoding the destination
10. THE CLI SHALL generate a final URL in the format `https://<start-host>/start/#/console?account_id=<id>&role_name=<role>&destination=<encoded-url>`
11. WHEN the `--clipboard` flag is true, THE CLI SHALL copy the generated URL to the system clipboard
12. THE CLI SHALL print the generated URL to stdout
13. WHEN the `--region` flag is set, THE CLI SHALL use the specified region for AWS API calls instead of the SSO session region

### Requirement 6: Shell-Friendly data retrieval

**User Story:** As a developer, I want to retrieve account IDs and role names as line-delimited output, so that I can pipe them to tools like fzf for interactive selection.

#### Acceptance criteria

1. WHEN the user runs `get accounts`, THE CLI SHALL print one account ID per line from the Lookup_Index, sorted numerically
2. WHEN the user runs `get roles --for <account-id>`, THE CLI SHALL print one role name per line for the specified account, sorted alphabetically (case-insensitive)
3. IF the `--for` flag value is not a 12-digit numeric string, THEN THE CLI SHALL return an error indicating the expected format
4. IF the account ID is not found in the Lookup_Index, THEN THE CLI SHALL return an error indicating the account was not found
5. THE CLI SHALL resolve the SSO_Profile from the `--profile` flag or the Config_File default
6. WHEN the user does not provide an SSO_Profile argument and a default is configured, THE CLI SHALL use the configured default
7. IF no SSO_Profile can be resolved, THEN THE CLI SHALL return an error instructing the user to set a profile
8. THE CLI SHALL read from the Lookup_Index cache, building it from the Accounts_Cache if the Lookup_Index is missing

### Requirement 7: account and role lookup

**User Story:** As a developer, I want to look up account details and search for roles by substring, so that I can quickly find specific accounts and roles from my local cache.

#### Acceptance criteria

1. WHEN the user runs `lookup account <identifier>`, THE CLI SHALL resolve the identifier against the Lookup_Index by matching account ID, profile name (case-insensitive), or account name (case-insensitive)
2. WHEN the identifier matches exactly one account and `--json` is not set, THE CLI SHALL print the account ID
3. WHEN the identifier matches exactly one account and `--json` is set, THE CLI SHALL print a JSON object containing the SSO_Profile, account ID, account name, profile names, and role names
4. IF the identifier matches multiple accounts, THEN THE CLI SHALL return an error listing the ambiguous account IDs
5. IF the identifier matches no accounts, THEN THE CLI SHALL return an error stating the identifier was not found
6. WHEN the user runs `lookup role <substring> --for <identifier>`, THE CLI SHALL resolve the account identifier and search for roles containing the substring (case-insensitive)
7. WHEN matching roles are found and `--json` is not set, THE CLI SHALL print one matching role name per line, sorted alphabetically (case-insensitive)
8. WHEN matching roles are found and `--json` is set, THE CLI SHALL print a JSON object containing the SSO_Profile, account ID, account name, query string, and matching role names
9. IF no roles match the substring, THEN THE CLI SHALL return an error stating no roles matched
10. IF the `--for` flag is not provided for `lookup role`, THEN THE CLI SHALL return an error stating the flag is required
11. THE CLI SHALL resolve the SSO_Profile from the `--profile` flag or the Config_File default

### Requirement 8: configuration validation

**User Story:** As a developer, I want to validate the integrity of managed block markers in my AWS config file, so that I can detect and fix configuration problems before they cause update failures.

#### Acceptance criteria

1. THE CLI SHALL support the aliases `check` and `lint` for the `validate` command
2. THE CLI SHALL inspect the AWS_Config_File for all managed block start and end markers
3. THE CLI SHALL check that each SSO_Profile has exactly one start marker and one matching end marker
4. THE CLI SHALL detect duplicate managed blocks for the same SSO_Profile
5. THE CLI SHALL detect overlapping managed blocks (a start marker for one profile inside another profile's block)
6. THE CLI SHALL detect unmatched end markers (end marker without a preceding start marker)
7. THE CLI SHALL detect unclosed managed blocks (start marker without a following end marker)
8. THE CLI SHALL check that each marked SSO_Profile has a corresponding SSO_Session_Section in the AWS_Config_File
9. THE CLI SHALL report SSO_Session_Sections that exist without corresponding managed block markers
10. WHEN all checks pass, THE CLI SHALL print "OK" for each profile, a summary count, and exit with code 0
11. WHEN any check fails, THE CLI SHALL print "FAIL" for the affected profile with details and exit with code 1
12. WHEN no managed profiles are found, THE CLI SHALL inform the user and suggest running `init`

### Requirement 9: profile name generation

**User Story:** As a developer, I want to customize how AWS CLI profile names are generated, so that I can use short, meaningful names that fit my workflow across multiple AWS Organizations.

#### Acceptance criteria

1. THE CLI SHALL generate profile names using the Profile_Name_Pattern defined in the Config_File under `<SSO_Profile>.rename.pattern.order`
2. THE CLI SHALL support the pattern tokens PREFIX, ACCOUNT, ROLE, and SUFFIX in any order
3. THE CLI SHALL join pattern tokens using the delimiter defined in `<SSO_Profile>.rename.pattern.delimiter` (default: `-`)
4. THE CLI SHALL substitute the PREFIX token with the value from `<SSO_Profile>.rename.prefix`
5. THE CLI SHALL substitute the SUFFIX token with the value from `<SSO_Profile>.rename.suffix`
6. WHEN a substring match rule in `<SSO_Profile>.rename.accounts.substr_match_replace` matches the account name (case-insensitive), THE CLI SHALL replace the entire account token with the replacement value
7. WHEN a substring match rule replacement value is empty, THE CLI SHALL omit the account token from the profile name
8. WHEN a substring match rule in `<SSO_Profile>.rename.roles.substr_match_replace` matches the role name (case-insensitive), THE CLI SHALL replace the entire role token with the replacement value
9. WHEN a substring match rule replacement value is empty, THE CLI SHALL omit the role token from the profile name
10. WHEN no pattern order is configured, THE CLI SHALL fall back to a default profile name of `<account-token>-<role-token>` where tokens are lowercased and non-alphanumeric characters are replaced with hyphens
11. WHEN empty tokens cause the generated name to be empty, THE CLI SHALL fall back to the default profile name generation
12. THE CLI SHALL omit PREFIX and SUFFIX tokens from the pattern when their values are empty strings

### Requirement 10: caching system

**User Story:** As a developer, I want account and role data to be cached locally, so that repeated commands are fast and do not require network calls.

#### Acceptance criteria

1. THE CLI SHALL cache account and role data in JSON files under `~/.config/aws-sso-manager/cache/`
2. THE CLI SHALL derive cache file names from a SHA-256 hash of the SSO_Profile name and any active filters
3. THE CLI SHALL store a `cached_at` timestamp in each cache file
4. THE CLI SHALL consider a cache entry expired when the elapsed time since `cached_at` exceeds the cache duration
5. THE CLI SHALL use a default cache duration of 24 hours
6. WHEN the `--cache-duration` global flag is set, THE CLI SHALL use the specified duration (supporting Go duration syntax plus a `d` suffix for days)
7. IF the cache duration value is empty, THEN THE CLI SHALL return an error
8. IF the cache duration value is zero or negative, THEN THE CLI SHALL return an error
9. WHEN a cache entry expires, THE CLI SHALL delete the expired cache file and fetch fresh data
10. THE CLI SHALL write cache files atomically (write to `.tmp` file, then rename)
11. THE CLI SHALL build and persist a Lookup_Index alongside the Accounts_Cache when no filters are applied
12. THE CLI SHALL set cache file permissions to 0600

### Requirement 11: file locking

**User Story:** As a developer, I want concurrent access to the AWS config file to be safe, so that simultaneous runs of the tool do not corrupt my configuration.

#### Acceptance criteria

1. THE CLI SHALL acquire an exclusive advisory file lock at `~/.config/.aws-sso-manager/.config.lock` before any write operation to the AWS_Config_File
2. THE CLI SHALL use non-blocking lock acquisition with a retry interval of 100 milliseconds
3. THE CLI SHALL time out lock acquisition after 5 seconds
4. IF the lock cannot be acquired within the timeout, THEN THE CLI SHALL return a timeout error
5. THE CLI SHALL write the current process ID to the lock file after acquiring the lock
6. THE CLI SHALL release the lock after the write operation completes, including in error paths (via deferred release)
7. IF the lock file directory does not exist, THEN THE CLI SHALL create the directory with permissions 0755
8. THE CLI SHALL set lock file permissions to 0600

### Requirement 12: global CLI configuration

**User Story:** As a developer, I want to configure global settings for the CLI tool, so that I can customize its behavior across all commands.

#### Acceptance criteria

1. THE CLI SHALL read configuration from a TOML file at the path specified by the `--config` flag (default: `~/.config/aws-sso-manager/config.toml`)
2. WHEN the default config file does not exist, THE CLI SHALL create the config directory and an empty config file
3. IF a non-default config file path is specified and the file does not exist, THEN THE CLI SHALL return an error
4. THE CLI SHALL support environment variables with the prefix `ASM_` that override config file values
5. THE CLI SHALL support the `--verbose` flag which is stackable: one `-v` sets info level, two `-vv` sets debug level, three or more sets debug level with the original filename and line number of the log message
6. THE CLI SHALL log to stderr with timestamps and caller information when verbosity is enabled
7. THE CLI SHALL use the `profile-name` config key as the default SSO_Profile when no profile argument is provided to a command

### Requirement 13: version information

**User Story:** As a developer, I want to check the version of the tool, so that I can verify I am running the expected release.

#### Acceptance criteria

1. WHEN the user runs `version`, THE CLI SHALL display version information

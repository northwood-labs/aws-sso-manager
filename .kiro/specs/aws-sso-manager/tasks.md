# Implementation Plan: AWS SSO Manager

## Overview

This plan covers four areas: (1) intentional behavior changes from the requirements that differ from current code, (2) property-based tests using `pgregory.net/rapid`, (3) unit test coverage for existing functionality, and (4) test infrastructure setup. The existing codebase is working; tasks focus on changes and test coverage, not re-implementation.

## Tasks

* [x] 1. Add test dependencies and shared test helpers
  * [x] 1.1 Add `pgregory.net/rapid` dependency to `go.mod`
    * Run `go get pgregory.net/rapid` to add the property-based testing library
    * _Requirements: Design Testing Strategy_
  * [x] 1.2 Create shared test generators in `cmd/testhelpers_test.go`
    * Create `genListAccount()` rapid generator producing random `listAccount` structs with valid IDs (12-digit), names, emails, and 1-5 roles
    * Create `genListAccounts(minAccounts, maxAccounts)` rapid generator producing `listAccounts` with sorted accounts/roles
    * Create `genLookupIndex()` rapid generator producing `listAWSAccountsLookupIndex` from random accounts
    * Create `genManagedBlockConfig(profiles []string)` rapid generator producing well-formed and malformed AWS config file content with managed block markers
    * Create `genProfilePatternConfig()` rapid generator producing random Viper config with pattern order, delimiter, prefix, suffix, and substr_match_replace maps
    * _Requirements: Design Testing Strategy_

* [x] 2. Implement behavior change: `--no-cache` fetch-then-delete ordering
  * [x] 2.1 Modify `list` command `--no-cache` logic in `cmd/list.go`
    * Currently `deleteListAWSAccountsCache` is called before the spinner/fetch. Change the order so that fresh data is fetched first (via the spinner), then the old cache files are deleted, then the fresh data is written to cache
    * Ensure the new flow: fetch → delete old cache → write new cache
    * _Requirements: 3.18_
  * [x] 2.2 Write unit test for `--no-cache` fetch-then-delete ordering
    * Test that when `--no-cache` is set, the fetcher is called even when a valid cache exists, and the old cache is replaced with fresh data
    * _Requirements: 3.18_

* [x] 3. Implement behavior change: lock file path
  * [x] 3.1 Change lock file path in `cmd/lockutils.go`
    * Change `lockPath` from `filepath.Join(lockDir, ".aws-sso-manager.config.lock")` (where `lockDir` is `filepath.Dir(awsConfigFilePath)`, i.e., `~/.aws/`) to `~/.config/.aws-sso-manager/.config.lock`
    * Create the lock directory `~/.config/.aws-sso-manager/` with permissions 0755 if it does not exist
    * Set lock file permissions to 0600
    * _Requirements: 11.1, 11.7, 11.8_
  * [x] 3.2 Write unit test for new lock file path
    * Verify the lock file is created at the new path `~/.config/.aws-sso-manager/.config.lock`
    * Verify the lock directory is created with 0755 permissions
    * Verify the lock file has 0600 permissions
    * Update existing `TestAcquireAWSConfigLockCreatesMissingDirectory` to reflect the new path
    * _Requirements: 11.1, 11.7, 11.8_

* [x] 4. Implement behavior change: environment variable prefix
  * [x] 4.1 Change env prefix from `ASV_` to `ASM_` in `cmd/root.go`
    * In `initializeConfig`, change `asvConfig.SetEnvPrefix("ASV")` to `asvConfig.SetEnvPrefix("ASM")`
    * _Requirements: 12.4_
  * [x] 4.2 Write unit test for `ASM_` env prefix
    * Set an `ASM_PROFILE_NAME` environment variable and verify Viper reads it correctly
    * Verify `ASV_` prefixed variables are no longer read
    * _Requirements: 12.4_

* [x] 5. Implement behavior change: verbose debug level with caller info
  * [x] 5.1 Update verbose flag handling in `cmd/root.go` `PersistentPreRunE`
    * Change the `switch` so that `fVerbose >= 3` sets `log.DebugLevel` and enables `ReportCaller` with the original filename and line number
    * Keep `fVerbose == 2` as `log.DebugLevel` without original filename/line number
    * _Requirements: 12.5_
  * [x] 5.2 Write unit test for verbose levels
    * Test that `-v` sets info, `-vv` sets debug, `-vvv` sets debug with `ReportCaller` enabled showing original filename and line number
    * _Requirements: 12.5_

* [x] 6. Checkpoint - Verify behavior changes
  * Ensure all tests pass, ask the user if questions arise.

* [x] 7. Property-based tests: URL normalization and data formatting
  * [x] 7.1 Write property test for SSO Start URL normalization (Property 1)
    * **Property 1: SSO Start URL Normalization Round Trip**
    * Generate random bare subdomains, dot-containing strings, slash-containing strings, and full URLs; verify output starts with `https://` and bare subdomains end with `.awsapps.com/start`; verify output is parseable by `url.Parse`
    * Add in `cmd/init_test.go`
    * **Validates: Requirements 1.5, 1.6, 1.7**
  * [x] 7.2 Write property test for account and role sorting (Property 2)
    * **Property 2: Account and Role Sorting**
    * Generate random `listAccounts`; sort using the same `sort.SliceStable` logic; verify accounts are sorted by name (CI) and roles within each account are sorted by name (CI)
    * Add in `cmd/awsutils_test.go`
    * **Validates: Requirements 3.7**
  * [x] 7.3 Write property test for output formats containing all data (Property 3)
    * **Property 3: Output Formats Contain All Data**
    * Generate random `listAccounts` and profile name; render CSV via `renderCSVTable`, markdown via `renderMarkdownTable`, and JSON via `json.Marshal`; verify every account ID, name, role name, and profile name appears in each output
    * Add in `cmd/list_test.go`
    * **Validates: Requirements 3.9, 3.10, 3.11**
  * [x] 7.4 Write property test for account and role filtering (Property 4)
    * **Property 4: Account and Role Filtering**
    * Generate random accounts and a non-empty filter substring; apply case-insensitive substring filtering; verify the result is a subset of the original and every result contains the filter substring
    * Add in `cmd/awsutils_test.go`
    * **Validates: Requirements 3.13, 3.14**

* [x] 8. Property-based tests: Lookup and cache
  * [x] 8.1 Write property test for lookup index round trip (Property 5)
    * **Property 5: Lookup Index Round Trip**
    * Generate random `listAccounts` and profile name; build `listAWSAccountsLookupIndex`; verify each account is findable by ID, by lowercased name, and by lowercased profile name; verify roles and profiles match
    * Add in `cmd/awsutils_test.go`
    * **Validates: Requirements 3.19, 7.1, 10.11**
  * [x] 8.2 Write property test for cache file path determinism (Property 10)
    * **Property 10: Cache File Path Determinism**
    * Generate random profile names and filter combinations; verify `cacheFilePath()` returns the same path for the same inputs and different paths for different inputs
    * Add in `cmd/awsutils_test.go`
    * **Validates: Requirements 10.2**
  * [x] 8.3 Write property test for cache expiry detection (Property 11)
    * **Property 11: Cache Expiry Detection**
    * Generate random timestamps and positive durations; write a cache file with the generated `cached_at`; verify `readListAWSAccountsCache` returns data when not expired and returns not-found when expired
    * Add in `cmd/awsutils_test.go`
    * **Validates: Requirements 10.4**
  * [x] 8.4 Write property test for cache duration parsing (Property 12)
    * **Property 12: Cache Duration Parsing**
    * Generate random valid Go duration strings with optional `Nd` day tokens; verify `parseCacheDurationFlag` returns a positive duration; verify day tokens are converted to hours; verify empty/zero/negative inputs return errors
    * Add in `cmd/root_test.go`
    * **Validates: Requirements 10.6, 10.8**

* [x] 9. Property-based tests: Config validation and profile naming
  * [x] 9.1 Write property test for managed block marker validation (Property 6)
    * **Property 6: Managed Block Marker Validation**
    * Generate random AWS config file content with well-formed and malformed managed block markers; verify `inspectManagedMarkers` detects mismatched counts, duplicates, overlaps, unmatched ends, and unclosed starts; verify well-formed configs produce empty issues
    * Add in `cmd/configutils_test.go`
    * **Validates: Requirements 8.3, 8.4, 8.5, 8.6, 8.7**
  * [x] 9.2 Write property test for profile name generation with pattern (Property 7)
    * **Property 7: Profile Name Generation with Pattern**
    * Generate random pattern configs (non-empty order, delimiter, prefix, suffix); verify `getProfileName` output is lowercased, contains expected tokens joined by delimiter, and omits empty prefix/suffix
    * Add in `cmd/configutils_test.go`
    * **Validates: Requirements 9.1, 9.2, 9.3, 9.4, 9.5, 9.12**
  * [x] 9.3 Write property test for substring match replacement (Property 8)
    * **Property 8: Substring Match Replacement in Profile Names**
    * Generate random account names and `substr_match_replace` maps where a key is a CI substring of the name; verify `getProfileName` replaces the account token with the replacement value
    * Add in `cmd/configutils_test.go`
    * **Validates: Requirements 9.6, 9.8**
  * [x] 9.4 Write property test for default profile name / toProfileToken idempotence (Property 9)
    * **Property 9: Default Profile Name Generation**
    * Generate random strings; verify `toProfileToken(toProfileToken(x)) == toProfileToken(x)` (idempotence); verify `buildDefaultProfileName` output is lowercased with non-alphanumeric chars replaced by hyphens
    * Add in `cmd/configutils_test.go`
    * **Validates: Requirements 9.10**

* [x] 10. Property-based tests: Console, get, lookup, update
  * [x] 10.1 Write property test for console URL account subdomain stripping (Property 13)
    * **Property 13: Console URL Account Subdomain Stripping**
    * Generate random AWS Console URLs with account subdomains; verify `stripAccountFromURL` removes the account subdomain; verify URLs without account subdomains pass through unchanged
    * Add in `cmd/console_test.go` (new file)
    * **Validates: Requirements 5.9**
  * [x] 10.2 Write property test for account ID validation (Property 14)
    * **Property 14: Account ID Validation**
    * Generate random strings that are not 12 digits; verify `getRoleNamesForAccountID` returns an error; generate valid 12-digit IDs present in the index; verify roles are returned
    * Add in `cmd/get_test.go`
    * **Validates: Requirements 6.3**
  * [x] 10.3 Write property test for lookup account resolution (Property 15)
    * **Property 15: Lookup Account Resolution Correctness**
    * Generate random lookup indexes and identifiers; verify `resolveLookupAccount` returns exactly one account for unique matches, ambiguity error for multiple matches, and not-found error for no matches
    * Add in `cmd/lookup_test.go`
    * **Validates: Requirements 7.4, 7.5**
  * [x] 10.4 Write property test for lookup role substring search (Property 16)
    * **Property 16: Lookup Role Substring Search**
    * Generate random accounts with roles and a non-empty search substring; verify role lookup returns only roles containing the substring (CI), sorted alphabetically (CI), and the result is a subset of the account's roles
    * Add in `cmd/lookup_test.go`
    * **Validates: Requirements 7.6, 7.7**
  * [x] 10.5 Write property test for update managed section generation (Property 17)
    * **Property 17: Update Managed Section Generation**
    * Generate random `listAccounts` with roles and a valid SSO session section; verify `buildUpdatedManagedSections` produces sections where each account-role has a `[profile <name>]` with exactly the keys `sso_session`, `sso_account_id`, `sso_role_name`, `region`, `output`; verify count equals total account-role combinations
    * Add in `cmd/update_test.go`
    * **Validates: Requirements 4.9**

* [x] 11. Checkpoint - Verify property tests
  * Ensure all tests pass, ask the user if questions arise.

* [x] 12. Unit tests for existing functionality
  * [x] 12.1 Write unit tests for console URL stripping in `cmd/console_test.go`
    * Table-driven tests for `stripAccountFromURL`: URLs with account subdomains, without subdomains, non-console URLs, edge cases
    * Table-driven tests for `getStartURL`: valid profile, missing profile
    * Table-driven tests for `minMaxRows`: various slice lengths
    * Table-driven tests for `getRolesForAccount`: matching account, missing account
    * _Requirements: 5.9_
  * [x] 12.2 Write unit tests for command alias registration
    * Verify `authCmd` has alias `login`
    * Verify `listCmd` has alias `ls`
    * Verify `updateCmd` has aliases `upgrade` and `sync`
    * Verify `validateCmd` has aliases `check` and `lint`
    * Add in `cmd/root_test.go`
    * _Requirements: 2.4, 3.4, 4.4, 8.1_
  * [x] 12.3 Write additional unit tests for `parseCacheDurationFlag` edge cases
    * Test empty string, negative duration (`-1h`), multi-day tokens (`2d12h`), case insensitivity of `d` suffix
    * Add in `cmd/root_test.go`
    * _Requirements: 10.6, 10.7, 10.8_
  * [x] 12.4 Write unit tests for `buildListAWSAccountsLookupIndex` edge cases
    * Test empty accounts list, accounts with no roles, duplicate account IDs, accounts with empty names
    * Add in `cmd/awsutils_test.go`
    * _Requirements: 3.19, 10.11_
  * [x] 12.5 Write unit tests for `quoteCSVCell` edge cases
    * Test empty string, string with commas, string with double quotes, string with newlines
    * Add in `cmd/list_test.go`
    * _Requirements: 3.10_

* [x] 13. Final checkpoint - Ensure all tests pass
  * Ensure all tests pass, ask the user if questions arise.

## Notes

* Tasks marked with `*` are optional and can be skipped for faster MVP
* Each task references specific requirements for traceability
* Checkpoints ensure incremental validation
* Property tests validate universal correctness properties from the design document using `pgregory.net/rapid`
* Unit tests validate specific examples and edge cases using table-driven style
* Behavior changes (tasks 2-5) are the only tasks that modify production code; all other tasks add tests only

# Implementation Plan: logout-command

## Overview

Implement the `logout` command in a single new file `cmd/logout.go` with tests in `cmd/logout_test.go`. The command resolves an SSO profile name (positional arg → Viper config → interactive prompt), locates the OIDC token cache file via existing `getSsoSession` and `getCacheFilePath` helpers, and deletes it. Missing files are handled gracefully; other errors are wrapped and returned.

## Tasks

* [x] 1. Create `cmd/logout.go` with command skeleton and test seam
  * [x] 1.1 Create `cmd/logout.go` with Apache 2.0 license header, `removeFile` test seam (`var removeFile = os.Remove`), `logoutCmd` variable with `Use: "logout [sso-profile-name]"`, `Args: cobra.RangeArgs(0, 1)`, and a stub `RunE` that returns `nil`
    * Register `logoutCmd` on `rootCmd` via `init()`
    * _Requirements: 1.1, 1.2, 1.3_

  * [x] 1.2 Implement profile resolution in `RunE`: check `args[0]`, fall back to `asmConfig.GetString("profile-name")`, then fall back to `promptProfileSelect`
    * Follow the same pattern as `authCmd.RunE` in `cmd/auth.go`
    * _Requirements: 2.1, 2.2, 2.3_

  * [x] 1.3 Implement cache file deletion logic in `RunE`: call `getSsoSession` → `getCacheFilePath` → `removeFile`, handle `os.ErrNotExist` (print info, return nil), wrap other errors with `fmt.Errorf` and `%w`, print confirmation on success
    * Use `errors.Is(err, os.ErrNotExist)` for missing file check
    * Print messages must include the profile name
    * _Requirements: 3.1, 3.2, 3.3, 3.4, 4.1, 4.2, 4.3, 4.4_

* [x] 2. Checkpoint
  * Ensure the code compiles, ask the user if questions arise.

* [x] 3. Write unit tests for `cmd/logout.go`
  * [x] 3.1 Write unit tests in `cmd/logout_test.go` for command registration: verify `logoutCmd` is in `rootCmd.Commands()`, has correct `Use`, `Args`, and non-nil `RunE`
    * _Requirements: 1.1, 1.2, 1.3_

  * [x] 3.2 Write unit tests for profile resolution paths: arg provided uses arg, no arg with Viper config uses config value, no arg and no config calls `promptProfileSelect`
    * Save/restore `promptProfileSelect` and `asmConfig` via `t.Cleanup`
    * Silence logger with `logger = slog.New(log.New(io.Discard))`
    * _Requirements: 2.1, 2.2, 2.3_

  * [x] 3.3 Write unit test for permission error wrapping: stub `removeFile` to return a permission error, assert the returned error wraps the original via `errors.Is`
    * _Requirements: 4.4_

* [x] 4. Write property-based tests for correctness properties
  * [x] 4.1 Write property test `TestPropertyLogoutDeletesCacheFile`
    * **Property 1: Cache file deletion**
    * Generate random profile names, create temp AWS config + cache files, run logout, assert file no longer exists
    * Minimum 100 iterations via `rapid.Check`
    * **Validates: Requirements 4.1.**

  * [x] 4.2 Write property test `TestPropertyLogoutMissingFileNoError`
    * **Property 2: Missing cache file is not an error**
    * Generate random profile names, create temp AWS config without cache file, stub `removeFile` to return `os.ErrNotExist`, run logout, assert nil return
    * Minimum 100 iterations via `rapid.Check`
    * **Validates: Requirements 4.3.**

  * [x] 4.3 Write property test `TestPropertyLogoutOutputContainsProfileName`
    * **Property 3: Output always contains the profile name**
    * Generate random profile names, capture stdout, assert output contains the profile name regardless of cache file existence
    * Minimum 100 iterations via `rapid.Check`
    * **Validates: Requirements 4.2, 4.3.**

  * [x] 4.4 Write property test `TestPropertyLogoutInvalidProfileReturnsError`
    * **Property 4: getSsoSession error propagation**
    * Generate random strings not matching any `[sso-session]` in config, run logout, assert error returned and `removeFile` was never called
    * Minimum 100 iterations via `rapid.Check`
    * **Validates: Requirements 3.3.**

* [x] 5. Final checkpoint
  * Ensure all tests pass, ask the user if questions arise.

## Notes

* Tasks marked with `*` are optional and can be skipped for faster MVP
* Each task references specific requirements for traceability
* Property tests use `pgregory.net/rapid` with minimum 100 iterations per property
* All test seams (`removeFile`, `promptProfileSelect`, `awsConfigFilePath`) saved/restored via `t.Cleanup`
* Logger silenced in tests: `logger = slog.New(log.New(io.Discard))`

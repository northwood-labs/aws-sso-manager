# Implementation Plan: SSO Profile Select

## Overview

Replace the free-text `huh.NewInput()` SSO profile prompt in `auth`, `list`, and `update` commands with a `huh.NewSelect()` dropdown populated from `[sso-session ...]` sections in `~/.aws/config`. Extract a shared `promptProfileSelect` helper, refactor `console.go` to use it, and keep `init` on `huh.NewInput()`.

## Tasks

* [x] 1. Create the shared `promptProfileSelect` helper
  * [x] 1.1 Add `promptProfileSelect` function variable to `cmd/` package
    * Create a new file `cmd/prompt.go` with the `promptProfileSelect` package-level function variable
    * The function calls `getAllManagedSections()`, returns a descriptive error if the slice is empty (mentioning `init`), and builds/runs a `huh.NewSelect[string]` with `minMaxRows` height clamping
    * Implement as `var promptProfileSelect = func(target *string) error { ... }` for test seam compatibility
    * _Requirements: 3.1, 3.2, 3.3_

  * [x] 1.2 Write unit test for empty-profile error case
    * Add `TestPromptProfileSelectReturnsErrorWhenNoProfiles` to a new `cmd/prompt_test.go`
    * Set `awsConfigFilePath` to a temp file with no `[sso-session ...]` sections
    * Assert the returned error contains a message mentioning `init`
    * _Requirements: 3.3_

* [x] 2. Switch `auth`, `list`, and `update` commands to use `promptProfileSelect`
  * [x] 2.1 Replace `huh.NewInput()` with `promptProfileSelect()` in `cmd/auth.go`
    * In `authCmd.RunE`, replace the `huh.NewInput()` block with a call to `promptProfileSelect(&profileName)`
    * _Requirements: 1.1_

  * [x] 2.2 Replace `huh.NewInput()` with `promptProfileSelect()` in `cmd/list.go`
    * In `listCmd.RunE`, replace the `huh.NewInput()` block with a call to `promptProfileSelect(&profileName)`
    * _Requirements: 1.2_

  * [x] 2.3 Replace `huh.NewInput()` with `promptProfileSelect()` in `cmd/update.go`
    * In `updateCmd.RunE`, replace the `huh.NewInput()` block with a call to `promptProfileSelect(&profileName)`
    * _Requirements: 1.3_

  * [x] 2.4 Write unit tests verifying `auth`, `list`, and `update` call the select prompt
    * Add `TestAuthCommandUsesSelectPrompt`, `TestListCommandUsesSelectPrompt`, `TestUpdateCommandUsesSelectPrompt` to `cmd/prompt_test.go`
    * Swap the `promptProfileSelect` seam to a spy that records invocation, use save/restore via `t.Cleanup`
    * _Requirements: 1.1, 1.2, 1.3_

* [x] 3. Refactor `console.go` to use the shared helper
  * [x] 3.1 Replace inline `huh.NewSelect` profile prompt in `cmd/console.go` with `promptProfileSelect`
    * In `consoleCmd.RunE`, replace the inline `huh.NewGroup(func() *huh.Select[string] { ... })` block for profile selection with a call to `promptProfileSelect(&profileName)`
    * Adjust the form group construction to skip the profile group when `profileName` is already set
    * _Requirements: 4.1, 4.2_

* [x] 4. Checkpoint - Ensure all tests pass
  * Ensure all tests pass, ask the user if questions arise.

* [x] 5. Verify `init` command is unchanged
  * [x] 5.1 Confirm `cmd/init.go` still uses `huh.NewInput()` for profile name collection
    * No code changes needed; verify the `init` command's `RunE` still calls `huh.NewInput()` and does not reference `promptProfileSelect`
    * _Requirements: 2.1, 2.2_

  * [x] 5.2 Write unit test confirming `init` does not use the select prompt
    * Add `TestInitCommandUsesInputPrompt` to `cmd/prompt_test.go`
    * Swap the `promptProfileSelect` seam to a spy and verify it is NOT called when `init` runs without a profile argument
    * _Requirements: 2.1, 2.2_

* [x] 6. Property-based test for SSO session parsing
  * [x] 6.1 Write property test `TestPropertySSOSessionParsing`
    * **Property 1: SSO session parsing extracts correct sorted profile names**
    * **Validates: Requirements 3.1, 3.2.**
    * Add to `cmd/prompt_test.go` (or `cmd/configutils_test.go` if more appropriate)
    * Generate random AWS config content with 1–5 `[sso-session <name>]` sections using names from `[a-z][a-z0-9]{2,10}`, optionally interleaved with `[profile ...]` sections and `[default]` preamble
    * Write to a temp file, point `awsConfigFilePath` at it, call `getAllManagedSections()`, assert the returned slice matches the input names in sorted order
    * Minimum 100 iterations (rapid default)

* [x] 7. Final checkpoint - Ensure all tests pass
  * Ensure all tests pass, ask the user if questions arise.

## Notes

* Tasks marked with `*` are optional and can be skipped for faster MVP
* Each task references specific requirements for traceability
* The `promptProfileSelect` function variable follows the project's test seam pattern (same as `listAWSAccountsFetcher`, `osExit`)
* `getAllManagedSections()` and `minMaxRows()` are reused as-is from `configutils.go` and `console.go`
* Property tests use `pgregory.net/rapid` per project conventions

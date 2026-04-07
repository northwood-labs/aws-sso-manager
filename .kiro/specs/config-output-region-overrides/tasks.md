# Implementation Plan: Config Output & Region Overrides

## Overview

Surgically modify `buildUpdatedManagedSections` in `cmd/update.go` to read `%.settings.region` and `%.settings.output` from the TOML config via `asmConfig`, falling back to `sso_region` and `"json"` respectively. Update documentation to describe the new keys. All code lives in `cmd/` (flat package). No schema or validation changes needed.

## Tasks

* [x] 1. Modify `buildUpdatedManagedSections` to resolve region and output overrides
  * [x] 1.1 Read overrides and apply fallback logic in `cmd/update.go`
    * Before the profile loop, read `asmConfig.GetString(profileName + ".settings.region")` and `asmConfig.GetString(profileName + ".settings.output")`
    * If `settings.region` is empty, fall back to `ssoSection.String("sso_region")`
    * If `settings.output` is empty, fall back to `"json"`
    * Replace the hardcoded `m["region"]` and `m["output"]` assignments with the resolved values
    * _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 4.1, 4.2_

  * [x] 1.2 Write property test for region override resolution
    * **Property 1: Region resolution respects config override with sso_region fallback**
    * Test function: `TestPropertyRegionOverrideResolution`
    * Save/restore `asmConfig` via `t.Cleanup`; set `<profile>.settings.region` on a fresh Viper instance
    * Generate random profile names, sso_region values, and optional settings.region values (including empty string and absent)
    * Assert every generated profile's `region` field equals `settings.region` when non-empty, else `sso_region`
    * Reuse `genListAccounts(1, 5)` from `cmd/testhelpers_test.go`
    * **Validates: Requirements 1.1, 1.2, 1.3, 4.1**

  * [x] 1.3 Write property test for output override resolution
    * **Property 2: Output resolution respects config override with "json" fallback**
    * Test function: `TestPropertyOutputOverrideResolution`
    * Save/restore `asmConfig` via `t.Cleanup`; set `<profile>.settings.output` on a fresh Viper instance
    * Generate random profile names and optional settings.output values (including empty string and absent)
    * Assert every generated profile's `output` field equals `settings.output` when non-empty, else `"json"`
    * Reuse `genListAccounts(1, 5)` from `cmd/testhelpers_test.go`
    * **Validates: Requirements 2.1, 2.2, 2.3, 4.2**

* [x] 2. Checkpoint — Ensure all tests pass
  * Ensure all tests pass, ask the user if questions arise.

* [x] 3. Add property tests for validation and idempotence
  * [x] 3.1 Write property test for settings key validation
    * **Property 3: Settings key validation accepts region/output and rejects unknown keys**
    * Test function: `TestPropertySettingsKeyValidation`
    * Generate random profile names; assert `validateConfigKey("<profile>.settings.region")` and `validateConfigKey("<profile>.settings.output")` return nil
    * Generate random unknown leaf names (not `"region"` or `"output"`); assert `validateConfigKey("<profile>.settings.<unknown>")` returns non-nil error
    * **Validates: Requirements 3.1, 3.2, 3.3**

  * [x] 3.2 Write property test for update idempotence with overrides
    * **Property 4: Update idempotence with settings overrides**
    * Test function: `TestPropertyUpdateIdempotenceWithOverrides`
    * Save/restore `asmConfig` via `t.Cleanup`; configure random settings.region and settings.output values
    * Call `buildUpdatedManagedSections` twice with identical inputs
    * Assert both calls produce identical output sections and counts
    * Reuse `genListAccounts(1, 5)` from `cmd/testhelpers_test.go`
    * **Validates: Requirements 5.1**

* [x] 4. Checkpoint — Ensure all tests pass
  * Ensure all tests pass, ask the user if questions arise.

* [x] 5. Update documentation in `docs/config_file.md`
  * [x] 5.1 Add settings keys to TOML key tree diagram and documentation sections
    * Add `settings.` subtree with `region (string)` and `output (string)` as siblings of `rename.` under the `%.` node in the tree diagram
    * Add a `### %.settings.region` section describing the key as a per-SSO-profile override for the `region` field, with fallback to `sso_region`
    * Add a `### %.settings.output` section describing the key as a per-SSO-profile override for the `output` field, with fallback to `json`
    * Add a `[%.settings]` table to the sample config demonstrating `region` and `output` values
    * _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5_

* [x] 6. Final checkpoint — Ensure all tests pass
  * Ensure all tests pass, ask the user if questions arise.

## Notes

* Tasks marked with `*` are optional and can be skipped for faster MVP
* The `asmConfig` test seam uses save/restore via `t.Cleanup` per `testing-conventions.md`
* Existing generators (`genListAccounts`, `genAccountID`) are reused; no new generators needed in `testhelpers_test.go`
* Property tests use `pgregory.net/rapid` with default 100+ iterations
* No changes to `cmd/configschema.go`, `cmd/configutils.go`, or `cmd/config.go` — schema and validation already support these keys

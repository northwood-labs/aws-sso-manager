---
inclusion: fileMatch
fileMatchPattern: "cmd/configutils*,cmd/update*,cmd/init*,cmd/validate*"
---

# Managed Block Rules

This document covers the marker-delimited regions of `~/.aws/config` that `aws-sso-manager` owns. Loaded when config-manipulation, update, init, or validate files are in context.

## Marker format

Markers are INI comments (prefixed with `;`), invisible to the AWS CLI and other INI parsers.

```ini
; -------- aws-sso-manager: start <profile> --------
[sso-session <profile>]
sso_start_url = ...
sso_region = ...

[profile <account>-<role>]
sso_session = <profile>
...
; -------- aws-sso-manager: end <profile> --------
```

Constraints:

* Each profile has exactly one start/end marker pair. Duplicates are an error.
* Content between markers is fully owned by this tool and rebuilt from scratch on `update`.
* Content outside markers is never modified.
* The prefix constants `managedStartMarkerPrefix` and `managedEndMarkerPrefix` (in `configutils.go`) define the marker strings used for scanning.

## Function call chain

Operations on managed blocks follow a strict ordering:

1. `inspectManagedMarkers()` — single-pass scanner that detects all structural anomalies (mismatched counts, duplicates, overlaps, orphaned ends, unclosed starts). Returns a `managedMarkerReport` with per-profile issue lists.
2. `validateManagedMarkers()` / `validateMarkers(profileName)` — wraps `inspectManagedMarkers()` and returns a joined error if any issues exist. Must succeed before extraction.
3. `getManagedSection(profileName)` — extracts content between a profile's markers into a temp file. Calls `validateManagedMarkers()` internally as a precondition.
4. `setManagedSection(tmpFile, profileName)` — replaces content between markers with new content from `tmpFile`. Uses an inject-once guard (`injectedInBlock`) to prevent duplication within a single block.

Always call validate before get; always call get before set. Do not bypass this chain.

## Init guard

`init` calls `markersExist(profileName)` before writing. If markers already exist for the profile (even without a matching `[sso-session]` header), `init` refuses to proceed. This prevents orphaned marker accumulation from repeated init runs.

## Update semantics

`update` rebuilds the managed block from scratch on every run:

1. Acquires the advisory lock via `acquireAWSConfigLock`.
2. Extracts current content with `getManagedSection`.
3. Builds a fresh `configFile.Sections` containing only the `[sso-session]` and current account/role profiles (stale profiles are intentionally dropped).
4. Writes the new content to a temp file, then calls `setManagedSection` to splice it back.
5. Atomic rename replaces the config file.

## Validate command

`validate` (aliases: `check`, `lint`) performs a comprehensive check across all profiles:

* Exactly one start and one matching end marker per profile.
* No duplicate blocks for the same profile name.
* Every marker has a corresponding `[sso-session <profile>]` section (detects orphaned markers).
* Every `[sso-session]` section has markers (detects unmanaged sections).
* Exit code 0 when all checks pass, 1 when any problem is found.

## Profile name generation

`getProfileName(profileName, account, role)` reads pattern config from Viper under the `<profileName>.rename` namespace:

| Config key                      | Purpose                                               |
|---------------------------------|-------------------------------------------------------|
| `pattern.order`                 | Token sequence: `account`, `role`, `prefix`, `suffix` |
| `pattern.delimiter`             | Separator between tokens (default: `-`)               |
| `prefix` / `suffix`             | Static strings prepended/appended to the name         |
| `accounts.substr_match_replace` | Map of substring → replacement for account names      |
| `roles.substr_match_replace`    | Map of substring → replacement for role names         |

Fallback: when no pattern is configured or all tokens resolve to empty, `buildDefaultProfileName(account, role)` produces `<account>-<role>` using `toProfileToken` on each part.

### `toProfileToken` contract

* Lowercases input, replaces non-alphanumeric runs with a single `-`, trims leading/trailing dashes.
* Must be idempotent: `toProfileToken(toProfileToken(x)) == toProfileToken(x)`.
* Empty input returns empty string.
* `buildDefaultProfileName` returns the literal `"profile"` only when both account and role tokens are empty.

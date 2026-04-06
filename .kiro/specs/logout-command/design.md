# Design Document: logout-command

## Overview

The `logout` command deletes the on-disk OIDC token cache file for a given SSO profile, effectively ending the session. It mirrors the `auth` command's profile resolution flow (positional arg → Viper config → interactive prompt), then calls `getSsoSession` → `getCacheFilePath` → `os.Remove`. No AWS API calls are needed; the operation is purely local file deletion.

The command is registered on `rootCmd` as a new Cobra subcommand in `cmd/logout.go`, following the same single-file-per-command pattern used by `auth`, `console`, `list`, etc.

## Important notes

* DO NOT try to use Bash. It is broken on this system. Use `zsh` instead.
* DO NOT try to find the `go` binary. It is installed and on the $PATH, so simply call it.

## Architecture

```mermaid
flowchart TD
    A[User runs: logout profile-name] --> B{Profile name provided?}
    B -->|arg[0]| C[Use arg as profileName]
    B -->|no arg| D{Viper config set?}
    D -->|yes| C
    D -->|no| E[promptProfileSelect]
    E --> C
    C --> F[getSsoSession profileName]
    F -->|error| G[Return error]
    F -->|ssoProfile| H[getCacheFilePath &ssoProfile]
    H -->|error| G
    H -->|path| I[os.Remove path]
    I -->|success| J[Print confirmation]
    I -->|os.ErrNotExist| K[Print 'no active session']
    I -->|other error| G
```

The logout command sits at the same level as `auth` in the command tree. It reuses existing infrastructure without introducing new packages, interfaces, or shared state.

## Components and Interfaces

### New file: `cmd/logout.go`

A single file containing:

* `logoutCmd` — package-level `*cobra.Command` variable, registered on `rootCmd` via `init()`.
* `RunE` function — the command body implementing the profile resolution → cache deletion flow.
* `removeFile` — package-level function variable (test seam) defaulting to `os.Remove`. Allows tests to intercept file deletion without touching the real filesystem.

### Reused components (no modifications)

| Component                       | Source            | Purpose                                     |
|---------------------------------|-------------------|---------------------------------------------|
| `getSsoSession(profileName)`    | `cmd/awsutils.go` | Resolves profile name → `ssoProfile` struct |
| `getCacheFilePath(&ssoProfile)` | `cmd/awsutils.go` | Resolves `ssoProfile` → cache file path     |
| `promptProfileSelect(*string)`  | `cmd/prompt.go`   | Interactive TUI profile picker              |
| `asmConfig`                     | `cmd/root.go`     | Viper instance for `profile-name` fallback  |

### Test seam: `removeFile`

```go
var removeFile = os.Remove
```

This follows the project's established pattern (see `listAWSAccountsFetcher`, `osExit`, `runRootCommand`). Tests save/restore via `t.Cleanup`.

## Data Models

No new data models. The command operates on existing types:

* `ssoProfile` — resolved from the AWS config file via `getSsoSession`.
* The cache file path is a plain `string` returned by `getCacheFilePath`.
* The cache file itself is a JSON-encoded `cacheFileData`, but the logout command does not read or parse it — it only deletes the file.

## Correctness Properties

_A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees._

### Property 1: Cache file deletion

_For any_ valid SSO profile with an existing cache file on disk, running the logout command SHALL remove that file so it no longer exists.

**Validates: Requirements 4.1**

### Property 2: Missing cache file is not an error

_For any_ valid SSO profile where the cache file does not exist on disk, running the logout command SHALL return nil (no error).

**Validates: Requirements 4.3**

### Property 3: Output always contains the profile name

_For any_ SSO profile name, the logout command's printed output SHALL contain that profile name, regardless of whether the cache file existed before the command ran.

**Validates: Requirements 4.2, 4.3**

### Property 4: getSsoSession error propagation

_For any_ profile name that does not correspond to an `[sso-session]` section in the AWS config file, the logout command SHALL return an error and SHALL NOT attempt file deletion.

**Validates: Requirements 3.3**

## Error Handling

| Scenario                                                | Behavior                                                                            |
|---------------------------------------------------------|-------------------------------------------------------------------------------------|
| `getSsoSession` returns error (missing/invalid profile) | Return the error immediately; no file operation attempted                           |
| `getCacheFilePath` returns error                        | Return the error immediately; no file operation attempted                           |
| `os.Remove` succeeds                                    | Print confirmation message including profile name                                   |
| `os.Remove` returns `os.ErrNotExist`                    | Print "no active session" message including profile name; return nil                |
| `os.Remove` returns other error (permission, I/O)       | Return a descriptive error wrapping the underlying cause via `fmt.Errorf` with `%w` |

All errors use `fmt.Errorf` with `%w` for wrapping, consistent with the project's error handling conventions. The command uses `RunE` so errors propagate to Cobra's error reporting.

## Testing Strategy

### Property-based tests (`pgregory.net/rapid`)

Each correctness property maps to a property-based test in `cmd/logout_test.go`:

| Property                        | Test function                                  | Strategy                                                                                                |
|---------------------------------|------------------------------------------------|---------------------------------------------------------------------------------------------------------|
| 1: Cache file deletion          | `TestPropertyLogoutDeletesCacheFile`           | Generate random profile names, create temp AWS config + cache files, run logout, assert file removed    |
| 2: Missing file no error        | `TestPropertyLogoutMissingFileNoError`         | Generate random profile names, create temp AWS config without cache file, run logout, assert nil return |
| 3: Output contains profile name | `TestPropertyLogoutOutputContainsProfileName`  | Generate random profile names, capture stdout, assert output contains the profile name                  |
| 4: Error propagation            | `TestPropertyLogoutInvalidProfileReturnsError` | Generate random strings not matching any `[sso-session]` in config, run logout, assert error returned   |

Configuration:
* Minimum 100 iterations per property (rapid's default)
* Each test tagged with: `// Feature: logout-command, Property N: <title>`
* Each test annotated with: `// **Validates: Requirements X.Y**`

### Unit tests (example-based)

| Scenario                  | What it verifies                                                                      |
|---------------------------|---------------------------------------------------------------------------------------|
| Command registration      | `logoutCmd` is registered on `rootCmd` with correct `Use`, `Args`, and non-nil `RunE` |
| Profile from arg          | Passing `args[0]` uses that as profile name                                           |
| Profile from Viper config | No args + Viper `profile-name` set → uses config value                                |
| Profile from prompt       | No args + no config → `promptProfileSelect` called                                    |
| Permission error wrapping | `removeFile` returns permission error → command returns wrapped error                 |

### Test seams used

| Seam                  | Purpose in logout tests                                            |
|-----------------------|--------------------------------------------------------------------|
| `removeFile` (new)    | Intercept `os.Remove` to verify deletion calls and simulate errors |
| `promptProfileSelect` | Stub interactive prompt to return a known profile name             |
| `awsConfigFilePath`   | Point to a temp AWS config file with test `[sso-session]` sections |

### What is NOT property-tested

* Command registration (smoke test — single assertion)
* Viper config fallback (example — specific scenario)
* Prompt fallback (example — specific scenario)
* Permission error wrapping (example — hard to generate arbitrary OS errors)

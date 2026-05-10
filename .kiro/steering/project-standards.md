---
inclusion: always
---

# Project Standards

Go CLI tool (`github.com/northwood-labs/aws-sso-manager`, Go 1.26+) that manages AWS Identity Center (SSO) profiles in `~/.aws/config`.

## Architecture

* Single flat package: all production code lives in `cmd/`. No sub-packages.
* CLI framework: Cobra + Fang for command dispatch, Viper for layered config (flags > env > file > defaults).
* TUI: Charmbracelet stack (huh for forms, lipgloss for styling, log for structured logging, spinner for progress).
* AWS interactions: AWS SDK for Go v2 (`sso`, `ssooidc` services).
* INI manipulation: `northwood-labs/aws-config-parser`.
* Property-based testing: `pgregory.net/rapid`.

## Shared state

Package-level variables are the intentional design for shared state. Do not refactor into dependency injection.

| Variable             | Purpose                                     |
| -------------------- | ------------------------------------------- |
| `asvConfig`          | Viper instance merging TOML, env, and flags |
| `logger`             | Charmbracelet structured logger (stderr)    |
| `awsConfigFilePath`  | Resolved path to `~/.aws/config`            |
| `awsManagerCacheDir` | Cache directory path                        |

## Test seams

Testability is achieved through package-level function variables that can be swapped in tests. Always save the original and restore via `t.Cleanup`.

Key seams: `listAWSAccountsFetcher`, `osExit`, `runRootCommand`, `fangExecute`.

See `testing-conventions.md` for full testing rules (loaded automatically with `*_test.go` files).

## Code style

* Commands must use `RunE` (returns `error`), never `Run`.
* Errors: use `errors.New` or `fmt.Errorf` with `%w` for wrapping. Never `panic` in production code.
* Use `errors.As` / `errors.Is` for error inspection, not type assertions.
* Prefer early returns over deep nesting.

## Naming conventions

| Item                        | Convention                                                                      |
| --------------------------- | ------------------------------------------------------------------------------- |
| Environment variable prefix | `ASM_` (not `ASV_`)                                                             |
| Config file                 | `~/.config/aws-sso-manager/config.toml`                                         |
| Cache directory             | `~/.config/aws-sso-manager/cache/`                                              |
| Lock file                   | `~/.config/.aws-sso-manager/.config.lock`                                       |
| Env key mapping             | Dots and hyphens become underscores (e.g., `profile-name` → `ASM_PROFILE_NAME`) |

## File operations

* All writes to `~/.aws/config` require the advisory lock via `acquireAWSConfigLock`.
* All file writes use atomic write-to-temp-then-rename.
* Cross-platform locking: `lockutils_unix.go` (flock) / `lockutils_windows.go` (LockFileEx). See `locking-rules.md` for details (loaded automatically with `lockutils*` files).
* Never use raw `syscall` — use `golang.org/x/sys/unix` or `golang.org/x/sys/windows`.

## Comments

* Function-level comments explain WHY, not WHAT. Describe rationale and purpose.
* Add inline comments to complex blocks explaining the reasoning behind the approach.

## Other steering documents

* `testing-conventions.md` — test style, PBT rules, test seams (auto-loaded with `*_test.go`).
* `locking-rules.md` — lock architecture and contract (auto-loaded with `lockutils*`).
* `config-schema.md` — TOML config schema reference (manual, use `#config-schema`).

---
inclusion: manual
---

# Architecture Reference

Pull this in with `#architecture-reference` when you need to understand the overall system structure.

## Module map

```text
main.go → cmd.Execute()
cmd/root.go        — root command, global flags, PersistentPreRunE, config init
cmd/init.go        — first-run SSO session setup with URL normalization
cmd/auth.go        — OAuth 2.0 device authorization flow, token caching
cmd/list.go        — account/role listing (TUI, JSON, CSV, Markdown), --no-cache
cmd/update.go      — config sync: rebuild managed block from current accounts/roles
cmd/console.go     — AWS Console deep-link URL generation
cmd/get.go         — line-delimited output for shell piping (accounts, roles)
cmd/lookup.go      — offline account/role resolution with substring fallback
cmd/validate.go    — managed block structural integrity checks
cmd/version.go     — version display
cmd/awsutils.go    — AWS SDK interactions, caching, lookup index, auth cache
cmd/configutils.go — managed block parsing, profile name generation, INI manipulation
cmd/lockutils.go   — cross-platform advisory file locking (shared logic)
cmd/lockutils_unix.go    — Unix flock implementation
cmd/lockutils_windows.go — Windows LockFileEx implementation
```

## Data flow

```text
User runs command
  → PersistentPreRunE (logger, config)
  → Profile resolution (arg → config default → TUI prompt)
  → Auth check (cache hit → skip, miss → device auth flow)
  → Account/role fetch (cache hit → return, miss → AWS API → cache write)
  → Command-specific logic (list/update/console/get/lookup)
  → Output (stdout for data, stderr for spinners/logs/notes)
```

## Key data structures

* `listAccounts` / `listAccount` / `listRole` — account/role hierarchy from AWS API.
* `listAWSAccountsLookupIndex` — O(1) lookup maps by ID, name (CI), profile (CI).
* `cacheFileData` — SSO OIDC auth token cache.
* `listAWSAccountsCacheData` — accounts cache with `cached_at` timestamp.
* `managedMarkerReport` — structural health of managed blocks.
* `ssoProfile` — parsed [sso-session] config.

## Design documents

* #[[file:docs/quickstart.md]] — quick flow summary with design decision rationale.
* #[[file:docs/comprehensive.md]] — deep architecture audit with full design rationale.
* #[[file:docs/config_file.md]] — TOML config schema and profile naming documentation.

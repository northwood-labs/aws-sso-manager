# Quick Flow Summary

## 1. Entry point

* Startup begins in [main.go](../main.go), where `main` calls `Execute` in [cmd/root.go](../cmd/root.go).
* `Execute` delegates CLI dispatch to fang/cobra through `runRootCommand`, using the root command registered in `rootCmd`.
* Before any subcommand runs, `PersistentPreRunE` initializes logging (with verbosity-dependent caller info) and loads config via `initializeConfig`.

## 2. Primary flow

1. Process starts in `main` ([main.go](../main.go)) and enters the root `Execute` function.
2. `runRootCommand` calls `fangExecute` with `rootCmd`, which resolves which subcommand to run.
3. Shared pre-run initialization executes via `PersistentPreRunE`: logger level selection (0=warn, 1=info, 2=debug, 3+=debug with source locations) and config file/env binding via `initializeConfig`.
4. For the highest-impact operational path, the `update` command handler runs.
5. `update` acquires an exclusive file lock via `acquireAWSConfigLock` (platform-specific: `flock` on Unix, `LockFileEx` on Windows), then extracts the managed block from AWS config via `getManagedSection`, which first calls `validateManagedMarkers` to verify structural integrity.
6. After parsing managed sections via `loadAWSConfig`, it retrieves SSO session details with `getSsoSession`, builds an AWS SDK config with `getSDKConfig`, then ensures authentication via `getOrRefreshAuthenticatedCache`.
7. It fetches accounts/roles through `listAWSAccounts` (cache-aside: reads cache first, falls back to AWS SSO API on miss), then rebuilds profile sections from scratch via `buildUpdatedManagedSections` and injects them back via `setManagedSection`.
8. The rewritten config is atomically committed via `os.Rename`, then the temporary files and lock are released. A note is printed to stderr reminding the user that cached data was used.

## 3. Module roles (short bullets)

* [main.go](../main.go): minimal process entry; forwards control to cmd package.
* [cmd/root.go](../cmd/root.go): root command, global flags (`--config`, `--cache-duration`, `--verbose`), pre-run config/bootstrap, CLI execution via `Execute` and `runRootCommand`. Environment variables use the `ASM_` prefix.
* [cmd/lockutils.go](../cmd/lockutils.go): exclusive file lock preventing concurrent writes to the AWS config. Platform-specific implementations in `lockutils_unix.go` (flock) and `lockutils_windows.go` (LockFileEx). Lock file lives at `~/.config/.aws-sso-manager/.config.lock`.
* [cmd/update.go](../cmd/update.go): main sync path that rebuilds account-role profiles into AWS config from scratch, with locking and atomic rename. Prints a cache-usage note to stderr.
* [cmd/awsutils.go](../cmd/awsutils.go): AWS config/session/cache helpers, cache-aside pattern for account data, lookup index construction, and AWS SSO API traversal.
* [cmd/configutils.go](../cmd/configutils.go): whole-file marker parsing, managed-section extraction/injection, and profile naming logic with configurable patterns.
* [cmd/validate.go](../cmd/validate.go): validates marker structural integrity and reports orphaned or unmanaged profiles.
* [cmd/list.go](../cmd/list.go): account/role discovery with multiple output formats (TUI table, JSON, CSV, Markdown). Supports `--no-cache` to fetch fresh data before deleting old cache. Supports `--accounts` and `--roles` substring filters.
* [cmd/get.go](../cmd/get.go): line-delimited output for shell piping. `get accounts` prints account IDs (or names with `--name`). `get roles --for <id>` prints role names.
* [cmd/lookup.go](../cmd/lookup.go): offline account/role resolution from the local lookup index. Supports exact match by ID, name, or profile, with substring fallback for partial matches.
* [cmd/auth.go](../cmd/auth.go): interactive SSO device auth and token cache creation for later commands.
* [cmd/console.go](../cmd/console.go): generates AWS Console deep-link URLs with pre-selected account and role. Strips account subdomains from pasted URLs.
* [cmd/init.go](../cmd/init.go): first-run setup that creates the SSO session section inside managed block markers. Normalizes SSO start URLs from shorthand formats.

## 4. Design decisions — why we built it this way

### Why managed block markers?

The tool needs to own a region of `~/.aws/config` without disturbing hand-edited sections. Comment-based markers (`; -------- aws-sso-manager: start/end <profile> --------`) let the tool identify its sections unambiguously while remaining invisible to the AWS CLI and other tools that read the config.

### Why atomic file writes?

`~/.aws/config` is a shared resource — other tools, editors, and concurrent invocations of this tool may read it at any time. Writing to a temp file and renaming avoids the window where a reader would see a half-written file. This is especially important on systems where the AWS CLI is invoked frequently by automation.

### Why advisory file locking?

Even with atomic renames, two concurrent `update` runs could each read the config, generate their own version, and then race to rename. The advisory lock serializes writers so the second invocation waits (up to 5 seconds) rather than silently overwriting the first. The lock file lives under `~/.config/.aws-sso-manager/` rather than `~/.aws/` to avoid polluting the AWS config directory.

### Why cross-platform locking?

The tool targets macOS, Linux, and Windows. Unix systems use `flock(2)` via `golang.org/x/sys/unix`; Windows uses `LockFileEx` via `golang.org/x/sys/windows`. The platform-specific code is isolated behind build tags so the shared logic remains clean.

### Why cache-aside with SHA-256 hashed filenames?

Repeated `list` and `update` calls shouldn't hit the AWS API every time — it's slow and rate-limited. The cache uses SHA-256 hashes of the profile name and active filters as filenames so that filtered and unfiltered results are cached independently. A filtered cache must never be served for an unfiltered request.

### Why `--no-cache` fetches before deleting?

If the fresh fetch fails (network error, expired token), the user still has their old cached data. Deleting first would leave them with nothing. The flow is: fetch fresh → delete old → write new.

### Why the `ASM_` environment variable prefix?

Environment variables let users override config file values without editing files — useful in CI/CD and containers. The prefix changed from `ASV_` to `ASM_` to match the tool's name (AWS SSO Manager). Hyphens and dots in config keys are mapped to underscores, so `profile-name` becomes `ASM_PROFILE_NAME`.

### Why substring matching in lookup?

Users rarely remember exact account names. Typing `lookup account internal` should find `internal-prod` without requiring the full name. Exact matches are tried first (by ID, profile name, account name) so that a user who types a complete name always gets a single result. Substring matching is the fallback.

## 5. Risks or unknowns

* Command dispatch is runtime-dynamic through cobra/fang in `runRootCommand`, so exact branch depends on argv.
* `update` and `init` depend on marker comment integrity; `validateManagedMarkers` (called inside `getManagedSection`) and the `validate` command guard against malformed markers via `inspectManagedMarkers`.
* Remote AWS pagination and permissions can change execution shape and introduce failures in `listAWSAccounts`.
* Interactive prompts for missing profile inputs introduce user-driven branches in `init`, `auth`, `update`, and `console`.
* The `update` command uses cached account data by default. Users must run `list --no-cache` first if they need fresh data.

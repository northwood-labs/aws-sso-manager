# Quick Flow Summary

## 1. Entry Point

* Startup begins in [main.go](../main.go), where `main` calls `Execute` in [cmd/root.go](../cmd/root.go).
* `Execute` delegates CLI dispatch to fang/cobra through `runRootCommand`, using the root command registered in `rootCmd`.
* Before any subcommand runs, `PersistentPreRunE` initializes logging and loads config via `initializeConfig`.

## 2. Primary Flow (5-10 steps)

1. Process starts in `main` ([main.go](../main.go)) and enters the root `Execute` function.
1. `runRootCommand` calls `fangExecute` with `rootCmd`, which resolves which subcommand to run.
1. Shared pre-run initialization executes via `PersistentPreRunE`: logger level selection and config file/env binding via `initializeConfig`.
1. For the highest-impact operational path, the `update` command handler runs.
1. `update` acquires an exclusive file lock via `acquireAWSConfigLock`, then extracts the managed block from AWS config via `getManagedSection`, which first calls `validateManagedMarkers` to verify structural integrity.
1. After parsing managed sections via `loadAWSConfig`, it retrieves SSO session details with `getSsoSession`, builds an AWS SDK config with `getSDKConfig`, then reads the cached auth token via `getCacheFilePath` and `cacheData.read`.
1. It fetches accounts/roles from AWS SSO API through `listAWSAccounts`, then rewrites profile sections and injects them back into AWS config via `setManagedSection` (an inject-once guard ensures no duplication).
1. The rewritten config is atomically committed via `os.Rename`, then the temporary files and lock are released via `Release`.

## 3. Module Roles (short bullets)

* [main.go](../main.go): minimal process entry; forwards control to cmd package.
* [cmd/root.go](../cmd/root.go): root command, global flags, pre-run config/bootstrap, CLI execution via `Execute` and `runRootCommand`.
* [cmd/lockutils.go](../cmd/lockutils.go): exclusive file lock preventing concurrent writes to the AWS config, via `acquireAWSConfigLock` and `Release`.
* [cmd/update.go](../cmd/update.go): main sync path that materializes account-role profiles into AWS config, with locking and atomic rename.
* [cmd/awsutils.go](../cmd/awsutils.go): AWS config/session/cache helpers and AWS SSO API traversal.
* [cmd/configutils.go](../cmd/configutils.go): whole-file marker parsing, managed-section extraction/injection, and profile naming logic.
* [cmd/validate.go](../cmd/validate.go): validates marker structural integrity and reports orphaned or unmanaged profiles.
* [cmd/list.go](../cmd/list.go): read-only account/role discovery path using the same AWS traversal core.
* [cmd/auth.go](../cmd/auth.go): interactive SSO device auth and token cache creation for later commands.

## 4. Risks or Unknowns

* Command dispatch is runtime-dynamic through cobra/fang in `runRootCommand`, so exact branch depends on argv.
* `update` and `init` depend on marker comment integrity; `validateManagedMarkers` (called inside `getManagedSection`) and the `validate` command in [cmd/validate.go](../cmd/validate.go) guard against malformed markers via `inspectManagedMarkers`.
* Remote AWS pagination and permissions can change execution shape and introduce failures in `listAWSAccounts`.
* Interactive prompts for missing profile inputs introduce user-driven branches in `init`, `auth`, and `update`.

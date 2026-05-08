---
inclusion: fileMatch
fileMatchPattern: "**/*.go"
---

# Zero Diagnostics Policy

All Go source files must be free of diagnostic errors and warnings. Code is not considered complete until `getDiagnostics` reports zero issues for every file touched during a change.

## Verification workflow

1. After editing any `.go` file, run `getDiagnostics` on that file.
2. If diagnostics are reported, fix every one before moving on.
3. After all fixes, run `golangci-lint run --fix <path>` on each changed file to auto-fix what the linter can and surface anything remaining.
4. Run `go vet ./...` to confirm the build is clean.
5. Run `go test` for the affected package to confirm no regressions.

Do not present work as finished while diagnostics remain.

## Linter categories and fix patterns

This project enables ~60 linters via `.golangci.yml` (v2 format, `default: none`). The most common diagnostic categories and their fixes are listed below.

### Declaration order (decorder)

The enforced order within a file is: `const`, `var`, `type`, `func`. Moving a declaration block to the wrong position triggers a warning.

### Structured logging (sloglint)

* **context**: all slog calls must use `*Context` variants (`InfoContext`, `DebugContext`, `WarnContext`, `ErrorContext`). Pass the `context.Context` from the function signature.
* **no-raw-keys**: define string constants for log keys and reference them instead of inline strings.
* **forbidden-keys**: the keys `time`, `level`, `msg`, `source`, and `foo` must not be used.
* **key-naming-case**: all log keys must be `snake_case`.
* **kv-only**: use key-value pairs, not `slog.Attr` typed attributes.
* **msg-style**: log messages must be capitalized.

### Error handling (err113, wrapcheck, errorlint)

* **err113**: do not create dynamic errors with `fmt.Errorf("...")` alone. Define package-level sentinel errors with `errors.New` and wrap them: `fmt.Errorf("%w: %s", ErrSentinel, detail)`.
* **wrapcheck**: errors returned from external packages (including other internal packages) must be wrapped with `%w`.
* **errorlint**: use `errors.Is` / `errors.As` instead of `==` or type assertions on errors.

### Context propagation (contextcheck)

Functions that have access to a `context.Context` must pass it to callees that accept one. If a helper is called from a context-bearing function, add `ctx context.Context` to the helper's signature.

### Parameter size (gocritic hugeParam)

Structs larger than 64 bytes should be passed by pointer. Common in this codebase: `config.FileRule` (88 bytes). Use `*config.FileRule` in function signatures and dereference at call sites where the original API expects a value.

### Cognitive complexity (gocognit)

Functions with complexity > 20 trigger a warning. Reduce complexity by extracting helper functions for distinct logical branches (e.g., "handle existing destination", "initialize from stub"). Each helper should have a single responsibility and a clear doc comment.

### Security (gosec)

* **G304** (file inclusion via variable): add `// #nosec G304` with a justification comment when the path is derived from a validated config manifest, not user input.
* **G204** (command injection): already suppressed in `hook.RunHook` — do not add new instances without review.
* **G301** (directory permissions): use `0o750` or stricter for `os.MkdirAll`. In test code, `0o755` may be acceptable with a `#nosec` annotation.

### Commented-out code (gocritic commentedOutCode)

Remove commented-out code blocks. If the code is needed for reference, move it behind a build tag or delete it and rely on version control.

### Nil safety

`SyncOptions.Logger` can be nil. Any function that uses the logger outside of `RunSync` (which sets the default) must guard against nil — use a `safeLogger` helper or check before calling.

## Suppression

When a diagnostic cannot be resolved cleanly, use the project's lint suppression comments documented in `docs/agents/style.md`. Always include a justification. Suppression is a last resort — prefer fixing the root cause.

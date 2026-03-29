---
inclusion: fileMatch
fileMatchPattern: "cmd/*_test.go"
---

# Testing Conventions

Loaded because a test file is in context. Follow these conventions for all test code in `cmd/`.

## Test Organization

All tests live in `package cmd` alongside production code. There are no test sub-packages.

## Unit Test Style

* Use table-driven tests with a `tests` slice of structs containing a descriptive `name` field.
* Iterate with `for _, tc := range tests` and call `t.Run(tc.name, ...)`.
* Use `t.Fatalf` for fatal assertions that should stop the subtest.
* Use `t.Errorf` for non-fatal assertions where subsequent checks are still meaningful.
* Use `t.Helper()` in any shared assertion or setup helper function.
* Use `t.TempDir()` for temporary directories (auto-cleaned).
* Use `t.Setenv()` for environment variable overrides (auto-restored).

## Test Seam Pattern

Package-level function variables serve as test seams. The save/restore pattern is:

```go
oldFetcher := listAWSAccountsFetcher
t.Cleanup(func() { listAWSAccountsFetcher = oldFetcher })

listAWSAccountsFetcher = func(input listAWSAccountsInput) (listAccounts, error) {
    return mockData, nil
}
```

Available seams and their purposes:

| Seam                     | Purpose                                         |
|--------------------------|-------------------------------------------------|
| `listAWSAccountsFetcher` | Mock AWS SSO API calls                          |
| `osExit`                 | Capture exit codes without killing test process |
| `runRootCommand`         | Test CLI dispatch without real execution        |
| `fangExecute`            | Test Fang integration without real execution    |

The same save/restore pattern applies to package-level state variables (`asvConfig`, `logger`, `awsConfigFilePath`, `awsManagerCacheDir`, `userHomeDir`, `cacheDuration`). Always restore via `t.Cleanup` or `defer`.

## Property-Based Testing (PBT)

Use `pgregory.net/rapid`. Never use `testing/quick`.

### Comment Tagging

Every PBT test function gets a comment immediately above it:

```go
// Feature: aws-sso-manager, Property N: <title>
```

Inside the function, link to requirements with:

```go
// **Validates: Requirements X.Y, X.Z**
```

The `// **Validates:**` comment goes either directly inside the test function or inside a specific subtest when the function has multiple `t.Run` blocks that validate different requirements.

### Test Function Naming

PBT test functions use the prefix `TestProperty`, e.g., `TestPropertyAccountAndRoleSorting`.

### Subtests in PBT

PBT tests may use `t.Run` to separate sub-properties. Each subtest calls `rapid.Check` independently:

```go
func TestPropertySSOStartURLNormalization(t *testing.T) {
    t.Run("bare_subdomain", func(t *testing.T) {
        rapid.Check(t, func(t *rapid.T) { ... })
    })

t.Run("full_url", func(t *testing.T) {
        rapid.Check(t, func(t *rapid.T) { ... })
    })
}
```

### Iteration Count

Each property must run at least 100 iterations (rapid's default). Do not reduce this.

### Shared Generators

Shared generators live in `cmd/testhelpers_test.go`. Reuse them instead of creating ad-hoc generators:

| Generator                         | Produces                                                        |
|-----------------------------------|-----------------------------------------------------------------|
| `genAccountID()`                  | Valid 12-digit numeric account ID string                        |
| `genListAccount()`                | Random account with valid ID, 1–5 roles, realistic fields       |
| `genListAccounts(min, max)`       | Pre-sorted `listAccounts` matching production sort order        |
| `genLookupIndex()`                | Lookup index built via real `buildListAWSAccountsLookupIndex`   |
| `genManagedBlockConfig(profiles)` | Well-formed AWS config content with managed block markers       |
| `genProfilePatternConfig()`       | Random profile naming config (order, delimiter, prefix, suffix) |

When a test needs data that an existing generator covers, use the generator. Only create new generators for genuinely new data shapes, and add them to `testhelpers_test.go`.

### Generator Design Principles

* Generators should produce realistic data that exercises production code paths.
* `genLookupIndex()` calls the real `buildListAWSAccountsLookupIndex` rather than hand-crafting mock indexes, so tests exercise actual index-building logic.
* `genListAccounts` pre-sorts to match production sort order so property tests can assert on sorted output directly.

## Mutation Testing

Mutation tests live in `cmd/mutation_test.go` behind the `//go:build mutation` build tag. They use `github.com/gtramontina/ooze` with a 0.75 minimum threshold. Do not modify the mutation test configuration without explicit instruction.

## Prohibited Patterns

* `os.Setenv` / `os.Unsetenv` — use `t.Setenv` instead.
* `os.Exit` / `log.Fatal` — never call these in test code.
* `syscall` — use `golang.org/x/sys/unix` or `golang.org/x/sys/windows`.
* Wildcard `*` in environment variable names — use `_` (e.g., `ASM_PROFILE_NAME`).
* `testing/quick` — use `pgregory.net/rapid` for all property-based tests.

## Logger in Tests

Silence the logger in tests that touch code using the package-level `logger`:

```go
logger = log.New(io.Discard)
```

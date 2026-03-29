---
inclusion: fileMatch
fileMatchPattern: "cmd/awsutils*,cmd/list*"
---

# Caching Rules

Loaded because an awsutils or list file is in context.

## Cache Layout

* Accounts cache: `~/.config/aws-sso-manager/cache/accounts-<sha256>.json`
* Lookup index: same path with `-lookup.json` suffix.
* SHA-256 key: `"listAWSAccounts.v1\x00<profile>\x00<accountFilter>\x00<roleFilter>"`.
* Filtered and unfiltered results are cached independently — never serve a filtered cache for an unfiltered request.

## Cache-Aside Pattern

`listAWSAccounts()` implements: check cache → return on hit → fetch from AWS API on miss → write to cache → return.

## Expiry

* Default TTL: 24 hours. Configurable via `--cache-duration` (supports Go durations + `Nd` day suffix).
* Expiry is checked on read via `cached_at + cacheDuration`. Expired files are deleted proactively.
* Zero and negative durations are rejected by `parseCacheDurationFlag`.

## --no-cache Ordering

When `--no-cache` is set on the `list` command, the flow is:

1. Fetch fresh data (bypassing cache read, calling `listAWSAccountsFetcher` directly).
2. Delete old cache files.
3. Write fresh data to cache.

This ordering ensures old data survives a failed fresh fetch.

## Lookup Index

* Only written when no account/role filters are active (`shouldWriteLookupCache`).
* Built by `buildListAWSAccountsLookupIndex` — three maps: by ID, by name (CI), by profile (CI).
* Consumed by `get` and `lookup` commands for offline resolution.

## Atomic Writes

All cache writes use write-to-`.tmp`-then-rename. Cache file permissions: 0600.

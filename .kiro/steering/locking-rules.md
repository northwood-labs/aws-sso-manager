---
inclusion: fileMatch
fileMatchPattern: "cmd/lockutils*"
---

# File Locking Rules

Loaded because a lockutils file is in context.

## Architecture

* `cmd/lockutils.go` — shared types, constants, and platform-agnostic acquire/release logic.
* `cmd/lockutils_unix.go` (`//go:build !windows`) — `lockFileNB`, `unlockFile`, `isLockBusy` via `golang.org/x/sys/unix.Flock`.
* `cmd/lockutils_windows.go` (`//go:build windows`) — same three functions via `golang.org/x/sys/windows.LockFileEx`/`UnlockFileEx`.

## Contract

* `lockFileNB(fd uintptr) error` — non-blocking exclusive lock attempt.
* `unlockFile(fd uintptr) error` — release the lock.
* `isLockBusy(err error) bool` — true if the error means "lock held by another process, retry".

## Rules

* Never use raw `syscall.Flock` — always go through `golang.org/x/sys`.
* The `//go:build` tag must be the very first line of platform-specific files.
* Lock file path: `~/.config/.aws-sso-manager/.config.lock` (not `~/.aws/`).
* Lock directory permissions: 0755. Lock file permissions: 0600.
* Timeout: 5 seconds. Retry interval: 100ms.
* PID is written to the lock file after acquisition for debugging stale locks.
* `Release()` must be safe to call on nil receiver (for unconditional defer).

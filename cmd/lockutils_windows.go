//go:build windows

// Copyright 2025-2026, Northwood Labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"errors"

	"golang.org/x/sys/windows"
)

const (
	// lockfileExclusiveLock requests an exclusive (write) lock, matching the
	// semantics of LOCK_EX on Unix.
	lockfileExclusiveLock = 0x02

	// lockfileFailImmediately makes the call non-blocking so we can implement
	// our own retry loop with timeout, matching LOCK_NB on Unix.
	lockfileFailImmediately = 0x01
)

// lockFileNB attempts a non-blocking exclusive lock on the file descriptor.
// On Windows, LockFileEx with LOCKFILE_FAIL_IMMEDIATELY is the equivalent of
// flock(LOCK_EX|LOCK_NB). We lock just 1 byte because we only need mutual
// exclusion, not range locking.
func lockFileNB(fd uintptr) error {
	ol := new(windows.Overlapped)
	return windows.LockFileEx(
		windows.Handle(fd),
		lockfileExclusiveLock|lockfileFailImmediately,
		0, // reserved
		1, // lock 1 byte
		0, // high-order bytes
		ol,
	)
}

// unlockFile releases the lock on the file descriptor.
func unlockFile(fd uintptr) error {
	ol := new(windows.Overlapped)
	return windows.UnlockFileEx(
		windows.Handle(fd),
		0, // reserved
		1, // unlock 1 byte
		0, // high-order bytes
		ol,
	)
}

// isLockBusy reports whether err indicates the lock is held by another process.
// On Windows, a non-blocking lock attempt returns ERROR_LOCK_VIOLATION.
func isLockBusy(err error) bool {
	return errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}

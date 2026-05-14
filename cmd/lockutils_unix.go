//go:build !windows

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
	"fmt"

	"golang.org/x/sys/unix"
)

// lockFileNB attempts a non-blocking exclusive lock on the file descriptor.
// Non-blocking so we can implement our own retry loop with timeout and context
// cancellation, rather than blocking the goroutine inside the kernel.
func lockFileNB(fd uintptr) error {
	if err := unix.Flock(int(fd), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fmt.Errorf("could not acquire file lock: %w", err)
	}

	return nil
}

// unlockFile releases the lock on the file descriptor.
func unlockFile(fd uintptr) error {
	if err := unix.Flock(int(fd), unix.LOCK_UN); err != nil {
		return fmt.Errorf("could not release file lock: %w", err)
	}

	return nil
}

// isLockBusy reports whether err indicates the lock is held by another process.
// On Unix, EWOULDBLOCK and EAGAIN both mean "lock is busy" (they're often the
// same errno). EINTR means a signal interrupted the call and it's safe to retry.
func isLockBusy(err error) bool {
	return errors.Is(err, unix.EWOULDBLOCK) ||
		errors.Is(err, unix.EAGAIN) ||
		errors.Is(err, unix.EINTR)
}

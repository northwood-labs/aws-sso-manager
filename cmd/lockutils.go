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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// awsConfigLockRetryInterval controls how often we re-attempt the lock.
	// 100ms is short enough to feel responsive but avoids busy-spinning the CPU.
	awsConfigLockRetryInterval = 100 * time.Millisecond

	// awsConfigLockTimeout caps the total wait so a stuck lock holder (e.g., a
	// crashed process that didn't release) doesn't block the user indefinitely.
	awsConfigLockTimeout = 5 * time.Second
)

// awsConfigLock wraps an open lock file so that Release can be deferred by the
// caller. Keeping the file handle here ensures the advisory lock stays held
// until explicitly released — closing the fd is what actually drops the lock.
type awsConfigLock struct {
	file *os.File
}

// acquireAWSConfigLock obtains an exclusive advisory lock before any write to
// ~/.aws/config. This prevents concurrent tool invocations (e.g., two terminal
// tabs running "update" simultaneously) from interleaving writes and corrupting
// the config file. The lock file lives under ~/.config/.aws-sso-manager/ rather
// than ~/.aws/ to avoid polluting the AWS config directory.
func acquireAWSConfigLock(ctx context.Context) (*awsConfigLock, error) {
	lockCtx, cancel := context.WithTimeout(ctx, awsConfigLockTimeout)
	defer cancel()

	lockDir := filepath.Join(userHomeDir, ".config", ".aws-sso-manager")
	if err := os.MkdirAll(lockDir, 0o0755); err != nil { // lint:allow_raw_number
		return nil, fmt.Errorf("create AWS config directory %q for locking: %w", lockDir, err)
	}

	lockPath := filepath.Join(lockDir, ".config.lock")

	lockFile, err := os.OpenFile( // lint:allow_dynamic_filename
		lockPath,
		os.O_CREATE|os.O_RDWR,
		0o0600, // lint:allow_raw_number
	)
	if err != nil {
		return nil, fmt.Errorf("open AWS config lock file %q: %w", lockPath, err)
	}

	for {
		err = lockFileNB(lockFile.Fd())
		if err == nil {
			// We hold the lock. Write our PID so that a human investigating a
			// stale lock file can identify which process held it. Truncate first
			// to clear any leftover PID from a previous holder.
			if truncateErr := lockFile.Truncate(0); truncateErr != nil {
				_ = lockFile.Close() // lint:allow_unhandled
				return nil, fmt.Errorf("truncate AWS config lock file %q: %w", lockPath, truncateErr)
			}

			if _, seekErr := lockFile.Seek(0, 0); seekErr != nil {
				_ = lockFile.Close() // lint:allow_unhandled
				return nil, fmt.Errorf("seek AWS config lock file %q: %w", lockPath, seekErr)
			}

			if _, writeErr := fmt.Fprintf(lockFile, "%d\n", os.Getpid()); writeErr != nil {
				_ = lockFile.Close() // lint:allow_unhandled
				return nil, fmt.Errorf("write AWS config lock file %q: %w", lockPath, writeErr)
			}

			return &awsConfigLock{file: lockFile}, nil
		}

		// The lock is held by another process. Only retry on transient "busy"
		// errors; anything else (e.g., permission denied) is a hard failure.
		if !isLockBusy(err) {
			_ = lockFile.Close() // lint:allow_unhandled
			return nil, fmt.Errorf("acquire AWS config lock %q: %w", lockPath, err)
		}

		select {
		case <-lockCtx.Done():
			_ = lockFile.Close() // lint:allow_unhandled
			return nil, fmt.Errorf("timed out waiting for AWS config lock %q: %w", lockPath, lockCtx.Err())
		case <-time.After(awsConfigLockRetryInterval):
		}
	}
}

// Release drops the advisory lock and closes the file. It is safe to call on a
// nil receiver so callers can unconditionally defer it. We unlock before closing
// because closing the fd implicitly drops the lock on some platforms, and we
// want the unlock error (if any) to be reported separately.
func (l *awsConfigLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}

	unlockErr := unlockFile(l.file.Fd())
	closeErr := l.file.Close()

	l.file = nil

	if unlockErr != nil {
		return fmt.Errorf("unlock AWS config lock: %w", unlockErr)
	}

	if closeErr != nil {
		return fmt.Errorf("close AWS config lock file: %w", closeErr)
	}

	return nil
}

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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	awsConfigLockRetryInterval = 100 * time.Millisecond
	awsConfigLockTimeout       = 5 * time.Second
)

type awsConfigLock struct {
	file *os.File
}

func acquireAWSConfigLock(ctx context.Context) (*awsConfigLock, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	lockCtx, cancel := context.WithTimeout(ctx, awsConfigLockTimeout)
	defer cancel()

	lockDir := filepath.Dir(awsConfigFilePath)
	if err := os.MkdirAll(lockDir, 0o0755); err != nil {
		return nil, fmt.Errorf("create AWS config directory %q for locking: %w", lockDir, err)
	}

	lockPath := filepath.Join(lockDir, ".aws-sso-manager.config.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o0600)
	if err != nil {
		return nil, fmt.Errorf("open AWS config lock file %q: %w", lockPath, err)
	}

	for {
		err = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			if truncateErr := lockFile.Truncate(0); truncateErr != nil {
				_ = lockFile.Close()
				return nil, fmt.Errorf("truncate AWS config lock file %q: %w", lockPath, truncateErr)
			}
			if _, seekErr := lockFile.Seek(0, 0); seekErr != nil {
				_ = lockFile.Close()
				return nil, fmt.Errorf("seek AWS config lock file %q: %w", lockPath, seekErr)
			}
			if _, writeErr := fmt.Fprintf(lockFile, "%d\n", os.Getpid()); writeErr != nil {
				_ = lockFile.Close()
				return nil, fmt.Errorf("write AWS config lock file %q: %w", lockPath, writeErr)
			}

			return &awsConfigLock{file: lockFile}, nil
		}

		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EINTR) {
			_ = lockFile.Close()
			return nil, fmt.Errorf("acquire AWS config lock %q: %w", lockPath, err)
		}

		select {
		case <-lockCtx.Done():
			_ = lockFile.Close()
			return nil, fmt.Errorf("timed out waiting for AWS config lock %q: %w", lockPath, lockCtx.Err())
		case <-time.After(awsConfigLockRetryInterval):
		}
	}
}

func (l *awsConfigLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}

	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
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

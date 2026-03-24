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
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

func TestParseCacheDurationFlag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{name: "go duration", input: "24h", expected: 24 * time.Hour},
		{name: "days only", input: "1d", expected: 24 * time.Hour},
		{name: "days and hours and minutes", input: "1d6h30m", expected: 30*time.Hour + 30*time.Minute},
		{name: "invalid", input: "abc", wantErr: true},
		{name: "zero", input: "0h", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCacheDurationFlag(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.input)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseCacheDurationFlag(%q): %v", tc.input, err)
			}

			if got != tc.expected {
				t.Fatalf("expected %s for %q, got %s", tc.expected, tc.input, got)
			}
		})
	}
}

func TestExecutePassesFangNotifyOption(t *testing.T) {
	oldRunRootCommand := runRootCommand
	oldOSExit := osExit
	t.Cleanup(func() {
		runRootCommand = oldRunRootCommand
		osExit = oldOSExit
	})

	called := false
	runRootCommand = func(ctx context.Context, cmd *cobra.Command, signals ...os.Signal) error {
		called = true

		if ctx == nil {
			t.Fatal("expected non-nil context passed to runRootCommand")
		}
		if cmd != rootCmd {
			t.Fatal("expected rootCmd to be passed to runRootCommand")
		}
		if len(signals) != 2 {
			t.Fatalf("expected exactly 2 notify signals, got %d", len(signals))
		}
		if signals[0] != syscall.SIGINT || signals[1] != syscall.SIGTERM {
			t.Fatalf("expected notify signals [SIGINT SIGTERM], got %v", signals)
		}

		return nil
	}

	exitCalled := false
	osExit = func(code int) {
		exitCalled = true
		t.Fatalf("did not expect osExit to be called, got code %d", code)
	}

	Execute()

	if !called {
		t.Fatal("expected runRootCommand to be called")
	}
	if exitCalled {
		t.Fatal("did not expect osExit to be called")
	}
}

func TestExecuteExitsOnFangError(t *testing.T) {
	oldRunRootCommand := runRootCommand
	oldOSExit := osExit
	t.Cleanup(func() {
		runRootCommand = oldRunRootCommand
		osExit = oldOSExit
	})

	runRootCommand = func(context.Context, *cobra.Command, ...os.Signal) error {
		return errors.New("boom")
	}

	exitCode := -1
	osExit = func(code int) {
		exitCode = code
	}

	Execute()

	if exitCode != 1 {
		t.Fatalf("expected osExit to be called with code 1, got %d", exitCode)
	}
}

func TestRunRootCommandUsesFangExecute(t *testing.T) {
	oldFangExecute := fangExecute
	t.Cleanup(func() {
		fangExecute = oldFangExecute
	})

	called := false
	fangExecute = func(ctx context.Context, cmd *cobra.Command, options ...fang.Option) error {
		called = true

		if ctx == nil {
			t.Fatal("expected non-nil context passed to fangExecute")
		}
		if cmd != rootCmd {
			t.Fatal("expected rootCmd to be passed to fangExecute")
		}
		if len(options) != 1 {
			t.Fatalf("expected exactly one Fang option, got %d", len(options))
		}

		return nil
	}

	if err := runRootCommand(context.Background(), rootCmd, syscall.SIGINT, syscall.SIGTERM); err != nil {
		t.Fatalf("runRootCommand returned error: %v", err)
	}

	if !called {
		t.Fatal("expected fangExecute to be called")
	}
}

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

// Log key constants for structured logging. The sloglint no-raw-keys rule
// requires all slog key arguments to be string constants rather than inline
// literals.
const (
	logKeyAccountID   = "account_id"
	logKeyBackupFile  = "backup_file"
	logKeyConfig      = "config"
	logKeyConsoleURL  = "console_url"
	logKeyCount       = "count"
	logKeyErr         = "err"
	logKeyFile        = "file"
	logKeyFrom        = "from"
	logKeyHome        = "home"
	logKeyProfile     = "profile"
	logKeyProfileName = "profile_name"
	logKeyRegion      = "region"
	logKeyRoleName    = "role_name"
	logKeySection     = "section"
	logKeyStartHost   = "start_host"
	logKeyTempFile    = "temp_file"
	logKeyTo          = "to"
	logKeyURL         = "url"
	logKeyUser        = "user"
)

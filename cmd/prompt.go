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
	"fmt"

	"charm.land/huh/v2"
)

// promptProfileSelect is a package-level function variable so tests can
// replace the real TUI prompt with a spy without needing an interface.
var promptProfileSelect = func(target *string) error {
	sections, err := getAllManagedSections()
	if err != nil {
		return fmt.Errorf("could not get managed sections: %w", err)
	}

	if len(sections) == 0 {
		return ErrNoSSOProfiles
	}

	return huh.NewSelect[string]().
		Title("Select an SSO profile...").
		Value(target).
		Height(minMaxRows(sections) + 1).
		Options(huh.NewOptions(sections...)...).
		Run()
}

// Copyright 2025-2026, Northwood Labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

var (
	styleSuccess = lipgloss.NewStyle().
			Bold(true).
			Foreground(clihelpers.ClrWhite).
			Background(clihelpers.ClrSuccess).
			Padding(0, 1)

	styleFailure = lipgloss.NewStyle().
			Bold(true).
			Foreground(clihelpers.ClrWhite).
			Background(clihelpers.ClrFailure).
			Padding(0, 1)

	validateCmd = &cobra.Command{
		Use:   "validate",
		Short: "Validates the managed block markers in the AWS config file.",
		Long: clihelpers.LongHelpText(`
		Validates the managed block markers in the AWS config file.

		aws-sso-manager relies on comment markers in your AWS config file to track
		which sections it manages:

			; -------- aws-sso-manager: start <profile> --------
			...
			; -------- aws-sso-manager: end <profile> --------

		This command checks that every managed profile has:

		* Exactly one start marker and one matching end marker (no mismatches).
		* No duplicate blocks for the same profile name.
		* A corresponding [sso-session <profile>] section in the config file.

		It also reports any markers that exist without a corresponding [sso-session]
		section (orphaned markers left behind by manual edits).

		Exit code is 0 when all checks pass, 1 when any problem is found.
		`),
		Args:    cobra.NoArgs,
		Aliases: []string{"check", "lint"},
		Example: strings.TrimSpace(dedent.Dedent(`
		# Validate that the ~/.aws/config file is valid.
		aws-sso-manager validate
		`)),
		RunE: func(_ *cobra.Command, _ []string) error {
			markerReport, err := inspectManagedMarkers()
			if err != nil {
				return fmt.Errorf("reading AWS config markers: %w", err)
			}

			// Collect all profile names from both sources so that we report on the
			// full universe: known sso-session sections and any marker-only
			// profiles.
			ssoSections, err := getAllManagedSections()
			if err != nil {
				return fmt.Errorf("reading AWS config: %w", err)
			}

			markedProfiles := markerReport.profiles

			// Union of both sets.
			union := map[string]struct{}{}
			for _, p := range ssoSections {
				union[p] = struct{}{}
			}

			for _, p := range markedProfiles {
				union[p] = struct{}{}
			}

			if len(union) == 0 {
				fmt.Printf("No managed profiles found in %s.\n", awsConfigFilePath)
				fmt.Println("Run `aws-sso-manager init` to create your first profile.")

				return nil
			}

			// Build lookup sets for fast membership tests.
			hasSSOSection := map[string]bool{}
			for _, p := range ssoSections {
				hasSSOSection[p] = true
			}

			hasMarker := map[string]bool{}
			for _, p := range markedProfiles {
				hasMarker[p] = true
			}

			type result struct {
				profile string
				errs    []string
			}

			results := []result{}

			foundProblems := false

			// Evaluate in a deterministic order.
			allProfiles := make([]string, 0, len(union))
			for p := range union {
				allProfiles = append(allProfiles, p)
			}

			// Sort for deterministic output.
			for i := range len(allProfiles) - 1 {
				for j := i + 1; j < len(allProfiles); j++ {
					if allProfiles[i] > allProfiles[j] {
						allProfiles[i], allProfiles[j] = allProfiles[j], allProfiles[i]
					}
				}
			}

			for _, profile := range allProfiles {
				errs := []string{}

				// Check structural integrity of markers.
				if hasMarker[profile] {
					errs = append(errs, markerReport.issues[profile]...)
				}

				// Marker exists but [sso-session] section is absent (orphaned marker).
				if hasMarker[profile] && !hasSSOSection[profile] {
					errs = append(errs, fmt.Sprintf(
						"managed block markers exist for %q but no [sso-session %s] section was found; "+
							"remove the markers or re-run `init`",
						profile, profile,
					))
				}

				// [sso-session] section exists but no markers (unmanaged section).
				if hasSSOSection[profile] && !hasMarker[profile] {
					errs = append(errs, fmt.Sprintf(
						"[sso-session %s] section exists but has no managed block markers; "+
							"this section will not be updated by `update`",
						profile,
					))
				}

				results = append(results, result{profile: profile, errs: errs})

				if len(errs) > 0 {
					foundProblems = true
				}
			}

			// Print report.
			for _, r := range results {
				if len(r.errs) == 0 {
					_, _ = lipgloss.Println("  " + styleSuccess.Render("OK") + " " + r.profile) // lint:allow_unhandled
				} else {
					_, _ = lipgloss.Println(styleFailure.Render("FAIL") + " " + r.profile) // lint:allow_unhandled

					for _, e := range r.errs {
						fmt.Printf("       → %s\n", e)
					}
				}
			}

			if foundProblems {
				return ErrValidationFailed
			}

			_, _ = lipgloss.Printf( // lint:allow_unhandled
				"\nAll %d managed profile(s) in %s are valid.\n",
				len(results),
				clihelpers.StyleInlineHighlight.Render(awsConfigFilePath),
			)

			return nil
		},
	}
)

func init() { // lint:allow_init
	rootCmd.AddCommand(validateCmd)
}

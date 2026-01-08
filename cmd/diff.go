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

	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use: "diff",
	Short: "Generates a diff of the current AWS CLI config versus the " +
		"available SSO accounts.",
	Long: clihelpers.LongHelpText(`
	Generates a diff of the current AWS CLI config versus the available SSO accounts.

	This command helps users identify discrepancies between their existing AWS CLI
	configuration and the SSO accounts that have been set up, allowing for easier
	synchronization and management of credentials.
	`),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("diff called")
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)

	// diffCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

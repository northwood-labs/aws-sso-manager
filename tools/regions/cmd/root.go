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
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall"

	"github.com/alitto/pond/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	clihelpers "github.com/northwood-labs/cli-helpers"
)

type RegionEntry struct {
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
}

var (
	fVerbose     int
	fConcurrency int

	// Test seam to verify wrapper behavior without invoking real process exit.
	fangNotifySignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	fangExecute       = fang.Execute
	runRootCommand    = func(ctx context.Context, cmd *cobra.Command, signals ...os.Signal) error {
		return fangExecute(ctx, cmd, fang.WithNotifySignal(signals...))
	}
	osExit = os.Exit

	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "regions",
		Short: "Lists all AWS regions by name and identifier.",
		Long: clihelpers.LongHelpText(`
		Lists all AWS regions by name and identifier. This is useful for quickly
		finding the correct region identifier for use in AWS CLI commands or
		configuration.

		Should be periodically run to reflect changes in available regions.
		The output is suitable for use in scripts and can be easily parsed as
		JSON.
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			awsConfig, err := config.LoadDefaultConfig(ctx)
			if err != nil {
				return fmt.Errorf("failed to load AWS config: %w", err)
			}

			ssmClient := ssm.NewFromConfig(awsConfig)
			pool := pond.NewResultPool[string](fConcurrency)
			group := pool.NewGroup()

			p := ssm.NewGetParametersByPathPaginator(ssmClient, &ssm.GetParametersByPathInput{
				Path:       aws.String("/aws/service/global-infrastructure/regions"),
				MaxResults: aws.Int32(10),
			})

			var results []string

			for p.HasMorePages() {
				// Get the next page of results
				page, err := p.NextPage(ctx)
				if err != nil {
					return fmt.Errorf("failed to get next page of results: %w", err)
				}

				// Get a batch of results
				for _, param := range page.Parameters {
					group.Submit(func() string {
						output, err := ssmClient.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
							Path: param.Name,
						})
						if err != nil {
							panic(err)
						}

						for _, innerParam := range output.Parameters {
							if strings.HasSuffix(aws.ToString(innerParam.Name), "/longName") {
								return fmt.Sprintf(
									"%s:::%s",
									aws.ToString(param.Value),
									aws.ToString(innerParam.Value),
								)
							}
						}

						return ""
					})
				}

				results, err = group.Wait()
				if err != nil {
					return fmt.Errorf("failed to get results: %w", err)
				}
			}

			regions := make([]RegionEntry, len(results))

			for i, result := range results {
				parts := strings.SplitN(result, ":::", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid result format: %q", result)
				}

				regions[i] = RegionEntry{
					Identifier: parts[0],
					Name:       parts[1],
				}
			}

			sort.Slice(regions, func(i, j int) bool {
				return regions[i].Identifier < regions[j].Identifier
			})

			jsonData, err := json.Marshal(regions)
			if err != nil {
				return fmt.Errorf("failed to marshal regions: %w", err)
			}

			fmt.Println(string(jsonData))

			return nil
		},
	}
)

func Execute() {
	if err := runRootCommand(
		context.Background(),
		rootCmd,
		fangNotifySignals...,
	); err != nil {
		osExit(1)
	}
}

func init() {
	rootCmd.Flags().IntVarP(
		&fConcurrency, "concurrency", "c", 10, "Number of concurrent requests.",
	)
	rootCmd.PersistentFlags().CountVarP(
		&fVerbose, "verbose", "v", "Increase verbosity. Can be specified multiple times.",
	)
}

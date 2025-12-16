// Copyright 2025, Northwood Labs
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
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	configFile "github.com/northwood-labs/aws-config-parser/ini"
	clihelpers "github.com/northwood-labs/cli-helpers"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initializes AWS SSO Vault configuration.",
	Long: clihelpers.LongHelpText(`
	Initializes AWS SSO Vault configuration by setting up the SSO config for
	AWS CLI and/or AWS Vault.
	`),
	Args: cobra.RangeArgs(0, 1),
	Example: strings.TrimSpace(dedent.Dedent(`
	aws-sso-vault init
	aws-sso-vault init <sso-profile>
	`)),
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			ssoStartURL string
			ssoRegion   string
			ssoScopes   string
			profileName string
		)

		logger.Infof("Passed %d arguments.", len(args))

		if len(args) == 1 {
			profileName = args[0]
		} else {
			profileName = asvConfig.GetString("profile-name")
		}

		if profileName == "" {
			err := huh.NewInput().
				Title("SSO profile name").
				Description("should be short; no spaces").
				Value(&profileName).
				Run()
			if err != nil {
				return err
			}
		}

		if asvConfig.Get("sso-start-url") != nil {
			ssoStartURL = asvConfig.Get("sso-start-url").(string)
		}

		if asvConfig.Get("sso-region") != nil {
			ssoRegion = asvConfig.Get("sso-region").(string)
		}

		if asvConfig.Get("sso-scopes") != nil {
			ssoScopes = asvConfig.Get("sso-scopes").(string)
		}

		logger.Infof("Read the AWS config file at %s.", awsConfigFilePath)

		sections, err := loadAWSConfig(awsConfigFilePath)
		cobra.CheckErr(err)

		sessionName := fmt.Sprintf("sso-session %s", profileName)

		logger.Infof("Load the session section and update values for [%s].", sessionName)

		section, ok := sections.GetSection(sessionName)
		if !ok {
			logger.Infof("Config file does not have section [%s]; creating new section.", sessionName)

			section = configFile.NewSection(sessionName)
		}

		logger.Info("Ask for SSO start URL if not provided already.")

		if ssoStartURL == "" {
			def := section.String("sso_start_url")
			if def == "" {
				def = "https://*.awsapps.com/start"
			}

			err := huh.NewInput().
				Title("SSO start URL?").
				Description("e.g., " + def).
				Value(&ssoStartURL).
				Suggestions([]string{def}).
				Placeholder(def).
				Run()
			cobra.CheckErr(err)
		}

		logger.Info("Ask for SSO region if not provided already.")

		if ssoRegion == "" {
			def := section.String("sso_region")
			if def == "" {
				def = "us-east-1"
			}

			err := huh.NewInput().
				Title("SSO region?").
				Description("e.g., " + def).
				Value(&ssoRegion).
				Suggestions([]string{def}).
				Placeholder(def).
				Run()
			cobra.CheckErr(err)
		}

		logger.Infof("Get the [%s] section or create it if it does not exist.", sessionName)

		sections = sections.SetSection(sessionName, section)
		if section, ok = sections.GetSection(sessionName); !ok {
			return fmt.Errorf("failed to create or get section [%s]", sessionName)
		}

		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered:", r)
			}
		}()

		logger.Info("Create ssoStartURL entry.")

		if v, err := configFile.NewStringValue(ssoStartURL); err != nil {
			return fmt.Errorf("failed to create 'sso_start_url' value: %w", err)
		} else {
			err = section.UpdateValue("sso_start_url", v)
			if err != nil {
				return fmt.Errorf("failed to update 'sso_start_url' value: %w", err)
			}
		}

		logger.Info("Create ssoRegion entry.")

		if v, err := configFile.NewStringValue(ssoRegion); err != nil {
			return fmt.Errorf("failed to create 'sso_region' value: %w", err)
		} else {
			err = section.UpdateValue("sso_region", v)
			if err != nil {
				return fmt.Errorf("failed to update 'sso_region' value: %w", err)
			}
		}

		logger.Info("Create ssoScope entry.")

		if v, err := configFile.NewStringValue(ssoScopes); err != nil {
			return fmt.Errorf("failed to create 'sso_registration_scopes' value: %w", err)
		} else {
			err = section.UpdateValue("sso_registration_scopes", v)
			if err != nil {
				return fmt.Errorf("failed to update 'sso_registration_scopes' value: %w", err)
			}
		}

		logger.Info("Write the configuration to disk.")

		f, err := os.OpenFile(awsConfigFilePath, os.O_APPEND|os.O_WRONLY, 0o0644)
		cobra.CheckErr(err)

		defer func() {
			err = f.Close()
			cobra.CheckErr(err)
		}()

		_, err = f.WriteString("; -------- aws-sso-vault: start " + profileName + " --------\n")
		cobra.CheckErr(err)

		_, err = f.WriteString(generateAWSConfig(sections))
		cobra.CheckErr(err)

		_, err = f.WriteString("; -------- aws-sso-vault: end " + profileName + " --------\n")
		cobra.CheckErr(err)

		fmt.Printf("Successfully initialized SSO configuration in %s\n", awsConfigFilePath)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().
		StringP("sso-start-url", "u", "", "The start URL for the AWS SSO portal (e.g., https://*.awsapps.com/start)")
	initCmd.Flags().StringP("sso-region", "r", "", "The AWS region where AWS SSO is configured (e.g., us-east-1)")
	initCmd.Flags().
		StringP("sso-scopes", "s", "sso:account:access", "The AWS SSO scope to request during authentication")
}

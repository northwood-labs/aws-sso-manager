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
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	configFile "github.com/northwood-labs/aws-config-parser/ini"
	clihelpers "github.com/northwood-labs/cli-helpers"
)

// initCmd sets up a new SSO session in ~/.aws/config. It's the first command a
// user runs for each AWS Organization — it creates the [sso-session] section
// inside a managed block so that subsequent "update" calls know which region of
// the config file to regenerate.
var initCmd = &cobra.Command{
	Use:   "init [sso-profile-name]",
	Short: "Initializes AWS SSO Manager configuration.",
	Long: clihelpers.LongHelpText(`
	Initializes AWS SSO Manager configuration by setting up the SSO config for
	AWS CLI and/or AWS Vault.
	`),
	Args: cobra.RangeArgs(0, 1),
	Example: strings.TrimSpace(dedent.Dedent(`
	# Initialize a new SSO profile with a short name.
	aws-sso-manager init <sso-profile-name>
	`)),
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			ssoStartURL string
			ssoRegion   string
			ssoScopes   string
			profileName string
		)

		logger.Info("Passed arguments", "count", len(args))

		if len(args) == 1 {
			profileName = args[0]
		} else {
			profileName = asmConfig.GetString("profile-name")
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

		if asmConfig.Get("sso-start-url") != nil {
			ssoStartURL = asmConfig.Get("sso-start-url").(string)
		}

		if asmConfig.Get("sso-region") != nil {
			ssoRegion = asmConfig.Get("sso-region").(string)
		}

		if asmConfig.Get("sso-scopes") != nil {
			ssoScopes = asmConfig.Get("sso-scopes").(string)
		}

		configLock, err := acquireAWSConfigLock(cmd.Context())
		if err != nil {
			return err
		}
		defer func() {
			if releaseErr := configLock.Release(); releaseErr != nil {
				logger.Error("Failed to release AWS config lock", "error", releaseErr)
			}
		}()

		logger.Info("Read the AWS config file", "config", awsConfigFilePath)

		sections, err := loadAWSConfig(awsConfigFilePath)
		cobra.CheckErr(err)

		sessionName := fmt.Sprintf("sso-session %s", profileName)

		logger.Info("Load the session section and check for existing section", "section", sessionName)

		_, ok := sections.GetSection(sessionName)
		if ok {
			return fmt.Errorf("config file already contains [%s] section. Delete it from the "+
				"config file and re-run `init`", sessionName)
		}

		// Guard against orphaned markers: the section header may have been manually
		// removed while the managed-block markers remain. Appending new markers in
		// that state would create a duplicate block.
		if exists, err := markersExist(profileName); err != nil {
			return err
		} else if exists {
			return fmt.Errorf(
				"config file already contains managed block markers for profile %q; "+
					"remove the markers and re-run `init`",
				profileName,
			)
		}

		// -------------------------------------------------------------------------------------------------------------

		section := configFile.NewSection(sessionName)

		logger.Info("Ask for SSO start URL if not provided already.")

		if ssoStartURL == "" {
			def := section.String("sso_start_url")
			if def == "" {
				def = "my-organization"
			}

			err := huh.NewInput().
				Title("SSO start URL?").
				Description("e.g., https://my-organization.awsapps.com/start or just 'my-organization'").
				Value(&ssoStartURL).
				Suggestions([]string{def}).
				Placeholder(def).
				Run()
			cobra.CheckErr(err)
		}

		normalizedStartURL, err := normalizeSSOStartURL(ssoStartURL)
		if err != nil {
			return fmt.Errorf("invalid SSO start URL: %w", err)
		}
		ssoStartURL = normalizedStartURL

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

		existingConfig, err := os.ReadFile(awsConfigFilePath)
		cobra.CheckErr(err)

		tmpConfig, err := os.CreateTemp(filepath.Dir(awsConfigFilePath), ".aws-sso-manager-init-*.ini")
		cobra.CheckErr(err)
		tmpConfigPath := tmpConfig.Name()

		defer func() {
			if tmpConfig == nil {
				return
			}

			if closeErr := tmpConfig.Close(); closeErr != nil {
				logger.Error("Failed to close temporary AWS config file", "error", closeErr)
			}
		}()

		if _, err = tmpConfig.Write(existingConfig); err != nil {
			return fmt.Errorf("write existing AWS config to temporary file: %w", err)
		}

		if len(existingConfig) > 0 && existingConfig[len(existingConfig)-1] != '\n' {
			if _, err = tmpConfig.WriteString("\n"); err != nil {
				return fmt.Errorf("write AWS config separator newline: %w", err)
			}
		}

		managedBlock := strings.Join([]string{
			"; -------- aws-sso-manager: start " + profileName + " --------",
			strings.TrimSpace(generateSingleAWSConfig(section)),
			"; -------- aws-sso-manager: end " + profileName + " --------",
		}, "\n") + "\n"

		if _, err = tmpConfig.WriteString(managedBlock); err != nil {
			return fmt.Errorf("write managed AWS config block: %w", err)
		}

		if err = tmpConfig.Chmod(0o0644); err != nil {
			return fmt.Errorf("set permissions on temporary AWS config file: %w", err)
		}

		if err = tmpConfig.Close(); err != nil {
			return fmt.Errorf("close temporary AWS config file: %w", err)
		}

		tmpConfig = nil

		if err = os.Rename(tmpConfigPath, awsConfigFilePath); err != nil {
			return fmt.Errorf("replace AWS config with initialized config: %w", err)
		}

		fmt.Printf("Successfully initialized SSO configuration in %s\n", awsConfigFilePath)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringP(
		"sso-start-url",
		"u",
		"",
		"The start URL for the AWS SSO portal, or just the awsapps subdomain "+
			"(e.g., `https://my-organization.awsapps.com/start` or `my-organization`)",
	)
	initCmd.Flags().StringP(
		"sso-region", "r", "", "The AWS region where AWS SSO is configured (e.g., us-east-1)",
	)
	initCmd.Flags().StringP(
		"sso-scopes", "s", "sso:account:access", "The AWS SSO scope to request during authentication",
	)
}

// normalizeSSOStartURL accepts multiple shorthand formats for the SSO start URL
// so users don't have to remember the full URL. AWS Identity Center URLs follow
// a predictable pattern, so we can expand bare subdomains ("my-org") into full
// URLs ("https://my-org.awsapps.com/start"). The resolution order handles
// progressively more complete inputs: bare subdomain → dotted host → host/path
// → full URL.
func normalizeSSOStartURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("value cannot be empty")
	}

	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", fmt.Errorf("could not parse %q: %w", raw, err)
		}

		if parsed.Scheme == "" || parsed.Host == "" {
			return "", fmt.Errorf("expected a full URL with scheme and host, got %q", raw)
		}

		return trimmed, nil
	}

	if strings.Contains(trimmed, "/") {
		candidate := "https://" + trimmed

		parsed, err := url.Parse(candidate)
		if err != nil {
			return "", fmt.Errorf("could not parse %q: %w", raw, err)
		}

		if parsed.Host == "" {
			return "", fmt.Errorf("expected a host or subdomain, got %q", raw)
		}

		return candidate, nil
	}

	if strings.Contains(trimmed, ".") {
		return "https://" + trimmed + "/start", nil
	}

	return fmt.Sprintf("https://%s.awsapps.com/start", trimmed), nil
}

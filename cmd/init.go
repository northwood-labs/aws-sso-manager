// Copyright 2025-2026, Northwood Labs, LLC <license@northwood-labs.com>
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
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	configfile "github.com/northwood-labs/aws-config-parser/ini"
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
		ssoStartURL := ""
		ssoRegion := ""
		ssoScopes := ""
		profileName := ""

		ctx := cmd.Context()

		logger.InfoContext(ctx, "Passed arguments", logKeyCount, len(args))

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
				return fmt.Errorf("could not read SSO profile name: %w", err)
			}
		}

		if v := asmConfig.Get("sso-start-url"); v != nil {
			if s, ok := v.(string); ok {
				ssoStartURL = s
			}
		}

		if v := asmConfig.Get("sso-region"); v != nil {
			if s, ok := v.(string); ok {
				ssoRegion = s
			}
		}

		if v := asmConfig.Get("sso-scopes"); v != nil {
			if s, ok := v.(string); ok {
				ssoScopes = s
			}
		}

		configLock, err := acquireAWSConfigLock(cmd.Context())
		if err != nil {
			return fmt.Errorf("could not acquire AWS config lock: %w", err)
		}
		defer func() {
			releaseErr := configLock.Release()
			if releaseErr != nil {
				logger.ErrorContext(ctx, "Failed to release AWS config lock", logKeyErr, releaseErr)
			}
		}()

		logger.InfoContext(ctx, "Read the AWS config file", logKeyConfig, awsConfigFilePath)

		sections, err := loadAWSConfig(ctx, awsConfigFilePath)
		cobra.CheckErr(err)

		sessionName := "sso-session " + profileName

		logger.InfoContext(ctx, "Load the session section and check for existing section", logKeySection, sessionName)

		_, ok := sections.GetSection(sessionName)
		if ok {
			return fmt.Errorf("%w: [%s]; delete it and re-run init", ErrConfigSectionExists, sessionName)
		}

		// Guard against orphaned markers: the section header may have been manually
		// removed while the managed-block markers remain. Appending new markers in
		// that state would create a duplicate block.
		if exists, markerErr := markersExist(profileName); markerErr != nil {
			return fmt.Errorf("could not check managed markers: %w", markerErr)
		} else if exists {
			return fmt.Errorf(
				"%w: profile %q; remove the markers and re-run init",
				ErrConfigMarkersExist,
				profileName,
			)
		}

		// -------------------------------------------------------------------------------------------------------------

		section := configfile.NewSection(sessionName)

		logger.InfoContext(ctx, "Ask for SSO start URL if not provided already.")

		if ssoStartURL == "" {
			def := section.String("sso_start_url")
			if def == "" {
				def = "my-organization"
			}

			err = huh.NewInput().
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

		logger.InfoContext(ctx, "Ask for SSO region if not provided already.")

		if ssoRegion == "" {
			def := section.String("sso_region")
			if def == "" {
				def = "us-east-1"
			}

			err = huh.NewInput().
				Title("SSO region?").
				Description("e.g., " + def).
				Value(&ssoRegion).
				Suggestions([]string{def}).
				Placeholder(def).
				Run()
			cobra.CheckErr(err)
		}

		logger.InfoContext(ctx, "Create ssoStartURL entry.")

		v, err := configfile.NewStringValue(ssoStartURL)
		if err != nil {
			return fmt.Errorf("failed to create 'sso_start_url' value: %w", err)
		}

		err = section.UpdateValue("sso_start_url", v)
		if err != nil {
			return fmt.Errorf("failed to update 'sso_start_url' value: %w", err)
		}

		logger.InfoContext(ctx, "Create ssoRegion entry.")

		v, err = configfile.NewStringValue(ssoRegion)
		if err != nil {
			return fmt.Errorf("failed to create 'sso_region' value: %w", err)
		}

		err = section.UpdateValue("sso_region", v)
		if err != nil {
			return fmt.Errorf("failed to update 'sso_region' value: %w", err)
		}

		logger.InfoContext(ctx, "Create ssoScope entry.")

		v, err = configfile.NewStringValue(ssoScopes)
		if err != nil {
			return fmt.Errorf("failed to create 'sso_registration_scopes' value: %w", err)
		}

		err = section.UpdateValue("sso_registration_scopes", v)
		if err != nil {
			return fmt.Errorf("failed to update 'sso_registration_scopes' value: %w", err)
		}

		logger.InfoContext(ctx, "Write the configuration to disk.")

		existingConfig, err := os.ReadFile(awsConfigFilePath)
		cobra.CheckErr(err)

		tmpConfig, err := os.CreateTemp(filepath.Dir(awsConfigFilePath), ".aws-sso-manager-init-*.ini")
		cobra.CheckErr(err)

		tmpConfigPath := tmpConfig.Name()

		defer func() {
			if tmpConfig == nil {
				return
			}

			closeErr := tmpConfig.Close()
			if closeErr != nil {
				logger.ErrorContext(ctx, "Failed to close temporary AWS config file", logKeyErr, closeErr)
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
			strings.TrimSpace(generateSingleAWSConfig(&section)),
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

func init() { // lint:allow_init
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
		return "", ErrValueEmpty
	}

	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", fmt.Errorf("could not parse %q: %w", raw, err)
		}

		if parsed.Scheme == "" || parsed.Host == "" {
			return "", fmt.Errorf("%w: got %q", ErrStartURLInvalid, raw)
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
			return "", fmt.Errorf("%w: got %q", ErrStartURLHostInvalid, raw)
		}

		return candidate, nil
	}

	if strings.Contains(trimmed, ".") {
		return "https://" + trimmed + "/start", nil
	}

	return fmt.Sprintf("https://%s.awsapps.com/start", trimmed), nil
}

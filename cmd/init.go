package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			ssoStartURL string
			ssoRegion   string
			ssoScopes   string
			profileName string
		)

		if asvConfig.Get("sso-start-url") != nil {
			ssoStartURL = asvConfig.Get("sso-start-url").(string)
		}

		if asvConfig.Get("sso-region") != nil {
			ssoRegion = asvConfig.Get("sso-region").(string)
		}

		if asvConfig.Get("sso-scopes") != nil {
			ssoScopes = asvConfig.Get("sso-scopes").(string)
		}

		if asvConfig.Get("profile-name") != nil {
			profileName = asvConfig.Get("profile-name").(string)
		}

		logger.Infof("Read the AWS config file at %s.", awsConfigFilePath)

		sections, err := loadAWSConfig(awsConfigFilePath)
		cobra.CheckErr(err)

		if profileName == "" {
			err := huh.NewInput().
				Title("SSO profile name").
				Description("should be short; no spaces").
				Value(&profileName).
				Run()
			cobra.CheckErr(err)
		}

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

		_, err = f.WriteString(generateAWSConfig(sections))
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
	initCmd.Flags().StringP("profile-name", "n", "", "The name of this SSO profile (recommend something short)")
}

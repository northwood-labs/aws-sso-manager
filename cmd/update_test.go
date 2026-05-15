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
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"charm.land/log/v2"
	configFile "github.com/northwood-labs/aws-config-parser/ini"
	"github.com/spf13/viper"
	"pgregory.net/rapid"
)

func mustSetStringValue(
	t *testing.T,
	section *configFile.Section,
	key, value string,
) {
	t.Helper()

	v, err := configFile.NewStringValue(value)
	if err != nil {
		t.Fatalf("new string value for %q: %v", key, err)
	}

	if err := section.UpdateValue(key, v); err != nil {
		t.Fatalf("update value for %q: %v", key, err)
	}
}

func assertHeadersInOrder(t *testing.T, content string, headers ...string) {
	t.Helper()

	lastIdx := -1

	for _, header := range headers {
		idx := strings.Index(content, header)
		if idx < 0 {
			t.Fatalf("expected header %q to exist:\n%s", header, content)
		}

		if idx <= lastIdx {
			t.Fatalf("expected headers in order %v:\n%s", headers, content)
		}

		lastIdx = idx
	}
}

func TestBuildUpdatedManagedSectionsDropsStaleProfiles(t *testing.T) {
	ssoProfile := "sso-session myprofile"

	ssoSection := configFile.NewSection(ssoProfile)

	mustSetStringValue(t, &ssoSection, "sso_region", "us-east-1")

	staleSection := configFile.NewSection("profile stale-role")

	mustSetStringValue(t, &staleSection, "sso_session", "myprofile")
	mustSetStringValue(t, &staleSection, "sso_account_id", "999999999999")
	mustSetStringValue(t, &staleSection, "sso_role_name", "OldRole")
	mustSetStringValue(t, &staleSection, "region", "us-east-1")
	mustSetStringValue(t, &staleSection, "output", "json")

	sections := configFile.NewSections()

	sections = sections.SetSection(ssoProfile, ssoSection)
	sections = sections.SetSection("profile stale-role", staleSection)

	accounts := listAccounts{
		Accounts: []listAccount{
			{
				ID: "111111111111",
				Roles: []listRole{
					{AccountID: "111111111111", Name: "ReadOnly", Profile: "new-readonly"},
				},
			},
		},
	}

	nextSections, count, err := buildUpdatedManagedSections(sections, ssoProfile, "myprofile", accounts)
	if err != nil {
		t.Fatalf("buildUpdatedManagedSections: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 updated profile, got %d", count)
	}

	if _, ok := nextSections.GetSection(ssoProfile); !ok {
		t.Fatalf("expected %q section to be preserved", ssoProfile)
	}

	if _, ok := nextSections.GetSection("profile new-readonly"); !ok {
		t.Fatalf("expected new profile section to exist")
	}

	if _, ok := nextSections.GetSection("profile stale-role"); ok {
		t.Fatalf("expected stale profile section to be dropped")
	}
}

func TestBuildUpdatedManagedSectionsUpdatesRoleFields(t *testing.T) {
	ssoProfile := "sso-session myprofile"

	ssoSection := configFile.NewSection(ssoProfile)

	mustSetStringValue(t, &ssoSection, "sso_region", "us-west-2")

	existingSection := configFile.NewSection("profile dev-admin")

	mustSetStringValue(t, &existingSection, "sso_session", "wrong")
	mustSetStringValue(t, &existingSection, "sso_account_id", "000000000000")
	mustSetStringValue(t, &existingSection, "sso_role_name", "OldRole")
	mustSetStringValue(t, &existingSection, "region", "us-east-1")
	mustSetStringValue(t, &existingSection, "output", "text")

	sections := configFile.NewSections()

	sections = sections.SetSection(ssoProfile, ssoSection)
	sections = sections.SetSection("profile dev-admin", existingSection)

	accounts := listAccounts{
		Accounts: []listAccount{
			{
				ID: "222222222222",
				Roles: []listRole{
					{AccountID: "222222222222", Name: "AdministratorAccess", Profile: "dev-admin"},
				},
			},
		},
	}

	nextSections, _, err := buildUpdatedManagedSections(sections, ssoProfile, "myprofile", accounts)
	if err != nil {
		t.Fatalf("buildUpdatedManagedSections: %v", err)
	}

	profileSection, ok := nextSections.GetSection("profile dev-admin")
	if !ok {
		t.Fatalf("expected updated profile section to exist")
	}

	if got := profileSection.String("sso_session"); got != "myprofile" {
		t.Fatalf("expected sso_session=myprofile, got %q", got)
	}

	if got := profileSection.String("sso_account_id"); got != "222222222222" {
		t.Fatalf("expected sso_account_id=222222222222, got %q", got)
	}

	if got := profileSection.String("sso_role_name"); got != "AdministratorAccess" {
		t.Fatalf("expected sso_role_name=AdministratorAccess, got %q", got)
	}

	if got := profileSection.String("region"); got != "us-west-2" {
		t.Fatalf("expected region=us-west-2, got %q", got)
	}

	if got := profileSection.String("output"); got != "json" {
		t.Fatalf("expected output=json, got %q", got)
	}
}

func TestUpdateManagedBlockRewriteIntegration(t *testing.T) {
	logger = slog.New(log.New(io.Discard))

	oldConfigPath := awsConfigFilePath

	t.Cleanup(func() { awsConfigFilePath = oldConfigPath })

	dir := t.TempDir()

	awsConfigFilePath = filepath.Join(dir, "config")

	initialConfig := strings.Join([]string{
		"[default]",
		"region = us-east-1",
		"",
		"; -------- aws-sso-manager: start nwl2 --------",
		"[profile stale-role]",
		"output = json",
		"region = us-east-2",
		"sso_account_id = 999999999999",
		"sso_role_name = OldRole",
		"sso_session = nwl2",
		"",
		"[sso-session nwl2]",
		"sso_region = us-east-2",
		"sso_registration_scopes = sso:account:access",
		"sso_start_url = https://northwood-labs.awsapps.com/start",
		"; -------- aws-sso-manager: end nwl2 --------",
	}, "\n") + "\n"

	if err := os.WriteFile(awsConfigFilePath, []byte(initialConfig), 0o0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	sections, err := loadAWSConfig(context.Background(), awsConfigFilePath)
	if err != nil {
		t.Fatalf("loadAWSConfig: %v", err)
	}

	accounts := listAccounts{
		Accounts: []listAccount{
			{
				ID: "111111111111",
				Roles: []listRole{
					{AccountID: "111111111111", Name: "PowerUserAccess", Profile: "sandbox-poweruseraccess"},
					{AccountID: "111111111111", Name: "ReadOnlyAccess", Profile: "sandbox-readonlyaccess"},
				},
			},
		},
	}

	nextSections, count, err := buildUpdatedManagedSections(sections, "sso-session nwl2", "nwl2", accounts)
	if err != nil {
		t.Fatalf("buildUpdatedManagedSections: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 generated profiles, got %d", count)
	}

	tmpManaged := filepath.Join(dir, "managed.ini")
	if writeErr := os.WriteFile(
		tmpManaged,
		[]byte(strings.TrimSpace(generateAWSConfig(nextSections))+"\n"),
		0o0644,
	); writeErr != nil {
		t.Fatalf("write managed tmp: %v", writeErr)
	}

	backupName, err := setManagedSection(tmpManaged, "nwl2")
	if err != nil {
		t.Fatalf("setManagedSection: %v", err)
	}

	content, err := os.ReadFile(backupName) // lint:allow_dynamic_filename
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}

	got := string(content)

	if strings.Contains(got, "[profile ]") {
		t.Fatalf("did not expect empty profile name in managed block:\n%s", got)
	}

	if strings.Contains(got, "[profile stale-role]") {
		t.Fatalf("expected stale profile to be removed from managed block:\n%s", got)
	}

	if !strings.Contains(got, "[profile sandbox-poweruseraccess]") ||
		!strings.Contains(got, "[profile sandbox-readonlyaccess]") {
		t.Fatalf("expected generated role profiles in managed block:\n%s", got)
	}

	if strings.Count(got, "[profile sandbox-poweruseraccess]") != 1 ||
		strings.Count(got, "[profile sandbox-readonlyaccess]") != 1 {
		t.Fatalf("expected each generated profile exactly once:\n%s", got)
	}

	if !strings.Contains(got, "[sso-session nwl2]") {
		t.Fatalf("expected managed block to preserve [sso-session nwl2]:\n%s", got)
	}

	if !strings.Contains(got, "sso_start_url = https://northwood-labs.awsapps.com/start") {
		t.Fatalf("expected managed block to preserve SSO start URL:\n%s", got)
	}

	// generateAWSConfig iterates sections.List(), which sorts section names; the
	// rendered managed block should therefore stay in deterministic order.
	assertHeadersInOrder(
		t,
		got,
		"[profile sandbox-poweruseraccess]",
		"[profile sandbox-readonlyaccess]",
		"[sso-session nwl2]",
	)

	backupName2, err := setManagedSection(tmpManaged, "nwl2")
	if err != nil {
		t.Fatalf("setManagedSection second pass: %v", err)
	}

	content2, err := os.ReadFile(backupName2) // lint:allow_dynamic_filename
	if err != nil {
		t.Fatalf("read backup second pass: %v", err)
	}

	got2 := string(content2)
	if got != got2 {
		t.Fatalf("expected deterministic output across repeated rewrites\nfirst:\n%s\nsecond:\n%s", got, got2)
	}
}

// Feature: aws-sso-manager, Property 17: Update Managed Section Generation
func TestPropertyUpdateManagedSectionGeneration(t *testing.T) { // lint:allow_complexity
	logger = slog.New(log.New(io.Discard))

	rapid.Check(t, func(t *rapid.T) {
		accounts := genListAccounts(1, 5).Draw(t, "accounts")
		profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,9}`).Draw(t, "profileName")
		ssoRegion := rapid.SampledFrom([]string{
			"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1",
		}).Draw(t, "ssoRegion")

		// Build a valid SSO session section with sso_region set.
		ssoProfile := "sso-session " + profileName
		ssoSection := configFile.NewSection(ssoProfile)

		regionVal, err := configFile.NewStringValue(ssoRegion)
		if err != nil {
			t.Fatalf("new string value for sso_region: %v", err)
		}

		if updateErr := ssoSection.UpdateValue("sso_region", regionVal); updateErr != nil {
			t.Fatalf("update sso_region: %v", updateErr)
		}

		sections := configFile.NewSections()

		sections = sections.SetSection(ssoProfile, ssoSection)

		// Call buildUpdatedManagedSections.
		nextSections, count, err := buildUpdatedManagedSections(sections, ssoProfile, profileName, accounts)
		if err != nil {
			t.Fatalf("buildUpdatedManagedSections: %v", err)
		}

		// Count total account-role combinations.
		totalRoles := 0
		for _, acct := range accounts.Accounts {
			totalRoles += len(acct.Roles)
		}

		// Verify count equals total account-role combinations.
		if count != totalRoles {
			t.Fatalf("expected count=%d, got %d", totalRoles, count)
		}

		expectedKeys := []string{"sso_session", "sso_account_id", "sso_role_name", "region", "output"}
		sort.Strings(expectedKeys)

		// For each account-role, verify a [profile <name>] section exists with exactly the right keys.
		for _, acct := range accounts.Accounts {
			for _, role := range acct.Roles {
				profileHeader := "profile " + role.Profile

				section, ok := nextSections.GetSection(profileHeader)
				if !ok {
					t.Fatalf("expected section [%s] to exist", profileHeader)
				}

				gotKeys := section.List()
				sort.Strings(gotKeys)

				if len(gotKeys) != len(expectedKeys) {
					t.Fatalf("section [%s]: expected keys %v, got %v", profileHeader, expectedKeys, gotKeys)
				}

				for i, k := range expectedKeys {
					if gotKeys[i] != k {
						t.Fatalf("section [%s]: expected keys %v, got %v", profileHeader, expectedKeys, gotKeys)
					}
				}

				// Verify key values.
				if got := section.String("sso_session"); got != profileName {
					t.Fatalf("section [%s]: expected sso_session=%q, got %q", profileHeader, profileName, got)
				}

				if got := section.String("sso_account_id"); got != role.AccountID {
					t.Fatalf("section [%s]: expected sso_account_id=%q, got %q", profileHeader, role.AccountID, got)
				}

				if got := section.String("sso_role_name"); got != role.Name {
					t.Fatalf("section [%s]: expected sso_role_name=%q, got %q", profileHeader, role.Name, got)
				}

				if got := section.String("region"); got != ssoRegion {
					t.Fatalf("section [%s]: expected region=%q, got %q", profileHeader, ssoRegion, got)
				}

				if got := section.String("output"); got != "json" {
					t.Fatalf("section [%s]: expected output=json, got %q", profileHeader, got)
				}
			}
		}
	})
}

// Feature: config-output-region-overrides, Property 1: Region resolution
// respects config override with sso_region fallback
func TestPropertyRegionOverrideResolution(t *testing.T) { // lint:allow_complexity
	// **Validates: Requirements 1.1, 1.2, 1.3, 4.1**
	logger = slog.New(log.New(io.Discard))

	oldConfig := asmConfig

	t.Cleanup(func() { asmConfig = oldConfig })

	rapid.Check(t, func(t *rapid.T) {
		accounts := genListAccounts(1, 5).Draw(t, "accounts")
		profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,9}`).Draw(t, "profileName")
		ssoRegion := rapid.SampledFrom([]string{
			"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1",
		}).Draw(t, "ssoRegion")

		// Decide whether a settings.region override is present.
		hasOverride := rapid.Bool().Draw(t, "hasRegionOverride")

		overrideRegion := ""
		if hasOverride {
			overrideRegion = rapid.SampledFrom([]string{
				"us-east-2", "eu-central-1", "ap-northeast-1", "sa-east-1",
			}).Draw(t, "overrideRegion")
		}

		// Configure a fresh Viper instance with the optional override.
		asmConfig = viper.New()
		if hasOverride {
			asmConfig.Set(profileName+".settings.global.region", overrideRegion)
		}

		// Build a valid SSO session section with sso_region set.
		ssoProfile := "sso-session " + profileName
		ssoSection := configFile.NewSection(ssoProfile)

		regionVal, err := configFile.NewStringValue(ssoRegion)
		if err != nil {
			t.Fatalf("new string value for sso_region: %v", err)
		}

		if updateErr := ssoSection.UpdateValue("sso_region", regionVal); updateErr != nil {
			t.Fatalf("update sso_region: %v", updateErr)
		}

		sections := configFile.NewSections()

		sections = sections.SetSection(ssoProfile, ssoSection)

		// Call buildUpdatedManagedSections.
		nextSections, _, err := buildUpdatedManagedSections(sections, ssoProfile, profileName, accounts)
		if err != nil {
			t.Fatalf("buildUpdatedManagedSections: %v", err)
		}

		// Determine expected region: override when non-empty, else sso_region.
		expectedRegion := ssoRegion
		if overrideRegion != "" {
			expectedRegion = overrideRegion
		}

		// Assert every generated profile's region field equals the expected value.
		for _, acct := range accounts.Accounts {
			for _, role := range acct.Roles {
				profileHeader := "profile " + role.Profile

				section, ok := nextSections.GetSection(profileHeader)
				if !ok {
					t.Fatalf("expected section [%s] to exist", profileHeader)
				}

				if got := section.String("region"); got != expectedRegion {
					t.Fatalf(
						"section [%s]: expected region=%q, got %q (hasOverride=%v, overrideRegion=%q, ssoRegion=%q)",
						profileHeader, expectedRegion, got, hasOverride, overrideRegion, ssoRegion,
					)
				}
			}
		}
	})
}

// Feature: config-output-region-overrides, Property 2: Output resolution respects config override with "json" fallback
func TestPropertyOutputOverrideResolution(t *testing.T) { // lint:allow_complexity
	// **Validates: Requirements 2.1, 2.2, 2.3, 4.2**
	logger = slog.New(log.New(io.Discard))

	oldConfig := asmConfig

	t.Cleanup(func() { asmConfig = oldConfig })

	rapid.Check(t, func(t *rapid.T) {
		accounts := genListAccounts(1, 5).Draw(t, "accounts")
		profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,9}`).Draw(t, "profileName")
		ssoRegion := rapid.SampledFrom([]string{
			"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1",
		}).Draw(t, "ssoRegion")

		// Decide whether a settings.output override is present.
		hasOverride := rapid.Bool().Draw(t, "hasOutputOverride")

		overrideOutput := ""
		if hasOverride {
			overrideOutput = rapid.SampledFrom([]string{
				"json", "text", "table", "yaml", "yaml-stream",
			}).Draw(t, "overrideOutput")
		}

		// Configure a fresh Viper instance with the optional override.
		asmConfig = viper.New()
		if hasOverride {
			asmConfig.Set(profileName+".settings.global.output", overrideOutput)
		}

		// Build a valid SSO session section with sso_region set.
		ssoProfile := "sso-session " + profileName
		ssoSection := configFile.NewSection(ssoProfile)

		regionVal, err := configFile.NewStringValue(ssoRegion)
		if err != nil {
			t.Fatalf("new string value for sso_region: %v", err)
		}

		if updateErr := ssoSection.UpdateValue("sso_region", regionVal); updateErr != nil {
			t.Fatalf("update sso_region: %v", updateErr)
		}

		sections := configFile.NewSections()

		sections = sections.SetSection(ssoProfile, ssoSection)

		nextSections, _, err := buildUpdatedManagedSections(sections, ssoProfile, profileName, accounts)
		if err != nil {
			t.Fatalf("buildUpdatedManagedSections: %v", err)
		}

		// Expected output: override when non-empty, else "json".
		expectedOutput := "json"
		if overrideOutput != "" {
			expectedOutput = overrideOutput
		}

		for _, acct := range accounts.Accounts {
			for _, role := range acct.Roles {
				profileHeader := "profile " + role.Profile

				section, ok := nextSections.GetSection(profileHeader)
				if !ok {
					t.Fatalf("expected section [%s] to exist", profileHeader)
				}

				if got := section.String("output"); got != expectedOutput {
					t.Fatalf(
						"section [%s]: expected output=%q, got %q (hasOverride=%v, overrideOutput=%q)",
						profileHeader, expectedOutput, got, hasOverride, overrideOutput,
					)
				}
			}
		}
	})
}

// Feature: config-output-region-overrides, Property 4: Update idempotence with settings overrides
func TestPropertyUpdateIdempotenceWithOverrides(t *testing.T) {
	// **Validates: Requirements 5.1**
	logger = slog.New(log.New(io.Discard))

	oldConfig := asmConfig

	t.Cleanup(func() { asmConfig = oldConfig })

	rapid.Check(t, func(t *rapid.T) {
		accounts := genListAccounts(1, 5).Draw(t, "accounts")
		profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,9}`).Draw(t, "profileName")
		ssoRegion := rapid.SampledFrom([]string{
			"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1",
		}).Draw(t, "ssoRegion")

		// Configure random settings overrides.
		asmConfig = viper.New()

		if rapid.Bool().Draw(t, "hasRegionOverride") {
			region := rapid.SampledFrom([]string{
				"us-east-2", "eu-central-1", "ap-northeast-1",
			}).Draw(t, "overrideRegion")
			asmConfig.Set(profileName+".settings.global.region", region)
		}

		if rapid.Bool().Draw(t, "hasOutputOverride") {
			output := rapid.SampledFrom([]string{
				"json", "text", "table", "yaml", "yaml-stream",
			}).Draw(t, "overrideOutput")
			asmConfig.Set(profileName+".settings.global.output", output)
		}

		// Build SSO session section.
		ssoProfile := "sso-session " + profileName
		ssoSection := configFile.NewSection(ssoProfile)

		regionVal, err := configFile.NewStringValue(ssoRegion)
		if err != nil {
			t.Fatalf("new string value for sso_region: %v", err)
		}

		if updateErr := ssoSection.UpdateValue("sso_region", regionVal); updateErr != nil {
			t.Fatalf("update sso_region: %v", updateErr)
		}

		sections := configFile.NewSections()

		sections = sections.SetSection(ssoProfile, ssoSection)

		// Call twice with identical inputs.
		sections1, count1, err := buildUpdatedManagedSections(sections, ssoProfile, profileName, accounts)
		if err != nil {
			t.Fatalf("first call: %v", err)
		}

		sections2, count2, err := buildUpdatedManagedSections(sections, ssoProfile, profileName, accounts)
		if err != nil {
			t.Fatalf("second call: %v", err)
		}

		if count1 != count2 {
			t.Fatalf("counts differ: %d vs %d", count1, count2)
		}

		// Compare rendered output.
		out1 := generateAWSConfig(sections1)

		out2 := generateAWSConfig(sections2)
		if out1 != out2 {
			t.Fatalf("outputs differ:\n--- first ---\n%s\n--- second ---\n%s", out1, out2)
		}
	})
}

// Feature: config-output-region-overrides, Property 5: Per-profile settings override global settings
func TestPropertyPerProfileOverridePrecedence(t *testing.T) { // lint:allow_complexity
	// **Validates: per-profile override takes precedence over global**
	logger = slog.New(log.New(io.Discard))

	oldConfig := asmConfig

	t.Cleanup(func() { asmConfig = oldConfig })

	rapid.Check(t, func(t *rapid.T) {
		accounts := genListAccounts(1, 3).Draw(t, "accounts")
		profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,9}`).Draw(t, "profileName")
		ssoRegion := rapid.SampledFrom([]string{
			"us-east-1", "us-west-2", "eu-west-1",
		}).Draw(t, "ssoRegion")

		globalRegion := rapid.SampledFrom([]string{
			"eu-central-1", "ap-southeast-1",
		}).Draw(t, "globalRegion")
		globalOutput := rapid.SampledFrom([]string{
			"text", "table",
		}).Draw(t, "globalOutput")

		asmConfig = viper.New()
		asmConfig.Set(profileName+".settings.global.region", globalRegion)
		asmConfig.Set(profileName+".settings.global.output", globalOutput)

		// Pick the first role to apply a per-profile override to.
		targetProfile := accounts.Accounts[0].Roles[0].Profile

		perProfileRegion := rapid.SampledFrom([]string{
			"sa-east-1", "ap-northeast-1",
		}).Draw(t, "perProfileRegion")
		perProfileOutput := rapid.SampledFrom([]string{
			"yaml", "yaml-stream",
		}).Draw(t, "perProfileOutput")

		asmConfig.Set(profileName+".settings."+targetProfile+".region", perProfileRegion)
		asmConfig.Set(profileName+".settings."+targetProfile+".output", perProfileOutput)

		// Build SSO session section.
		ssoProfile := "sso-session " + profileName
		ssoSection := configFile.NewSection(ssoProfile)

		regionVal, err := configFile.NewStringValue(ssoRegion)
		if err != nil {
			t.Fatalf("new string value for sso_region: %v", err)
		}

		if updateErr := ssoSection.UpdateValue("sso_region", regionVal); updateErr != nil {
			t.Fatalf("update sso_region: %v", updateErr)
		}

		sections := configFile.NewSections()

		sections = sections.SetSection(ssoProfile, ssoSection)

		nextSections, _, err := buildUpdatedManagedSections(sections, ssoProfile, profileName, accounts)
		if err != nil {
			t.Fatalf("buildUpdatedManagedSections: %v", err)
		}

		for _, acct := range accounts.Accounts {
			for _, role := range acct.Roles {
				profileHeader := "profile " + role.Profile

				section, ok := nextSections.GetSection(profileHeader)
				if !ok {
					t.Fatalf("expected section [%s] to exist", profileHeader)
				}

				if role.Profile == targetProfile {
					// Per-profile override should win.
					if got := section.String("region"); got != perProfileRegion {
						t.Fatalf(
							"section [%s]: expected per-profile region=%q, got %q",
							profileHeader,
							perProfileRegion,
							got,
						)
					}

					if got := section.String("output"); got != perProfileOutput {
						t.Fatalf(
							"section [%s]: expected per-profile output=%q, got %q",
							profileHeader,
							perProfileOutput,
							got,
						)
					}
				} else {
					// Global should apply.
					if got := section.String("region"); got != globalRegion {
						t.Fatalf("section [%s]: expected global region=%q, got %q", profileHeader, globalRegion, got)
					}

					if got := section.String("output"); got != globalOutput {
						t.Fatalf("section [%s]: expected global output=%q, got %q", profileHeader, globalOutput, got)
					}
				}
			}
		}
	})
}

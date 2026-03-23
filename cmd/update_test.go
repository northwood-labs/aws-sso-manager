package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	configFile "github.com/northwood-labs/aws-config-parser/ini"
)

func mustSetStringValue(t *testing.T, section configFile.Section, key, value string) configFile.Section {
	t.Helper()

	v, err := configFile.NewStringValue(value)
	if err != nil {
		t.Fatalf("new string value for %q: %v", key, err)
	}

	if err := section.UpdateValue(key, v); err != nil {
		t.Fatalf("update value for %q: %v", key, err)
	}

	return section
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
	ssoSection = mustSetStringValue(t, ssoSection, "sso_region", "us-east-1")

	staleSection := configFile.NewSection("profile stale-role")
	staleSection = mustSetStringValue(t, staleSection, "sso_session", "myprofile")
	staleSection = mustSetStringValue(t, staleSection, "sso_account_id", "999999999999")
	staleSection = mustSetStringValue(t, staleSection, "sso_role_name", "OldRole")
	staleSection = mustSetStringValue(t, staleSection, "region", "us-east-1")
	staleSection = mustSetStringValue(t, staleSection, "output", "json")

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
	ssoSection = mustSetStringValue(t, ssoSection, "sso_region", "us-west-2")

	existingSection := configFile.NewSection("profile dev-admin")
	existingSection = mustSetStringValue(t, existingSection, "sso_session", "wrong")
	existingSection = mustSetStringValue(t, existingSection, "sso_account_id", "000000000000")
	existingSection = mustSetStringValue(t, existingSection, "sso_role_name", "OldRole")
	existingSection = mustSetStringValue(t, existingSection, "region", "us-east-1")
	existingSection = mustSetStringValue(t, existingSection, "output", "text")

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
	logger = log.New(io.Discard)

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

	sections, err := loadAWSConfig(awsConfigFilePath)
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
	if err := os.WriteFile(
		tmpManaged,
		[]byte(strings.TrimSpace(generateAWSConfig(nextSections))+"\n"),
		0o0644,
	); err != nil {
		t.Fatalf("write managed tmp: %v", err)
	}

	backupName, err := setManagedSection(tmpManaged, "nwl2")
	if err != nil {
		t.Fatalf("setManagedSection: %v", err)
	}

	content, err := os.ReadFile(backupName)
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

	content2, err := os.ReadFile(backupName2)
	if err != nil {
		t.Fatalf("read backup second pass: %v", err)
	}

	got2 := string(content2)
	if got != got2 {
		t.Fatalf("expected deterministic output across repeated rewrites\nfirst:\n%s\nsecond:\n%s", got, got2)
	}
}

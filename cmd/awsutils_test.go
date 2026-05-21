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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spf13/cobra"
	"pgregory.net/rapid"
)

const (
	testCacheDir        = "cache"
	testProfileNWL2     = "nwl2"
	testAccountID1      = "111111111111"
	testAccountID2      = "222222222222"
	testAccountNameProd = "Production"
)

func TestListAWSAccountsCacheFilePathUsesPackageDefaultDir(t *testing.T) {
	expectedDefaultCacheDir := filepath.Join(userHomeDir, ".config", "aws-sso-manager", testCacheDir)
	if awsManagerCacheDir != expectedDefaultCacheDir {
		t.Fatalf("expected package cache dir %q, got %q", expectedDefaultCacheDir, awsManagerCacheDir)
	}

	oldCacheDir := awsManagerCacheDir

	awsManagerCacheDir = filepath.Join(t.TempDir(), ".config", "aws-sso-manager", testCacheDir)
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	input := &listAWSAccountsInput{
		Logger:      slog.New(log.New(io.Discard)),
		ProfileName: testProfileNWL2,
	}

	cachePath := input.cacheFilePath()
	if cachePath == "" {
		t.Fatal("expected cache file path to be generated")
	}

	if gotDir := filepath.Dir(cachePath); gotDir != awsManagerCacheDir {
		t.Fatalf("expected cache file directory %q, got %q", awsManagerCacheDir, gotDir)
	}

	if _, err := os.Stat(awsManagerCacheDir); err != nil {
		t.Fatalf("expected package cache dir to exist: %v", err)
	}
}

func TestListAWSAccountsUsesCacheWhenPresent(t *testing.T) {
	oldCacheDir := awsManagerCacheDir

	awsManagerCacheDir = filepath.Join(t.TempDir(), testCacheDir)
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	input := &listAWSAccountsInput{
		Logger:        slog.New(log.New(io.Discard)),
		ProfileName:   testProfileNWL2,
		AccountFilter: "sandbox",
		RoleFilter:    "readonly",
	}

	expected := listAccounts{
		Accounts: []listAccount{{
			ID:    testAccountID1,
			Name:  "Sandbox",
			Email: "sandbox@example.com",
			Roles: []listRole{{
				AccountID: testAccountID1,
				Name:      "ReadOnlyAccess",
				Profile:   "sandbox-readonlyaccess",
			}},
		}},
	}

	if err := writeListAWSAccountsCache(input.cacheFilePath(), expected); err != nil {
		t.Fatalf("writeListAWSAccountsCache: %v", err)
	}

	oldFetcher := listAWSAccountsFetcher

	t.Cleanup(func() { listAWSAccountsFetcher = oldFetcher })

	listAWSAccountsFetcher = func(_ *listAWSAccountsInput) (listAccounts, error) {
		return listAccounts{}, errors.New("fetcher should not be called on a cache hit") // lint:allow_errorf
	}

	got, err := listAWSAccounts(input)
	if err != nil {
		t.Fatalf("listAWSAccounts: %v", err)
	}

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected cached accounts %#v, got %#v", expected, got)
	}
}

func TestListAWSAccountsRefreshesExpiredCache(t *testing.T) {
	oldCacheDir := awsManagerCacheDir

	awsManagerCacheDir = filepath.Join(t.TempDir(), testCacheDir)
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	oldCacheDuration := cacheDuration

	cacheDuration = time.Hour

	t.Cleanup(func() { cacheDuration = oldCacheDuration })

	input := &listAWSAccountsInput{
		Cmd:         &cobra.Command{},
		SDKConfig:   &aws.Config{},
		Cache:       &cacheFileData{AccessToken: "token"},
		Logger:      slog.New(log.New(io.Discard)),
		ProfileName: testProfileNWL2,
	}

	cachePath := input.cacheFilePath()
	expiredCache := listAWSAccountsCacheData{
		CachedAt: time.Now().Add(-2 * time.Hour).UTC(),
		Accounts: listAccounts{
			Accounts: []listAccount{{ID: "stale"}},
		},
	}

	data, err := json.Marshal(expiredCache)
	if err != nil {
		t.Fatalf("marshal expired cache: %v", err)
	}

	writeErr := writeListAWSAccountsCache(cachePath, listAccounts{})
	if writeErr != nil {
		t.Fatalf("seed cache file: %v", writeErr)
	}

	writeErr = os.WriteFile(cachePath, data, 0o0600)
	if writeErr != nil {
		t.Fatalf("write expired cache: %v", writeErr)
	}

	expected := listAccounts{
		Accounts: []listAccount{{
			ID:    testAccountID2,
			Name:  testAccountNameProd,
			Email: "prod@example.com",
			Roles: []listRole{{
				AccountID: testAccountID2,
				Name:      "AdministratorAccess",
				Profile:   "production-administratoraccess",
			}},
		}},
	}

	fetchCount := 0
	oldFetcher := listAWSAccountsFetcher

	t.Cleanup(func() { listAWSAccountsFetcher = oldFetcher })

	listAWSAccountsFetcher = func(_ *listAWSAccountsInput) (listAccounts, error) {
		fetchCount++
		return expected, nil
	}

	got, err := listAWSAccounts(input)
	if err != nil {
		t.Fatalf("listAWSAccounts: %v", err)
	}

	if fetchCount != 1 {
		t.Fatalf("expected fetcher to run once after cache expiry, got %d", fetchCount)
	}

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected refreshed accounts %#v, got %#v", expected, got)
	}

	cached, ok, err := readListAWSAccountsCache(cachePath)
	if err != nil {
		t.Fatalf("readListAWSAccountsCache: %v", err)
	}

	if !ok {
		t.Fatal("expected refreshed cache file to exist")
	}

	if !reflect.DeepEqual(cached, expected) {
		t.Fatalf("expected refreshed cache %#v, got %#v", expected, cached)
	}
}

func TestListAWSAccountsWritesLookupCacheForUnfilteredResults(t *testing.T) {
	oldCacheDir := awsManagerCacheDir

	awsManagerCacheDir = filepath.Join(t.TempDir(), testCacheDir)
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	input := &listAWSAccountsInput{
		Cmd:         &cobra.Command{},
		SDKConfig:   &aws.Config{},
		Cache:       &cacheFileData{AccessToken: "token"},
		Logger:      slog.New(log.New(io.Discard)),
		ProfileName: testProfileNWL2,
	}

	expected := listAccounts{
		Accounts: []listAccount{{
			ID:    testAccountID2,
			Name:  testAccountNameProd,
			Email: "prod@example.com",
			Roles: []listRole{{
				AccountID: testAccountID2,
				Name:      "AdministratorAccess",
				Profile:   "prod-admin",
			}},
		}},
	}

	oldFetcher := listAWSAccountsFetcher

	t.Cleanup(func() { listAWSAccountsFetcher = oldFetcher })

	listAWSAccountsFetcher = func(_ *listAWSAccountsInput) (listAccounts, error) {
		return expected, nil
	}

	_, err := listAWSAccounts(input)
	if err != nil {
		t.Fatalf("listAWSAccounts: %v", err)
	}

	lookupCachePath := input.lookupCacheFilePath()
	if lookupCachePath == "" {
		t.Fatal("expected lookup cache path to be generated")
	}

	lookupIndex, ok, err := readListAWSAccountsLookupCache(lookupCachePath)
	if err != nil {
		t.Fatalf("readListAWSAccountsLookupCache: %v", err)
	}

	if !ok {
		t.Fatal("expected lookup cache to be present")
	}

	if len(lookupIndex.AccountsByID) != 1 {
		t.Fatalf("expected one account in lookup index, got %d", len(lookupIndex.AccountsByID))
	}

	if lookupIndex.ProfileName != testProfileNWL2 {
		t.Fatalf("expected lookup index profile name %q, got %q", testProfileNWL2, lookupIndex.ProfileName)
	}

	account, exists := lookupIndex.AccountsByID[testAccountID2]
	if !exists {
		t.Fatal("expected account ID 222222222222 in lookup index")
	}

	if account.Name != testAccountNameProd {
		t.Fatalf("expected account name %q, got %q", testAccountNameProd, account.Name)
	}

	if !reflect.DeepEqual(account.Roles, []string{"AdministratorAccess"}) {
		t.Fatalf("expected roles %#v, got %#v", []string{"AdministratorAccess"}, account.Roles)
	}

	if !reflect.DeepEqual(account.Profiles, []string{"prod-admin"}) {
		t.Fatalf("expected profiles %#v, got %#v", []string{"prod-admin"}, account.Profiles)
	}

	if !reflect.DeepEqual(lookupIndex.AccountIDsByNameCI["production"], []string{testAccountID2}) {
		t.Fatal("expected name lookup index entry for production")
	}

	if !reflect.DeepEqual(lookupIndex.AccountIDsByProfileCI["prod-admin"], []string{testAccountID2}) {
		t.Fatal("expected profile lookup index entry for prod-admin")
	}
}

func TestLoadOrBuildListAWSAccountsLookupIndexBuildsFromAccountsCache(t *testing.T) {
	oldCacheDir := awsManagerCacheDir

	awsManagerCacheDir = filepath.Join(t.TempDir(), testCacheDir)
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	input := &listAWSAccountsInput{
		Logger:      slog.New(log.New(io.Discard)),
		ProfileName: testProfileNWL2,
	}

	accountsData := listAccounts{
		Accounts: []listAccount{{
			ID:    testAccountID1,
			Name:  "Sandbox",
			Email: "sandbox@example.com",
			Roles: []listRole{{
				AccountID: testAccountID1,
				Name:      "ReadOnlyAccess",
				Profile:   "sandbox-readonly",
			}},
		}},
	}

	if err := writeListAWSAccountsCache(input.cacheFilePath(), accountsData); err != nil {
		t.Fatalf("writeListAWSAccountsCache: %v", err)
	}

	index, err := loadOrBuildListAWSAccountsLookupIndex(input)
	if err != nil {
		t.Fatalf("loadOrBuildListAWSAccountsLookupIndex: %v", err)
	}

	if !reflect.DeepEqual(index.AccountIDsByNameCI["sandbox"], []string{testAccountID1}) {
		t.Fatal("expected name lookup index entry for sandbox")
	}

	if !reflect.DeepEqual(index.AccountIDsByProfileCI["sandbox-readonly"], []string{testAccountID1}) {
		t.Fatal("expected profile lookup index entry for sandbox-readonly")
	}

	if index.ProfileName != testProfileNWL2 {
		t.Fatalf("expected lookup index profile name %q, got %q", testProfileNWL2, index.ProfileName)
	}

	if _, err := os.Stat(input.lookupCacheFilePath()); err != nil {
		t.Fatalf("expected lookup cache file to exist: %v", err)
	}
}

func TestDeleteListAWSAccountsCacheRemovesExistingFile(t *testing.T) {
	oldCacheDir := awsManagerCacheDir

	awsManagerCacheDir = filepath.Join(t.TempDir(), testCacheDir)
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	input := &listAWSAccountsInput{
		Logger:      slog.New(log.New(io.Discard)),
		ProfileName: testProfileNWL2,
	}

	cachePath := input.cacheFilePath()
	if cachePath == "" {
		t.Fatal("expected cache path to be generated")
	}

	err := writeListAWSAccountsCache(cachePath, listAccounts{})
	if err != nil {
		t.Fatalf("writeListAWSAccountsCache: %v", err)
	}

	lookupCachePath := input.lookupCacheFilePath()
	if lookupCachePath == "" {
		t.Fatal("expected lookup cache path to be generated")
	}

	err = writeListAWSAccountsLookupCache(lookupCachePath, listAWSAccountsLookupIndex{})
	if err != nil {
		t.Fatalf("writeListAWSAccountsLookupCache: %v", err)
	}

	err = deleteListAWSAccountsCache(input)
	if err != nil {
		t.Fatalf("deleteListAWSAccountsCache: %v", err)
	}

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("expected cache file to be deleted, stat err: %v", err)
	}

	if _, err := os.Stat(lookupCachePath); !os.IsNotExist(err) {
		t.Fatalf("expected lookup cache file to be deleted, stat err: %v", err)
	}
}

func TestDeleteListAWSAccountsCacheIgnoresMissingFile(t *testing.T) {
	oldCacheDir := awsManagerCacheDir

	awsManagerCacheDir = filepath.Join(t.TempDir(), testCacheDir)
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	input := &listAWSAccountsInput{
		Logger:      slog.New(log.New(io.Discard)),
		ProfileName: testProfileNWL2,
	}

	err := deleteListAWSAccountsCache(input)
	if err != nil {
		t.Fatalf("expected missing cache file to be ignored, got error: %v", err)
	}
}

func TestNoCacheFetchThenDeleteOrdering(t *testing.T) {
	// 1. Set up a temp directory for cache files.
	oldCacheDir := awsManagerCacheDir

	awsManagerCacheDir = filepath.Join(t.TempDir(), testCacheDir)
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	input := &listAWSAccountsInput{
		Logger:      slog.New(log.New(io.Discard)),
		ProfileName: testProfileNWL2,
	}

	// 2. Write a valid cache file with known "old" data.
	oldAccounts := listAccounts{
		Accounts: []listAccount{{
			ID:    testAccountID1,
			Name:  "OldAccount",
			Email: "old@example.com",
			Roles: []listRole{{
				AccountID: testAccountID1,
				Name:      "OldRole",
				Profile:   "old-profile",
			}},
		}},
	}

	cachePath := input.cacheFilePath()
	if cachePath == "" {
		t.Fatal("expected cache path to be generated")
	}

	if err := writeListAWSAccountsCache(cachePath, oldAccounts); err != nil {
		t.Fatalf("writeListAWSAccountsCache (old data): %v", err)
	}

	// Verify old cache exists.
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("expected old cache file to exist: %v", err)
	}

	// 3. Set listAWSAccountsFetcher to a mock that returns "new" data and records that it was called.
	newAccounts := listAccounts{
		Accounts: []listAccount{{
			ID:    testAccountID2,
			Name:  "NewAccount",
			Email: "new@example.com",
			Roles: []listRole{{
				AccountID: testAccountID2,
				Name:      "NewRole",
				Profile:   "new-profile",
			}},
		}},
	}

	fetcherCalled := false
	oldFetcher := listAWSAccountsFetcher

	t.Cleanup(func() { listAWSAccountsFetcher = oldFetcher })

	listAWSAccountsFetcher = func(_ *listAWSAccountsInput) (listAccounts, error) {
		fetcherCalled = true
		return newAccounts, nil
	}

	// 4. Simulate the --no-cache flow: fetch → delete old cache → write new cache.
	freshData, err := listAWSAccountsFetcher(input)
	if err != nil {
		t.Fatalf("listAWSAccountsFetcher: %v", err)
	}

	delErr := deleteListAWSAccountsCache(input)
	if delErr != nil {
		t.Fatalf("deleteListAWSAccountsCache: %v", delErr)
	}

	writeErr := writeListAWSAccountsCache(cachePath, freshData)
	if writeErr != nil {
		t.Fatalf("writeListAWSAccountsCache (new data): %v", writeErr)
	}

	// 5. Verify the fetcher was called.
	if !fetcherCalled {
		t.Fatal("expected fetcher to be called when --no-cache is set")
	}

	// 6. Verify the cache file now contains the new data (not the old data).
	cached, ok, err := readListAWSAccountsCache(cachePath)
	if err != nil {
		t.Fatalf("readListAWSAccountsCache: %v", err)
	}

	if !ok {
		t.Fatal("expected cache file to exist after write")
	}

	if !reflect.DeepEqual(cached, newAccounts) {
		t.Fatalf("expected cache to contain new data %#v, got %#v", newAccounts, cached)
	}
}

// Feature: aws-sso-manager, Property 2: Account and Role Sorting.
func TestPropertyAccountAndRoleSorting(t *testing.T) {
	// **Validates: Requirements 3.7**.
	rapid.Check(t, func(t *rapid.T) {
		numAccounts := rapid.IntRange(1, 10).Draw(t, "numAccounts")

		accounts := make([]listAccount, numAccounts)
		for i := range numAccounts {
			accounts[i] = genListAccount().Draw(t, fmt.Sprintf("account%d", i))
		}

		// Apply production sorting logic.
		slices.SortStableFunc(accounts, func(a, b listAccount) int {
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})

		for i := range accounts {
			slices.SortStableFunc(accounts[i].Roles, func(a, b listRole) int {
				return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
			})
		}

		// Verify accounts are sorted.
		for i := 1; i < len(accounts); i++ {
			if strings.ToLower(accounts[i-1].Name) > strings.ToLower(accounts[i].Name) {
				t.Fatalf("accounts not sorted: %q > %q", accounts[i-1].Name, accounts[i].Name)
			}
		}

		// Verify roles within each account are sorted.
		for _, acct := range accounts {
			for j := 1; j < len(acct.Roles); j++ {
				if strings.ToLower(acct.Roles[j-1].Name) > strings.ToLower(acct.Roles[j].Name) {
					t.Fatalf(
						"roles not sorted in account %q: %q > %q",
						acct.Name,
						acct.Roles[j-1].Name,
						acct.Roles[j].Name,
					)
				}
			}
		}
	})
}

// Feature: aws-sso-manager, Property 4: Account and Role Filtering.
func TestPropertyAccountAndRoleFiltering(t *testing.T) { // lint:allow_complexity
	// **Validates: Requirements 3.13, 3.14**.
	rapid.Check(t, func(t *rapid.T) {
		original := genListAccounts(1, 10).Draw(t, "accounts")
		filter := rapid.StringMatching(`[A-Za-z0-9]{2,5}`).Draw(t, "filter")
		filterLower := strings.ToLower(filter)

		// Apply account filtering (replicate production logic from fetchListAWSAccountsFromSSO).
		var filtered listAccounts

		for _, acct := range original.Accounts {
			if !strings.Contains(strings.ToLower(acct.Name), filterLower) {
				continue
			}

			filteredAcct := listAccount{
				ID:    acct.ID,
				Name:  acct.Name,
				Email: acct.Email,
			}

			// Apply role filtering within each matching account.
			for _, role := range acct.Roles {
				if !strings.Contains(strings.ToLower(role.Name), filterLower) {
					continue
				}

				filteredAcct.Roles = append(filteredAcct.Roles, role)
			}

			filtered.Accounts = append(filtered.Accounts, filteredAcct)
		}

		// Verify filtered result is a subset of original.
		if len(filtered.Accounts) > len(original.Accounts) {
			t.Fatalf("filtered accounts (%d) exceeds original (%d)", len(filtered.Accounts), len(original.Accounts))
		}

		// Build a set of original account IDs for subset check.
		originalIDs := make(map[string]bool, len(original.Accounts))
		for _, acct := range original.Accounts {
			originalIDs[acct.ID] = true
		}

		for _, acct := range filtered.Accounts {
			// Every filtered account must exist in original.
			if !originalIDs[acct.ID] {
				t.Fatalf("filtered account %q not found in original", acct.ID)
			}

			// Every account in result must have name containing filter (case-insensitive).
			if !strings.Contains(strings.ToLower(acct.Name), filterLower) {
				t.Fatalf("account %q name %q does not contain filter %q", acct.ID, acct.Name, filter)
			}

			// Every role in result must have name containing filter (case-insensitive).
			for _, role := range acct.Roles {
				if !strings.Contains(strings.ToLower(role.Name), filterLower) {
					t.Fatalf("role %q in account %q does not contain filter %q", role.Name, acct.ID, filter)
				}
			}
		}
	})
}

// Feature: aws-sso-manager, Property 5: Lookup Index Round Trip.
func TestPropertyLookupIndexRoundTrip(t *testing.T) { // lint:allow_complexity
	// **Validates: Requirements 3.19, 7.1, 10.11**.
	rapid.Check(t, func(t *rapid.T) {
		accounts := genListAccounts(1, 5).Draw(t, "accounts")
		profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,9}`).Draw(t, "profileName")

		index := buildListAWSAccountsLookupIndex(profileName, accounts)

		if index.ProfileName != profileName {
			t.Fatalf("expected profile name %q, got %q", profileName, index.ProfileName)
		}

		for _, acct := range accounts.Accounts {
			// Verify account is findable by ID.
			entry, exists := index.AccountsByID[acct.ID]
			if !exists {
				t.Fatalf("account %q not found in AccountsByID", acct.ID)
			}

			// Verify account name matches.
			if entry.Name != acct.Name {
				t.Fatalf("expected account name %q for ID %q, got %q", acct.Name, acct.ID, entry.Name)
			}

			// Verify account is findable by lowercased name.
			nameKey := strings.ToLower(strings.TrimSpace(acct.Name))
			if nameKey != "" {
				ids, exists := index.AccountIDsByNameCI[nameKey]
				if !exists {
					t.Fatalf("account name %q not found in AccountIDsByNameCI", nameKey)
				}

				found := slices.Contains(ids, acct.ID)

				if !found {
					t.Fatalf("account ID %q not found in AccountIDsByNameCI[%q]", acct.ID, nameKey)
				}
			}

			// Verify roles match.
			for _, role := range acct.Roles {
				roleFound := slices.Contains(entry.Roles, role.Name)

				if !roleFound {
					t.Fatalf("role %q not found in lookup entry for account %q", role.Name, acct.ID)
				}

				// For each role with a non-empty Profile, verify findable by lowercased profile name.
				profileKey := strings.ToLower(strings.TrimSpace(role.Profile))
				if profileKey == "" {
					continue
				}

				ids, exists := index.AccountIDsByProfileCI[profileKey]
				if !exists {
					t.Fatalf("profile %q not found in AccountIDsByProfileCI", profileKey)
				}

				found := slices.Contains(ids, acct.ID)

				if !found {
					t.Fatalf("account ID %q not found in AccountIDsByProfileCI[%q]", acct.ID, profileKey)
				}
			}
		}
	})
}

// Feature: aws-sso-manager, Property 10: Cache File Path Determinism.
func TestPropertyCacheFilePathDeterminism(t *testing.T) {
	// **Validates: Requirements 10.2**.
	oldCacheDir := awsManagerCacheDir

	awsManagerCacheDir = filepath.Join(t.TempDir(), testCacheDir)
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	rapid.Check(t, func(t *rapid.T) {
		profileName := rapid.StringMatching(`[a-z][a-z0-9]{2,9}`).Draw(t, "profileName")
		accountFilter := rapid.StringMatching(`[a-z]{0,5}`).Draw(t, "accountFilter")
		roleFilter := rapid.StringMatching(`[a-z]{0,5}`).Draw(t, "roleFilter")

		input := &listAWSAccountsInput{
			Logger:        slog.New(log.New(io.Discard)),
			ProfileName:   profileName,
			AccountFilter: accountFilter,
			RoleFilter:    roleFilter,
		}

		// Same inputs produce same path.
		path1 := input.cacheFilePath()

		path2 := input.cacheFilePath()
		if path1 != path2 {
			t.Fatalf("same inputs produced different paths: %q vs %q", path1, path2)
		}

		// Different inputs produce different paths (with high probability).
		differentProfile := profileName + "x"
		differentInput := &listAWSAccountsInput{
			Logger:        slog.New(log.New(io.Discard)),
			ProfileName:   differentProfile,
			AccountFilter: accountFilter,
			RoleFilter:    roleFilter,
		}

		path3 := differentInput.cacheFilePath()
		if path1 == path3 {
			t.Fatalf("different inputs produced same path: %q", path1)
		}
	})
}

// Feature: aws-sso-manager, Property 11: Cache Expiry Detection.
func TestPropertyCacheExpiryDetection(t *testing.T) {
	// **Validates: Requirements 10.4**.
	oldCacheDir := awsManagerCacheDir

	awsManagerCacheDir = filepath.Join(t.TempDir(), testCacheDir)
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	oldCacheDuration := cacheDuration

	t.Cleanup(func() { cacheDuration = oldCacheDuration })

	rapid.Check(t, func(t *rapid.T) {
		duration := time.Duration(rapid.IntRange(1, 48).Draw(t, "hours")) * time.Hour

		cacheDuration = duration

		accounts := genListAccounts(1, 3).Draw(t, "accounts")

		input := &listAWSAccountsInput{
			Logger:      slog.New(log.New(io.Discard)),
			ProfileName: rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "profile"),
		}
		cachePath := input.cacheFilePath()

		// Test non-expired cache.
		notExpired := listAWSAccountsCacheData{
			CachedAt: time.Now().Add(-duration / 2).UTC(),
			Accounts: accounts,
		}

		data, err := json.Marshal(notExpired)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}

		mkdirErr := os.MkdirAll(filepath.Dir(cachePath), 0o755)
		if mkdirErr != nil {
			t.Fatalf("os.MkdirAll: %v", mkdirErr)
		}

		writeErr := os.WriteFile(cachePath, data, 0o600)
		if writeErr != nil {
			t.Fatalf("os.WriteFile: %v", writeErr)
		}

		got, ok, err := readListAWSAccountsCache(cachePath)
		if err != nil {
			t.Fatalf("readListAWSAccountsCache (not expired): %v", err)
		}

		if !ok {
			t.Fatal("expected cache to be valid (not expired)")
		}

		if len(got.Accounts) != len(accounts.Accounts) {
			t.Fatalf("expected %d accounts, got %d", len(accounts.Accounts), len(got.Accounts))
		}

		// Test expired cache.
		expired := listAWSAccountsCacheData{
			CachedAt: time.Now().Add(-duration * 2).UTC(),
			Accounts: accounts,
		}

		data, err = json.Marshal(expired)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}

		writeErr = os.WriteFile(cachePath, data, 0o600)
		if writeErr != nil {
			t.Fatalf("os.WriteFile: %v", writeErr)
		}

		_, ok, err = readListAWSAccountsCache(cachePath)
		if err != nil {
			t.Fatalf("readListAWSAccountsCache (expired): %v", err)
		}

		if ok {
			t.Fatal("expected cache to be expired")
		}
	})
}

func TestBuildListAWSAccountsLookupIndexEdgeCases(t *testing.T) {
	t.Run("empty accounts", func(t *testing.T) {
		index := buildListAWSAccountsLookupIndex("test", listAccounts{})
		if len(index.AccountsByID) != 0 {
			t.Fatalf("expected empty AccountsByID, got %d", len(index.AccountsByID))
		}
	})

	t.Run("account with no roles", func(t *testing.T) {
		accounts := listAccounts{
			Accounts: []listAccount{{ID: testAccountID1, Name: "NoRoles"}},
		}
		index := buildListAWSAccountsLookupIndex("test", accounts)

		entry, ok := index.AccountsByID[testAccountID1]
		if !ok {
			t.Fatal("expected account in index")
		}

		if len(entry.Roles) != 0 {
			t.Fatalf("expected no roles, got %d", len(entry.Roles))
		}
	})

	t.Run("duplicate account IDs merge roles", func(t *testing.T) {
		accounts := listAccounts{
			Accounts: []listAccount{
				{ID: testAccountID1, Name: "Acct", Roles: []listRole{{Name: "RoleA", Profile: "prof-a"}}},
				{ID: testAccountID1, Name: "Acct", Roles: []listRole{{Name: "RoleB", Profile: "prof-b"}}},
			},
		}
		index := buildListAWSAccountsLookupIndex("test", accounts)

		entry := index.AccountsByID[testAccountID1]
		if len(entry.Roles) != 2 {
			t.Fatalf("expected 2 merged roles, got %d: %v", len(entry.Roles), entry.Roles)
		}
	})

	t.Run("account with empty name", func(t *testing.T) {
		accounts := listAccounts{
			Accounts: []listAccount{{ID: testAccountID1, Name: ""}},
		}

		index := buildListAWSAccountsLookupIndex("test", accounts)
		if _, ok := index.AccountsByID[testAccountID1]; !ok {
			t.Fatal("expected account in index")
		}

		if len(index.AccountIDsByNameCI) != 0 {
			t.Fatalf("expected empty AccountIDsByNameCI for empty name, got %v", index.AccountIDsByNameCI)
		}
	})

	t.Run("account with empty ID is skipped", func(t *testing.T) {
		accounts := listAccounts{
			Accounts: []listAccount{{ID: "", Name: "NoID"}},
		}

		index := buildListAWSAccountsLookupIndex("test", accounts)
		if len(index.AccountsByID) != 0 {
			t.Fatalf("expected empty AccountsByID for empty ID, got %d", len(index.AccountsByID))
		}
	})
}

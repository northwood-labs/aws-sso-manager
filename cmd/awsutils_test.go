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
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

func TestListAWSAccountsCacheFilePathUsesPackageDefaultDir(t *testing.T) {
	expectedDefaultCacheDir := filepath.Join(userHomeDir, ".config", "aws-sso-manager", "cache")
	if awsManagerCacheDir != expectedDefaultCacheDir {
		t.Fatalf("expected package cache dir %q, got %q", expectedDefaultCacheDir, awsManagerCacheDir)
	}

	oldCacheDir := awsManagerCacheDir
	awsManagerCacheDir = filepath.Join(t.TempDir(), ".config", "aws-sso-manager", "cache")
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	input := listAWSAccountsInput{
		Logger:      log.New(io.Discard),
		ProfileName: "nwl2",
	}

	cachePath := input.cacheFilePath()
	if cachePath == "" {
		t.Fatalf("expected cache file path to be generated")
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
	awsManagerCacheDir = filepath.Join(t.TempDir(), "cache")
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	input := listAWSAccountsInput{
		Logger:        log.New(io.Discard),
		ProfileName:   "nwl2",
		AccountFilter: "sandbox",
		RoleFilter:    "readonly",
	}

	expected := listAccounts{
		Accounts: []listAccount{{
			ID:    "111111111111",
			Name:  "Sandbox",
			Email: "sandbox@example.com",
			Roles: []listRole{{
				AccountID: "111111111111",
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
	listAWSAccountsFetcher = func(input listAWSAccountsInput) (listAccounts, error) {
		return listAccounts{}, errors.New("fetcher should not be called on a cache hit")
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
	awsManagerCacheDir = filepath.Join(t.TempDir(), "cache")
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	oldCacheDuration := cacheDuration
	cacheDuration = time.Hour
	t.Cleanup(func() { cacheDuration = oldCacheDuration })

	input := listAWSAccountsInput{
		Cmd:         &cobra.Command{},
		SDKConfig:   &aws.Config{},
		Cache:       &cacheFileData{AccessToken: "token"},
		Logger:      log.New(io.Discard),
		ProfileName: "nwl2",
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

	if err := writeListAWSAccountsCache(cachePath, listAccounts{}); err != nil {
		t.Fatalf("seed cache file: %v", err)
	}

	if err := os.WriteFile(cachePath, data, 0o0600); err != nil {
		t.Fatalf("write expired cache: %v", err)
	}

	expected := listAccounts{
		Accounts: []listAccount{{
			ID:    "222222222222",
			Name:  "Production",
			Email: "prod@example.com",
			Roles: []listRole{{
				AccountID: "222222222222",
				Name:      "AdministratorAccess",
				Profile:   "production-administratoraccess",
			}},
		}},
	}

	fetchCount := 0
	oldFetcher := listAWSAccountsFetcher
	t.Cleanup(func() { listAWSAccountsFetcher = oldFetcher })
	listAWSAccountsFetcher = func(input listAWSAccountsInput) (listAccounts, error) {
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
		t.Fatalf("expected refreshed cache file to exist")
	}

	if !reflect.DeepEqual(cached, expected) {
		t.Fatalf("expected refreshed cache %#v, got %#v", expected, cached)
	}
}

func TestListAWSAccountsWritesLookupCacheForUnfilteredResults(t *testing.T) {
	oldCacheDir := awsManagerCacheDir
	awsManagerCacheDir = filepath.Join(t.TempDir(), "cache")
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	input := listAWSAccountsInput{
		Cmd:         &cobra.Command{},
		SDKConfig:   &aws.Config{},
		Cache:       &cacheFileData{AccessToken: "token"},
		Logger:      log.New(io.Discard),
		ProfileName: "nwl2",
	}

	expected := listAccounts{
		Accounts: []listAccount{{
			ID:    "222222222222",
			Name:  "Production",
			Email: "prod@example.com",
			Roles: []listRole{{
				AccountID: "222222222222",
				Name:      "AdministratorAccess",
				Profile:   "prod-admin",
			}},
		}},
	}

	oldFetcher := listAWSAccountsFetcher
	t.Cleanup(func() { listAWSAccountsFetcher = oldFetcher })
	listAWSAccountsFetcher = func(input listAWSAccountsInput) (listAccounts, error) {
		return expected, nil
	}

	_, err := listAWSAccounts(input)
	if err != nil {
		t.Fatalf("listAWSAccounts: %v", err)
	}

	lookupCachePath := input.lookupCacheFilePath()
	if lookupCachePath == "" {
		t.Fatalf("expected lookup cache path to be generated")
	}

	lookupIndex, ok, err := readListAWSAccountsLookupCache(lookupCachePath)
	if err != nil {
		t.Fatalf("readListAWSAccountsLookupCache: %v", err)
	}
	if !ok {
		t.Fatalf("expected lookup cache to be present")
	}

	if len(lookupIndex.AccountsByID) != 1 {
		t.Fatalf("expected one account in lookup index, got %d", len(lookupIndex.AccountsByID))
	}

	if lookupIndex.ProfileName != "nwl2" {
		t.Fatalf("expected lookup index profile name %q, got %q", "nwl2", lookupIndex.ProfileName)
	}

	account, exists := lookupIndex.AccountsByID["222222222222"]
	if !exists {
		t.Fatalf("expected account ID 222222222222 in lookup index")
	}

	if account.Name != "Production" {
		t.Fatalf("expected account name %q, got %q", "Production", account.Name)
	}

	if !reflect.DeepEqual(account.Roles, []string{"AdministratorAccess"}) {
		t.Fatalf("expected roles %#v, got %#v", []string{"AdministratorAccess"}, account.Roles)
	}

	if !reflect.DeepEqual(account.Profiles, []string{"prod-admin"}) {
		t.Fatalf("expected profiles %#v, got %#v", []string{"prod-admin"}, account.Profiles)
	}

	if !reflect.DeepEqual(lookupIndex.AccountIDsByNameCI["production"], []string{"222222222222"}) {
		t.Fatalf("expected name lookup index entry for production")
	}

	if !reflect.DeepEqual(lookupIndex.AccountIDsByProfileCI["prod-admin"], []string{"222222222222"}) {
		t.Fatalf("expected profile lookup index entry for prod-admin")
	}
}

func TestLoadOrBuildListAWSAccountsLookupIndexBuildsFromAccountsCache(t *testing.T) {
	oldCacheDir := awsManagerCacheDir
	awsManagerCacheDir = filepath.Join(t.TempDir(), "cache")
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	input := listAWSAccountsInput{
		Logger:      log.New(io.Discard),
		ProfileName: "nwl2",
	}

	accountsData := listAccounts{
		Accounts: []listAccount{{
			ID:    "111111111111",
			Name:  "Sandbox",
			Email: "sandbox@example.com",
			Roles: []listRole{{
				AccountID: "111111111111",
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

	if !reflect.DeepEqual(index.AccountIDsByNameCI["sandbox"], []string{"111111111111"}) {
		t.Fatalf("expected name lookup index entry for sandbox")
	}

	if !reflect.DeepEqual(index.AccountIDsByProfileCI["sandbox-readonly"], []string{"111111111111"}) {
		t.Fatalf("expected profile lookup index entry for sandbox-readonly")
	}

	if index.ProfileName != "nwl2" {
		t.Fatalf("expected lookup index profile name %q, got %q", "nwl2", index.ProfileName)
	}

	if _, err := os.Stat(input.lookupCacheFilePath()); err != nil {
		t.Fatalf("expected lookup cache file to exist: %v", err)
	}
}

func TestDeleteListAWSAccountsCacheRemovesExistingFile(t *testing.T) {
	oldCacheDir := awsManagerCacheDir
	awsManagerCacheDir = filepath.Join(t.TempDir(), "cache")
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	input := listAWSAccountsInput{
		Logger:      log.New(io.Discard),
		ProfileName: "nwl2",
	}

	cachePath := input.cacheFilePath()
	if cachePath == "" {
		t.Fatalf("expected cache path to be generated")
	}

	if err := writeListAWSAccountsCache(cachePath, listAccounts{}); err != nil {
		t.Fatalf("writeListAWSAccountsCache: %v", err)
	}

	lookupCachePath := input.lookupCacheFilePath()
	if lookupCachePath == "" {
		t.Fatalf("expected lookup cache path to be generated")
	}

	if err := writeListAWSAccountsLookupCache(lookupCachePath, listAWSAccountsLookupIndex{}); err != nil {
		t.Fatalf("writeListAWSAccountsLookupCache: %v", err)
	}

	if err := deleteListAWSAccountsCache(input); err != nil {
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
	awsManagerCacheDir = filepath.Join(t.TempDir(), "cache")
	t.Cleanup(func() { awsManagerCacheDir = oldCacheDir })

	input := listAWSAccountsInput{
		Logger:      log.New(io.Discard),
		ProfileName: "nwl2",
	}

	if err := deleteListAWSAccountsCache(input); err != nil {
		t.Fatalf("expected missing cache file to be ignored, got error: %v", err)
	}
}

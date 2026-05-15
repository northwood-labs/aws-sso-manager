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

import "errors"

var (
	// ErrSSOTokenExpired indicates the cached SSO access token is past its expiry time.
	ErrSSOTokenExpired = errors.New("the SSO access token has expired")

	// ErrNoSSOProfiles indicates no sso-session sections were found in the AWS config file.
	ErrNoSSOProfiles = errors.New("no SSO profiles found in AWS config; run `aws-sso-manager init` to create one")

	// ErrCacheDurationEmpty indicates the cache duration flag was empty.
	ErrCacheDurationEmpty = errors.New("cache duration cannot be empty")

	// ErrCacheDurationInvalid indicates the cache duration parsed to zero or negative.
	ErrCacheDurationInvalid = errors.New("cache duration must be greater than zero")

	// ErrCachePathLookup indicates the lookup cache file path could not be determined.
	ErrCachePathLookup = errors.New("could not determine lookup cache file path")

	// ErrCachePathAccounts indicates the accounts cache file path could not be determined.
	ErrCachePathAccounts = errors.New("could not determine accounts cache file path")

	// ErrCachePathGeneral indicates a cache file path could not be determined.
	ErrCachePathGeneral = errors.New("could not determine the cache file path")

	// ErrCacheMissing indicates neither the lookup cache nor the accounts cache exists.
	ErrCacheMissing = errors.New("lookup cache is missing and no accounts cache exists")

	// ErrConfigKeyEmpty indicates a config key argument was empty.
	ErrConfigKeyEmpty = errors.New("config key cannot be empty")

	// ErrConfigKeyNotSet indicates the requested config key has no value.
	ErrConfigKeyNotSet = errors.New("key is not set")

	// ErrConfigKeyNotFound indicates the key does not exist in the config file.
	ErrConfigKeyNotFound = errors.New("key not found in config file")

	// ErrConfigKeyInvalid indicates the config key path is structurally invalid.
	ErrConfigKeyInvalid = errors.New("config key is not valid")

	// ErrConfigFileNotExist indicates the config file does not exist at the expected path.
	ErrConfigFileNotExist = errors.New("config file does not exist")

	// ErrConfigSectionExists indicates the config file already has the requested section.
	ErrConfigSectionExists = errors.New("config file already contains section")

	// ErrConfigMarkersExist indicates managed block markers already exist for the profile.
	ErrConfigMarkersExist = errors.New("config file already contains managed block markers")

	// ErrConfigSectionMissing indicates a required section is absent from the config file.
	ErrConfigSectionMissing = errors.New("config file does not have section")

	// ErrConfigSectionCreate indicates a section could not be created or retrieved.
	ErrConfigSectionCreate = errors.New("failed to create or get section")

	// ErrFlagForRequired indicates the --for flag was not provided.
	ErrFlagForRequired = errors.New("flag --for is required")

	// ErrNoProfileConfigured indicates no profile was set via config or flag.
	ErrNoProfileConfigured = errors.New("no profile configured; set profile-name in config or pass --profile")

	// ErrOutputFormatConflict indicates multiple output format flags were set.
	ErrOutputFormatConflict = errors.New("choose only one output format flag: --json, --csv, or --markdown")

	// ErrValueEmpty indicates a required input value was empty.
	ErrValueEmpty = errors.New("value cannot be empty")

	// ErrRoleSubstringEmpty indicates the role search substring was empty.
	ErrRoleSubstringEmpty = errors.New("role substring cannot be empty")

	// ErrAccountIDInvalid indicates the account ID is not a valid 12-digit string.
	ErrAccountIDInvalid = errors.New("--for must be a 12-digit AWS account ID")

	// ErrAccountIDNotFound indicates the account ID was not found in the lookup cache.
	ErrAccountIDNotFound = errors.New("account ID was not found in lookup cache")

	// ErrAccountIdentEmpty indicates the account identifier argument was empty.
	ErrAccountIdentEmpty = errors.New("account identifier cannot be empty")

	// ErrAccountIdentNotFound indicates no account matched the given identifier.
	ErrAccountIdentNotFound = errors.New("account identifier not found")

	// ErrAccountIdentAmbiguous indicates the identifier matched multiple accounts.
	ErrAccountIdentAmbiguous = errors.New("account identifier is ambiguous")

	// ErrAccountDataMissing indicates account data is absent from the lookup cache.
	ErrAccountDataMissing = errors.New("account data is missing from lookup cache")

	// ErrNoRolesMatched indicates no roles matched the search criteria.
	ErrNoRolesMatched = errors.New("no roles matched")

	// ErrProfileNotExist indicates the named profile is absent from the AWS config file.
	ErrProfileNotExist = errors.New("profile does not exist in the AWS config file")

	// ErrStartURLNotFound indicates the sso_start_url could not be discovered for a profile.
	ErrStartURLNotFound = errors.New("could not discover sso_start_url for profile")

	// ErrStartURLInvalid indicates the SSO start URL is not a valid full URL.
	ErrStartURLInvalid = errors.New("expected a full URL with scheme and host")

	// ErrStartURLHostInvalid indicates the SSO start URL host portion is invalid.
	ErrStartURLHostInvalid = errors.New("expected a host or subdomain")

	// ErrCommandRequired indicates the Cmd field was nil in listAWSAccountsInput.
	ErrCommandRequired = errors.New("command is required")

	// ErrSDKConfigRequired indicates the SDKConfig field was nil in listAWSAccountsInput.
	ErrSDKConfigRequired = errors.New("AWS SDK config is required")

	// ErrSSOCacheRequired indicates the Cache field was nil in listAWSAccountsInput.
	ErrSSOCacheRequired = errors.New("SSO cache data is required")

	// ErrValidationFailed indicates one or more managed profiles have configuration problems.
	ErrValidationFailed = errors.New("one or more managed profiles have configuration problems")

	// ErrManagedMarkerIssue indicates a structural problem with managed block markers.
	ErrManagedMarkerIssue = errors.New("managed marker issue")
)

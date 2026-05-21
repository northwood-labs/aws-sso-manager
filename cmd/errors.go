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

import "errors"

var (
	// errSSOTokenExpired indicates the cached SSO access token is past its
	// expiry time.
	errSSOTokenExpired = errors.New("the SSO access token has expired")

	// errNoSSOProfiles indicates no sso-session sections were found in the AWS
	// config file.
	errNoSSOProfiles = errors.New("no SSO profiles found in AWS config; run `aws-sso-manager init` to create one")

	// errCacheDurationEmpty indicates the cache duration flag was empty.
	errCacheDurationEmpty = errors.New("cache duration cannot be empty")

	// errCacheDurationInvalid indicates the cache duration parsed to zero or
	// negative.
	errCacheDurationInvalid = errors.New("cache duration must be greater than zero")

	// errCachePathLookup indicates the lookup cache file path could not be
	// determined.
	errCachePathLookup = errors.New("could not determine lookup cache file path")

	// errCachePathAccounts indicates the accounts cache file path could not be
	// determined.
	errCachePathAccounts = errors.New("could not determine accounts cache file path")

	// errCachePathGeneral indicates a cache file path could not be determined.
	errCachePathGeneral = errors.New("could not determine the cache file path")

	// errCacheMissing indicates neither the lookup cache nor the accounts cache
	// exists.
	errCacheMissing = errors.New("lookup cache is missing and no accounts cache exists")

	// errConfigKeyEmpty indicates a config key argument was empty.
	errConfigKeyEmpty = errors.New("config key cannot be empty")

	// errConfigKeyNotSet indicates the requested config key has no value.
	errConfigKeyNotSet = errors.New("key is not set")

	// errConfigKeyNotFound indicates the key does not exist in the config file.
	errConfigKeyNotFound = errors.New("key not found in config file")

	// errConfigKeyInvalid indicates the config key path is structurally
	// invalid.
	errConfigKeyInvalid = errors.New("config key is not valid")

	// errConfigFileNotExist indicates the config file does not exist at the
	// expected path.
	errConfigFileNotExist = errors.New("config file does not exist")

	// errConfigSectionExists indicates the config file already has the
	// requested section.
	errConfigSectionExists = errors.New("config file already contains section")

	// errConfigMarkersExist indicates managed block markers already exist for
	// the profile.
	errConfigMarkersExist = errors.New("config file already contains managed block markers")

	// errConfigSectionMissing indicates a required section is absent from the
	// config file.
	errConfigSectionMissing = errors.New("config file does not have section")

	// errConfigSectionCreate indicates a section could not be created or
	// retrieved.
	errConfigSectionCreate = errors.New("failed to create or get section")

	// errFlagForRequired indicates the --for flag was not provided.
	errFlagForRequired = errors.New("flag --for is required")

	// errNoProfileConfigured indicates no profile was set via config or flag.
	errNoProfileConfigured = errors.New("no profile configured; set profile-name in config or pass --profile")

	// errOutputFormatConflict indicates multiple output format flags were set.
	errOutputFormatConflict = errors.New("choose only one output format flag: --json, --csv, or --markdown")

	// errValueEmpty indicates a required input value was empty.
	errValueEmpty = errors.New("value cannot be empty")

	// errRoleSubstringEmpty indicates the role search substring was empty.
	errRoleSubstringEmpty = errors.New("role substring cannot be empty")

	// errAccountIDInvalid indicates the account ID is not a valid 12-digit
	// string.
	errAccountIDInvalid = errors.New("--for must be a 12-digit AWS account ID")

	// errAccountIDNotFound indicates the account ID was not found in the lookup
	// cache.
	errAccountIDNotFound = errors.New("account ID was not found in lookup cache")

	// errAccountIdentEmpty indicates the account identifier argument was empty.
	errAccountIdentEmpty = errors.New("account identifier cannot be empty")

	// errAccountIdentNotFound indicates no account matched the given
	// identifier.
	errAccountIdentNotFound = errors.New("account identifier not found")

	// errAccountIdentAmbiguous indicates the identifier matched multiple
	// accounts.
	errAccountIdentAmbiguous = errors.New("account identifier is ambiguous")

	// errAccountDataMissing indicates account data is absent from the lookup
	// cache.
	errAccountDataMissing = errors.New("account data is missing from lookup cache")

	// errNoRolesMatched indicates no roles matched the search criteria.
	errNoRolesMatched = errors.New("no roles matched")

	// errProfileNotExist indicates the named profile is absent from the AWS
	// config file.
	errProfileNotExist = errors.New("profile does not exist in the AWS config file")

	// errStartURLNotFound indicates the sso_start_url could not be discovered
	// for a profile.
	errStartURLNotFound = errors.New("could not discover sso_start_url for profile")

	// errStartURLInvalid indicates the SSO start URL is not a valid full URL.
	errStartURLInvalid = errors.New("expected a full URL with scheme and host")

	// errStartURLHostInvalid indicates the SSO start URL host portion is
	// invalid.
	errStartURLHostInvalid = errors.New("expected a host or subdomain")

	// errCommandRequired indicates the Cmd field was nil in
	// listAWSAccountsInput.
	errCommandRequired = errors.New("command is required")

	// errSDKConfigRequired indicates the SDKConfig field was nil in
	// listAWSAccountsInput.
	errSDKConfigRequired = errors.New("AWS SDK config is required")

	// errSSOCacheRequired indicates the Cache field was nil in
	// listAWSAccountsInput.
	errSSOCacheRequired = errors.New("SSO cache data is required")

	// errValidationFailed indicates one or more managed profiles have
	// configuration problems.
	errValidationFailed = errors.New("one or more managed profiles have configuration problems")

	// errManagedMarkerIssue indicates a structural problem with managed block
	// markers.
	errManagedMarkerIssue = errors.New("managed marker issue")
)

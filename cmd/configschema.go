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

import "reflect"

// These types model the TOML configuration file at
// ~/.config/aws-sso-manager/config.toml so that a JSON Schema can be generated
// programmatically via reflection. The dynamic SSO profile keys (e.g., "abc",
// "nwl") are handled via JSONSchemaExtend on ConfigFile, which sets
// additionalProperties to reference SSOProfileConfig.

// additionalPropertiesTypes maps struct types that accept dynamic keys to the
// type those keys should validate against. This mirrors the JSONSchemaExtend
// additionalProperties pattern used for JSON Schema generation.
var additionalPropertiesTypes = map[reflect.Type]reflect.Type{
	reflect.TypeFor[SettingsConfig](): reflect.TypeFor[GlobalSettingsConfig](),
}

type (
	// PatternConfig controls how profile name tokens are assembled. Order defines
	// which tokens appear and in what sequence; Delimiter separates them.
	PatternConfig struct {
		Delimiter string   `json:"delimiter,omitempty" jsonschema:"description=Delimiter between tokens in the generated profile name.,default=-"                                               toml:"delimiter,omitempty"` // lint:ignore_length
		Order     []string `json:"order,omitempty"     jsonschema:"description=Ordered list of tokens to include in the generated profile name.,enum=PREFIX,enum=ACCOUNT,enum=ROLE,enum=SUFFIX" toml:"order,omitempty"`     // lint:ignore_length
	}

	// AccountRenameConfig holds the rules for rewriting AWS account names in
	// generated profile names. Supports both regex-based and substring-based
	// matching strategies.
	AccountRenameConfig struct {
		// GlobalRegexReplace map[string]string // lint:allow_commented lint:ignore_length.
		SubstrMatchReplace map[string]string `json:"substr_match_replace,omitempty" jsonschema:"description=If the account name contains the key the entire name is replaced with the value." toml:"substr_match_replace,omitempty"` // lint:ignore_length lint:allow_format
	}

	// RoleRenameConfig holds the rules for rewriting AWS role names in generated
	// profile names. Same matching strategies as AccountRenameConfig.
	RoleRenameConfig struct {
		// GlobalRegexReplace map[string]string // lint:allow_commented lint:ignore_length.
		SubstrMatchReplace map[string]string `json:"substr_match_replace,omitempty" jsonschema:"description=If the role name contains the key the entire name is replaced with the value." toml:"substr_match_replace,omitempty"` // lint:ignore_length lint:allow_format
	}

	// RenameConfig groups all profile-name generation settings for a single SSO
	// profile. It controls prefix/suffix tokens, the assembly pattern, and
	// account/role name rewriting rules.
	RenameConfig struct {
		Accounts AccountRenameConfig `json:"accounts"         jsonschema:"description=Rules for rewriting AWS account names in generated profile names."          toml:"accounts,omitempty"` // lint:ignore_length
		Roles    RoleRenameConfig    `json:"roles"            jsonschema:"description=Rules for rewriting AWS role names in generated profile names."             toml:"roles,omitempty"`    // lint:ignore_length
		Prefix   string              `json:"prefix,omitempty" jsonschema:"description=Standard prefix added to all generated profile names for this SSO profile." toml:"prefix,omitempty"`   // lint:ignore_length
		Suffix   string              `json:"suffix,omitempty" jsonschema:"description=Standard suffix added to all generated profile names for this SSO profile." toml:"suffix,omitempty"`   // lint:ignore_length
		Pattern  PatternConfig       `json:"pattern"          jsonschema:"description=Controls how profile name tokens are ordered and delimited."                toml:"pattern,omitempty"`  // lint:ignore_length
	}

	// GlobalSettingsConfig holds per-profile AWS CLI defaults that are applied to
	// every generated [profile ...] section for this SSO profile.
	GlobalSettingsConfig struct {
		Region               string `json:"region,omitempty"                 jsonschema:"description=Default AWS region for profiles generated under this SSO profile."                                                                                                 toml:"region,omitempty"`                 // lint:ignore_length
		Output               string `json:"output,omitempty"                 jsonschema:"description=Default output format (json text table yaml yaml-stream) for profiles generated under this SSO profile.,enum=json,enum=text,enum=table,enum=yaml,enum=yaml-stream" toml:"output,omitempty"`                 // lint:ignore_length
		DurationSeconds      string `json:"duration_seconds,omitempty"       jsonschema:"description=Session duration in seconds for assumed roles."                                                                                                                    toml:"duration_seconds,omitempty"`       // lint:ignore_length lint:allow_format
		SDKUAAppID           string `json:"sdk_ua_app_id,omitempty"          jsonschema:"description=Application ID appended to the SDK user-agent string."                                                                                                             toml:"sdk_ua_app_id,omitempty"`          // lint:ignore_length lint:allow_format
		UseDualstackEndpoint string `json:"use_dualstack_endpoint,omitempty" jsonschema:"description=Enable dual-stack (IPv4/IPv6) endpoints.,enum=true,enum=false"                                                                                                     toml:"use_dualstack_endpoint,omitempty"` // lint:ignore_length lint:allow_format
		UseFIPSEndpoint      string `json:"use_fips_endpoint,omitempty"      jsonschema:"description=Enable FIPS-compliant endpoints.,enum=true,enum=false"                                                                                                             toml:"use_fips_endpoint,omitempty"`      // lint:ignore_length lint:allow_format
		TCPKeepAlive         string `json:"tcp_keepalive,omitempty"          jsonschema:"description=Enable TCP keep-alive for connections.,enum=true,enum=false"                                                                                                       toml:"tcp_keepalive,omitempty"`          // lint:ignore_length lint:allow_format
	}
)

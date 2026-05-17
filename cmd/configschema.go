// Copyright 2025-2026, Northwood Labs
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
	"reflect"
	"strings"

	"github.com/invopop/jsonschema"
)

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
		// GlobalRegexReplace map[string]string // lint:allow_commented lint:ignore_length
		SubstrMatchReplace map[string]string `json:"substr_match_replace,omitempty" jsonschema:"description=If the account name contains the key the entire name is replaced with the value." toml:"substr_match_replace,omitempty"` // lint:ignore_length lint:allow_format
	}

	// RoleRenameConfig holds the rules for rewriting AWS role names in generated
	// profile names. Same matching strategies as AccountRenameConfig.
	RoleRenameConfig struct {
		// GlobalRegexReplace map[string]string // lint:allow_commented lint:ignore_length
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

	// SettingsConfig groups settings scopes for a single SSO profile. The Global
	// sub-table holds defaults applied to every generated [profile ...] section.
	// Per-profile overrides live under dynamic keys that match generated AWS CLI
	// Any other key under settings is treated as a per-profile override keyed by
	// the generated AWS CLI profile name (e.g., [abc.settings.sandbox-admin]).
	SettingsConfig struct {
		Global GlobalSettingsConfig `json:"global" jsonschema:"description=Global defaults applied to every generated profile under this SSO profile." toml:"global,omitempty"` // lint:ignore_length
	}

	// SSOProfileConfig represents the configuration for a single SSO profile
	// (AWS Organization). The profile key (e.g., "abc", "nwl") is the dynamic
	// map key at the top level of the TOML file.
	SSOProfileConfig struct {
		Settings SettingsConfig `json:"settings" jsonschema:"description=Default AWS CLI settings applied to every generated profile under this SSO profile." toml:"settings,omitempty"` // lint:ignore_length
		Rename   RenameConfig   `json:"rename"   jsonschema:"description=Profile name generation and rewriting rules for this SSO profile."                   toml:"rename"`             // lint:ignore_length
	}

	// ConfigFile is the root schema for the TOML configuration file. The fixed
	// "profile-name" key sets the default SSO profile. All other top-level keys
	// are dynamic SSO profile identifiers mapped to SSOProfileConfig.
	ConfigFile struct {
		ProfileName string `json:"profile-name,omitempty" jsonschema:"description=Default SSO profile name used when no profile is explicitly provided to the CLI."` // lint:ignore_length lint:allow_format
	}
)

// JSONSchemaExtend marks dynamic per-profile override keys under settings as
// valid by referencing GlobalSettingsConfig as additionalProperties. This
// mirrors the pattern used by ConfigFile for dynamic SSO profile keys.
func (SettingsConfig) JSONSchemaExtend(schema *jsonschema.Schema) {
	schema.AdditionalProperties = &jsonschema.Schema{
		Ref: "#/$defs/GlobalSettingsConfig",
	}
}

// JSONSchemaExtend adds additionalProperties referencing SSOProfileConfig so
// that dynamic SSO profile keys (e.g., "abc", "nwl") are validated against
// the correct sub-schema. Standard reflection only captures the fixed
// "profile-name" property; this hook fills in the dynamic part.
func (ConfigFile) JSONSchemaExtend(schema *jsonschema.Schema) {
	schema.AdditionalProperties = &jsonschema.Schema{
		Ref: "#/$defs/SSOProfileConfig",
	}
}

// validateConfigKey checks whether a dot-delimited key is a valid path in the
// config file schema. This prevents `config set` from persisting keys that
// belong to Viper's merged state (flags, env vars) but not the TOML file.
// The validation walks the struct types via reflection so it stays in sync
// with the schema automatically.
func validateConfigKey(key string) error {
	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return ErrConfigKeyEmpty
	}

	// "profile-name" is the only fixed top-level key.
	if parts[0] == "profile-name" {
		if len(parts) == 1 {
			return nil
		}

		return fmt.Errorf("%w: %q is a string, not a table", ErrConfigKeyInvalid, "profile-name")
	}

	// Everything else is <sso-profile>.rename.* — the first segment is the
	// dynamic profile name, so we skip it and validate the rest against
	// SSOProfileConfig.
	if len(parts) < 2 { // lint:allow_raw_number
		return fmt.Errorf("%w: expected <profile>.<path> for %q", ErrConfigKeyInvalid, key)
	}

	if err := walkStructPath(reflect.TypeFor[SSOProfileConfig](), parts[1:], key); err != nil {
		return fmt.Errorf("validating config key path: %w", err)
	}

	return nil
}

// walkStructPath recursively validates that a sequence of dot-split key
// segments corresponds to a valid path through the given struct type. Map
// fields (map[string]string) accept any remaining key as a map entry.
// Struct types registered in additionalPropertiesTypes accept unknown keys
// and validate remaining segments against the registered value type.
func walkStructPath(t reflect.Type, parts []string, fullKey string) error { // lint:allow_complexity
	if len(parts) == 0 {
		return nil
	}

	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return fmt.Errorf("%w: unexpected non-struct type at segment %q in %q", ErrConfigKeyInvalid, parts[0], fullKey)
	}

	segment := parts[0]

	for field := range t.Fields() {
		jsonTag := field.Tag.Get("json")
		jsonName, _, _ := strings.Cut(jsonTag, ",")

		if jsonName != segment {
			continue
		}

		ft := field.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}

		remaining := parts[1:]

		switch ft.Kind() {
		case reflect.Map:
			if len(remaining) <= 1 {
				return nil
			}

			return fmt.Errorf(
				"%w: %q is a map, keys cannot be nested further in %q",
				ErrConfigKeyInvalid,
				segment,
				fullKey,
			)

		case reflect.Struct:
			if len(remaining) == 0 {
				return nil
			}

			if walkErr := walkStructPath(ft, remaining, fullKey); walkErr != nil {
				return fmt.Errorf("struct field %q: %w", segment, walkErr)
			}

			return nil

		case reflect.Slice, reflect.String, reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Float32, reflect.Float64:
			if len(remaining) == 0 {
				return nil
			}

			return fmt.Errorf("%w: %q is a leaf value, not a table in %q", ErrConfigKeyInvalid, segment, fullKey)

		default:
			if len(remaining) == 0 {
				return nil
			}

			return fmt.Errorf("%w: cannot descend into %q in %q", ErrConfigKeyInvalid, segment, fullKey)
		}
	}

	// No struct field matched. If this struct type accepts dynamic keys
	// (additional properties), treat the segment as a dynamic key and
	// validate the remaining path against the registered value type.
	if valueType, ok := additionalPropertiesTypes[t]; ok {
		remaining := parts[1:]
		if len(remaining) == 0 {
			return nil
		}

		if walkErr := walkStructPath(valueType, remaining, fullKey); walkErr != nil {
			return fmt.Errorf("dynamic key %q: %w", segment, walkErr)
		}

		return nil
	}

	return fmt.Errorf("%w: unknown config key %q in %q", ErrConfigKeyInvalid, segment, fullKey)
}

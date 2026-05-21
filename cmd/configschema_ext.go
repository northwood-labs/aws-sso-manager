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
	"reflect"
	"strings"

	"github.com/invopop/jsonschema"
)

type (
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
		return errConfigKeyEmpty
	}

	// "profile-name" is the only fixed top-level key.
	if parts[0] == "profile-name" {
		if len(parts) == 1 {
			return nil
		}

		return fmt.Errorf("%w: %q is a string, not a table", errConfigKeyInvalid, "profile-name")
	}

	// Everything else is <sso-profile>.rename.* — the first segment is the
	// dynamic profile name, so we skip it and validate the rest against
	// SSOProfileConfig.
	if len(parts) < 2 {
		return fmt.Errorf("%w: expected <profile>.<path> for %q", errConfigKeyInvalid, key)
	}

	err := walkStructPath(reflect.TypeFor[SSOProfileConfig](), parts[1:], key)
	if err != nil {
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
		return fmt.Errorf(
			"%w: unexpected non-struct type at segment %q in %q",
			errConfigKeyInvalid,
			parts[0],
			fullKey,
		)
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
				errConfigKeyInvalid,
				segment,
				fullKey,
			)

		case reflect.Struct:
			if len(remaining) == 0 {
				return nil
			}

			walkErr := walkStructPath(ft, remaining, fullKey)
			if walkErr != nil {
				return fmt.Errorf("struct field %q: %w", segment, walkErr)
			}

			return nil

		case reflect.Slice, reflect.String, reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Float32, reflect.Float64:
			if len(remaining) == 0 {
				return nil
			}

			return fmt.Errorf("%w: %q is a leaf value, not a table in %q", errConfigKeyInvalid, segment, fullKey)

		default:
			if len(remaining) == 0 {
				return nil
			}

			return fmt.Errorf("%w: cannot descend into %q in %q", errConfigKeyInvalid, segment, fullKey)
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

		walkErr := walkStructPath(valueType, remaining, fullKey)
		if walkErr != nil {
			return fmt.Errorf("dynamic key %q: %w", segment, walkErr)
		}

		return nil
	}

	return fmt.Errorf("%w: unknown config key %q in %q", errConfigKeyInvalid, segment, fullKey)
}

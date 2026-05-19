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

// configschema generates a JSON Schema from the Go struct model of the TOML
// configuration file. Run it to produce docs/config-schema.json.
package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"

	"github.com/invopop/jsonschema"

	"github.com/northwood-labs/aws-sso-manager/cmd"
)

func main() {
	r := &jsonschema.Reflector{
		Anonymous:      true,
		ExpandedStruct: true,
	}

	schema := r.Reflect(&cmd.ConfigFile{})

	schema.Title = "aws-sso-manager configuration"
	schema.Description = "Schema for ~/.config/aws-sso-manager/config.toml"

	// Reflect SSOProfileConfig with a non-expanded reflector so its full
	// definition tree lands in $defs. The root ConfigFile only references it
	// via additionalProperties (injected by JSONSchemaExtend), so the
	// expanded reflector doesn't discover it automatically.
	sub := (&jsonschema.Reflector{Anonymous: true}).Reflect(&cmd.SSOProfileConfig{})

	if schema.Definitions == nil {
		schema.Definitions = make(jsonschema.Definitions)
	}

	maps.Copy(schema.Definitions, sub.Definitions)

	out, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling schema: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(out))
}

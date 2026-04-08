# Configuration File

The configuration file is mainly intended to help streamline and simplify how profiles are named in the AWS config file, and fix/standardize how accounts are named as compared to how they were named in the AWS Organizations account listings.

It is also designed to help you manage configs across multiple AWS Organizations.

## Lexicon

| Term        | Meaning                                                                                                                                                                                     |
|-------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Account     | The represents the individual AWS account. This can be given a friendly name like `prod`, `nonprod`, `sandbox`, etc.                                                                        |
| Profile     | This is the AWS profile name as listed in the `~/.aws/config` file. It's what the `AWS_PROFILE` environment variable would point to.                                                        |
| Role        | When working with AWS Organizations, the default roles typically include `*AdministratorAccess` or `*ReadOnlyAccess`.                                                                       |
| SSO Profile | This is a short, user-defined identifier for the AWS Organization. For example, _Northwood Labs_ might use `nwl`. _Cloudflare_ might use `cfl`. _Stripe_ might use `strp` or even `stripe`. |

## General documentation

> [!NOTE]
> _Most_ configurations are scoped to a particular SSO Profile. This allows you to set up your naming rules per-Organization. For brevity, this is marked in the names as `%`.

The configuration file lives at `~/.config/aws-sso-manager/config.toml` for macOS and Linux, and `%USERPROFILE%\.config\aws-sso-manager\config.toml` for Windows.

This is the organization of the [TOML](https://toml.io) keys in the config file.

```plain
├── profile-name (string)
└── %.
    ├── settings.
    │   ├── global.
    │   │   ├── region (string)
    │   │   ├── output (string)
    │   │   ├── duration_seconds (string)
    │   │   ├── sdk_ua_app_id (string)
    │   │   ├── use_dualstack_endpoint (string)
    │   │   ├── use_fips_endpoint (string)
    │   │   └── tcp_keepalive (string)
    │   └── <profile-name>.
    │       ├── region (string)
    │       ├── output (string)
    │       ├── duration_seconds (string)
    │       ├── sdk_ua_app_id (string)
    │       ├── use_dualstack_endpoint (string)
    │       ├── use_fips_endpoint (string)
    │       └── tcp_keepalive (string)
    └── rename.
        ├── prefix (string)
        ├── suffix (string)
        ├── accounts.
        │   ├── global_regex_replace (map)
        │   │   └── <key> = <value>
        │   └── substr_match_replace (map)
        │       └── <key> = <value>
        ├── pattern.
        │   ├── delimiter (string)
        │   └── order (enum)
        └── roles.
            ├── global_regex_replace (map)
            │   └── <key> = <value>
            └── substr_match_replace (map)
                └── <key> = <value>
```

### profile-name

This is a `string` and represents the _SSO profile_ name to use as a default when no _SSO profile_ is explicitly provided to the CLI.

If you run `aws-sso-manager auth` and this config value is set to `abc`, then the command will execute `aws-sso-manager auth abc`. However, if you explicitly run `aws-sso-manager auth xyz`, then the command will execute that.

### %.settings.global.region

This is a `string` that overrides the `region` field in every generated `[profile ...]` block for a particular SSO profile. When set to a non-empty value, all profiles generated under that SSO profile will use this region instead of the `sso_region` from the `[sso-session ...]` section.

When this value is empty or absent, the `sso_region` from the corresponding `[sso-session ...]` section is used as the fallback.

### %.settings.global.output

This is a `string` that overrides the `output` field in every generated `[profile ...]` block for a particular SSO profile. Valid values are `json`, `text`, `table`, `yaml`, and `yaml-stream`.

When this value is empty or absent, `json` is used as the default.

### %.settings.global.duration\_seconds

This is a `string` that sets the `duration_seconds` field in every generated `[profile ...]` block for a particular SSO profile. This controls the session duration for assumed roles.

Only written to the profile when explicitly configured.

### %.settings.global.sdk\_ua\_app\_id

This is a `string` that sets the `sdk_ua_app_id` field in every generated `[profile ...]` block. This appends an application ID to the SDK user-agent string.

Only written to the profile when explicitly configured.

### %.settings.global.use\_dualstack\_endpoint

This is a `string` (`true` or `false`) that sets the `use_dualstack_endpoint` field in every generated `[profile ...]` block. Enables dual-stack (IPv4/IPv6) endpoints.

Only written to the profile when explicitly configured.

### %.settings.global.use\_fips\_endpoint

This is a `string` (`true` or `false`) that sets the `use_fips_endpoint` field in every generated `[profile ...]` block. Enables FIPS-compliant endpoints.

Only written to the profile when explicitly configured.

### %.settings.global.tcp\_keepalive

This is a `string` (`true` or `false`) that sets the `tcp_keepalive` field in every generated `[profile ...]` block. Enables TCP keep-alive for connections.

Only written to the profile when explicitly configured.

### %.settings.\<profile-name\>.region

This is a `string` that overrides the `region` field for a single generated `[profile ...]` block, identified by its profile name. This takes precedence over `%.settings.global.region`.

When this value is empty or absent, the global setting (or `sso_region` fallback) is used.

### %.settings.\<profile-name\>.output

This is a `string` that overrides the `output` field for a single generated `[profile ...]` block, identified by its profile name. This takes precedence over `%.settings.global.output`. Valid values are `json`, `text`, `table`, `yaml`, and `yaml-stream`.

When this value is empty or absent, the global setting (or `json` fallback) is used.

The per-profile scope also supports `duration_seconds`, `sdk_ua_app_id`, `use_dualstack_endpoint`, `use_fips_endpoint`, and `tcp_keepalive` with the same override behavior.

### %.rename.prefix

Working alongside `%.rename.pattern.order`, this is a standard prefix to add to all AWS profiles that are generated for a particular AWS Organization. This is most useful when you are juggling accounts for multiple organizations (e.g., `abc`, `xyz`) that might have similarly-named accounts (e.g., `prod`, `nonprod`).

If you do contracting or freelancing where you have access to multiple AWS orgs, this can come in handy.

### %.rename.suffix

Same as `%.rename.prefix`, but is a standard suffix.

### %.rename.accounts.global_regex_replace

This is a `map` (or _dictionary_, or _associative array_, or _hash table_).

Some organizations may have already created several accounts with one naming pattern, then decide to change the naming pattern later down the line. So when you go to create an a list of AWS profiles for yourself, it can get a little gnarly.

So if you have an account named `Company @ ZZZ - Service name - Production`, and you want to use `zzz` as its identifier, you can write a regular expression to grab the account identifier and lowercase it.

If the regex matches the string, then a lowercase version of `$1` will be used for the replacement value. This is useful when you have too many accounts to rename/remap them all manually.

You can add multiple `"key" = "value"` pairs to match different patterns.

### %.rename.accounts.substr_match_replace

This is a `map` (or _dictionary_, or _associative array_, or _hash table_).

If the name of the account contains the substring passed as the `key`, then the entire name is replaced with `value`.

So if an AWS account name is `ABC-ProductionAccount`, and we set `"Production" = "prod"` as a config value, then the AWS profile would represent the Production account with `prod`.

You can add multiple `"key" = "value"` pairs to match different patterns.

### %.rename.pattern.delimiter

This is a `string`, usually a single character, that is used to delimit the bits of data (`%.rename.pattern.order`) in the name.

The default value is a hyphen (`-`). Other common values could be a period (`.`) or an underscore (`_`).

### %.rename.pattern.order

This accepts a set of enum values, in order, in a list. The default value is `["PREFIX", "ACCOUNT", "ROLE", "SUFFIX"]`.

* `PREFIX` is mapped to `%.rename.prefix`.
* `SUFFIX` is mapped to `%.rename.suffix`.

If these are empty, they will be ignored in the pattern. Extra delimiters (`%.rename.pattern.delimiter`) will be trimmed. You might choose to set either of these as a company/organization identifier to distinguish them from other accounts.

### %.rename.roles.global_regex_replace

This is a `map` (or _dictionary_, or _associative array_, or _hash table_).

Some organizations may have already created several accounts with one naming pattern, then decide to change the naming pattern later down the line. So when you go to create an a list of AWS profiles for yourself, it can get a little gnarly.

So if you have an SSO role named `ABC-AdministratorAccess-abcd1234`, and you want to use `admin` as its identifier, you can write a regular expression to grab the role identifier and lowercase it.

If the regex matches the string, then you can use `$1`, `$2`, etc., in the replacement value. This is useful when you have too many roles to rename/remap them all manually.

You can add multiple `"key" = "value"` pairs to match different patterns.

### %.rename.roles.substr_match_replace

This is a `map` (or _dictionary_, or _associative array_, or _hash table_).

If the name of the role contains the substring passed as the `key`, then the entire name is replaced with `value`.

So if a role name is `ABC-AdministratorAccess-abcd1234`, and we set `"AdministratorAccess" = "admin"` as a config value, then the AWS profile would represent the admin role with `admin`.

You can add multiple `"key" = "value"` pairs to match different patterns.

## Sample config

```toml
# This is the default SSO profile to use if nothing else is provided.
profile-name = "abc"

# Per-profile AWS CLI defaults applied to every generated profile.
[abc.settings.global]
region = "us-west-2"
output = "json"
use_fips_endpoint = "true"

# Per-profile override for a specific generated profile.
[abc.settings.prod-admin]
region = "us-east-1"
duration_seconds = "43200"

# Used for generating account profiles under the SSO profile.
#
# Pattern order keyword values: "PREFIX", "ACCOUNT", "ROLE", "SUFFIX"
# If any are missing, it will be removed from the naming.
[abc.rename]
pattern.order     = ["PREFIX", "ACCOUNT", "ROLE", "SUFFIX"]
pattern.delimiter = "-"

prefix = ""
suffix = ""

  [abc.rename.accounts]
  # Perform replacements, in order, for every result. Useful for cleaning up
  # poor naming patterns.
  global_regex_replace = {}

    # If account name matches regex/substring, perform the replacement for the
    # entire name.
    [abc.rename.accounts.substr_match_replace]
    "Production" = "prod"
    "Non-Prod"   = "nonprod"

  [abc.rename.roles]
  # Perform replacements, in order, for every result. Useful for cleaning up
  # poor naming patterns.
  global_regex_replace = {}

    # If role name matches regex/substring, perform the replacement for the
    # entire name.
    [abc.rename.roles.substr_match_replace]
    "AdministratorAccess" = "admin"
    "PowerUserAccess"     = "" # Treat as default.
    "ReadOnlyAccess"      = "ro"
```

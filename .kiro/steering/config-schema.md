---
inclusion: manual
---

# Config Schema Reference

Pull this in with `#config-schema` when working on configuration-related code.

Full documentation: #[[file:docs/config_file.md]]

## Quick Reference

```toml
# Default SSO profile when no argument is provided
profile-name = "abc"

[abc.rename]
pattern.order     = ["PREFIX", "ACCOUNT", "ROLE", "SUFFIX"]
pattern.delimiter = "-"
prefix = ""
suffix = ""

  [abc.rename.accounts.substr_match_replace]
  "Production" = "prod"

  [abc.rename.roles.substr_match_replace]
  "AdministratorAccess" = "admin"
```

## Key Rules

* Config path: `~/.config/aws-sso-manager/config.toml` (created automatically if missing).
* All settings are scoped per SSO profile (the `%` in `%.rename.*`).
* Empty prefix/suffix are omitted from generated names, not rendered as empty tokens.
* `substr_match_replace` is case-insensitive substring matching — if the key is found anywhere in the account/role name, the entire token is replaced with the value.
* Empty replacement value means "omit this token entirely".
* When no pattern is configured, `buildDefaultProfileName` produces `<account>-<role>` with non-alphanumeric chars replaced by hyphens.

## Environment Variable Override

* Prefix: `ASM_`
* Key mapping: hyphens and dots become underscores (e.g., `profile-name` → `ASM_PROFILE_NAME`).
* Precedence: CLI flags > env vars > config file > defaults.

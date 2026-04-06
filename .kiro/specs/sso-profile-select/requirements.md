# Requirements Document

## Introduction

The CLI currently prompts for the SSO profile name using a free-text input (`huh.NewInput()`) when the profile is not supplied as an argument or via configuration. This is error-prone because users must remember and type the exact profile identifier. This feature replaces the free-text input with a select/dropdown list (`huh.NewSelect()`) populated from the `[sso-session ...]` sections in the AWS config file (`~/.aws/config`). The `init` command is excluded because it creates new profiles that do not yet exist in the config file.

## Glossary

* **CLI**: The `aws-sso-manager` command-line application
* **SSO_Profile_Prompt**: The interactive TUI element that collects the SSO profile name from the user when one is not provided via argument or configuration
* **AWS_Config_File**: The AWS shared configuration file, typically located at `~/.aws/config`, resolved via `awsConfigFilePath`
* **SSO_Session_Section**: An INI section in the AWS_Config_File with the header `[sso-session <name>]`
* **Profile_Identifier**: The `<name>` portion of an SSO_Session_Section header
* **Select_Widget**: A `huh.NewSelect()` TUI element that presents a list of options for the user to choose from
* **Input_Widget**: A `huh.NewInput()` TUI element that accepts free-text input

## Requirements

### Requirement 1: Replace free-text input with select list for non-init commands

**User Story:** As a CLI user, I want to select my SSO profile from a list of existing profiles, so that I do not have to remember and type the exact profile name.

#### Acceptance Criteria

1. WHEN the `auth` command requires an SSO profile name and none is provided via argument or configuration, THE SSO_Profile_Prompt SHALL display a Select_Widget populated with Profile_Identifiers parsed from the AWS_Config_File.
2. WHEN the `list` command requires an SSO profile name and none is provided via argument or configuration, THE SSO_Profile_Prompt SHALL display a Select_Widget populated with Profile_Identifiers parsed from the AWS_Config_File.
3. WHEN the `update` command requires an SSO profile name and none is provided via argument or configuration, THE SSO_Profile_Prompt SHALL display a Select_Widget populated with Profile_Identifiers parsed from the AWS_Config_File.

### Requirement 2: Preserve free-text input for the init command

**User Story:** As a CLI user, I want to type a new profile name when running `init`, so that I can create profiles that do not yet exist in the config file.

#### Acceptance Criteria

1. WHEN the `init` command requires an SSO profile name and none is provided via argument or configuration, THE SSO_Profile_Prompt SHALL display an Input_Widget that accepts free-text input.
2. THE `init` command SHALL NOT use a Select_Widget for SSO profile name collection.

### Requirement 3: Populate select options from AWS config file

**User Story:** As a CLI user, I want the profile list to reflect the SSO sessions currently defined in my AWS config, so that I always see an accurate set of choices.

#### Acceptance Criteria

1. THE CLI SHALL read Profile_Identifiers by parsing all SSO_Session_Section headers from the AWS_Config_File.
2. THE CLI SHALL present the Profile_Identifiers in sorted alphabetical order within the Select_Widget.
3. IF the AWS_Config_File contains zero SSO_Session_Sections, THEN THE CLI SHALL return a descriptive error message indicating no SSO profiles are available and suggest running `init`.

### Requirement 4: Console command retains existing select behavior

**User Story:** As a CLI user, I want the `console` command to continue using its existing select-based profile prompt, so that the UX remains consistent.

#### Acceptance Criteria

1. THE `console` command SHALL continue to use a Select_Widget for SSO profile selection when no profile is provided.
2. THE `console` command SHALL populate its Select_Widget with Profile_Identifiers parsed from the AWS_Config_File using the same data source as other non-init commands.

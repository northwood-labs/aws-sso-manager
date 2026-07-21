# AWS SSO Manager

Logging into a user interface to grab credentials that you copy-paste into your shell session is both **cumbersome** and **insecure**.

This tool will help you manage your AWS Organizations and [AWS Identity Center] (née AWS SSO) profiles stored in `~/.aws/config`.

These SSO profiles can be used with `$AWS_PROFILE`, [AWS Vault], or other tools, allowing you to pull credentials _easily_ and keep them secure by passing them exclusively to an ephemeral sub-shell.

It is _complementary_ to the AWS CLI and tools like [AWS Vault].

## Features

* Allows you to login to [AWS Identity Center](https://docs.aws.amazon.com/singlesignon/latest/userguide/what-is.html) (née _SSO_), then update your AWS config file with all of the profiles you have access to.

* Updates to your AWS config file are sandboxed in a way that does not mess with existing credentials or profiles.

* Supports multiple _AWS Identity Center_ profiles.

* Is complementary to tools such as [AWS Vault] in that it sets up the AWS config file that AWS Vault uses to perform its magic.

* Leverages the modern [SSO token provider configuration](https://docs.aws.amazon.com/sdkref/latest/guide/feature-sso-credentials.html#sso-token-config).

* Supports the [`AWS_CONFIG_FILE`](https://docs.aws.amazon.com/sdkref/latest/guide/file-location.html) environment variable for alternate config files.

* Supports custom profile names via configuration file and pattern-matching.

* Supports configuring a default SSO profile to interact with.

* Does not require the [AWS CLI](https://aws.amazon.com/cli/).

* Only need to install a single binary (no dependencies).

* Supports Linux, macOS, and Windows.

### Non-features

* Not meant for solutions that do not leverage _AWS Identity Center_, e.g., `saml2aws`.
* Chooses to let _AWS Vault_ do what it does best, and doesn't try to replicate its functionality.

## Adding your AWS identity center account

> [!NOTE]
> If you have a file called `~/.aws/credentials`, the goal is to **delete** it so that only `~/.aws/config` remains. We will NOT be storing credentials in plain text.

1. Since this is the first time you're adding credentials, your `~/.aws/config` should be blank. If you already have credentials there, that's fine, but if you want this tool to manage them, you should delete them and set them up anew.

https://github.com/user-attachments/assets/10753743-8749-4438-b2ef-44d73bbe0c58

You will be asked for your _SSO Start URL_, and your SSO region. You may need to ask your AWS administrator or team mates for this information.

## Authenticate with your AWS identity center account

1. Log into the SSO session. This will require a web browser to go through the authentication flow for your SSO provider.

    https://github.com/user-attachments/assets/ebbafe38-17ac-4531-9943-ef29660085c5

2. A new browser tab will open asking you to confirm the code in your Terminal matches the code on-screen. **If they match**, choose _Allow_.

    <div><img src="docs/auth@2x.png" alt="Match the authorization code" width="50%"></div>

3. You will be asked to grant permission to a client. Choose _Allow_.

    <div><img src="docs/approve@2x.png" alt="Grant permission to botocore" width="50%"></div>

## Listing and updating your accounts

> [!NOTE]
> The `aws-sso-manager: start` and `end` markers are what allows the `update` functionality to work correctly. Removing them will break updates. Please leave these markers in-place.

With a user configuration file, you can control how your AWS accounts and profile names are generated.

https://github.com/user-attachments/assets/0ff7303e-19a4-40b4-9f15-4c2a0019b601

See `docs/config_file.md` for more information.

## Configuring a default AWS Organization

In order to save some typing, if there's one AWS Organization that you use most often, you can set it as your default value. If you want to run commands on a different AWS Organization, you can still specify the alias at runtime.

https://github.com/user-attachments/assets/5125fc70-f3f8-472e-a093-5a1beb1ec91d

## Lookup accounts and role names

You often need to lookup AWS Account IDs or SSO profile names as part of running commands with AWS CLI. AWS SSO Manager can simplify this.

https://github.com/user-attachments/assets/8accdeb0-8469-4354-a170-16e67f3983da

## Generate console links

One of the things that AWS Identity Center enables is the ability to generate links to the AWS Console with a built-in AWS Account ID and Organizations Role.

https://github.com/user-attachments/assets/0b1b37e9-7bf8-4966-8460-874199de957c

## Usage

### With AWS CLI (via `--profile`)

```bash
aws s3 ls --profile {PROFILE}
```

### With AWS CLI (via `AWS_PROFILE`)

```bash
AWS_PROFILE={PROFILE} aws s3 ls
```

### With [AWS vault](https://github.com/ByteNess/aws-vault/blob/main/USAGE.md) (via `exec` or `login`)

```bash
aws-vault exec {PROFILE} -- aws s3 ls
```

```bash
aws-vault login {PROFILE}
```

### With [AWS SDKs](https://builder.aws.com/build/tools)

See the documentation for your specific SDK, but everything supported in the AWS CLI is also supported in the AWS SDKs.

## Troubleshooting

### `The profile [sso-session {PROFILE}] does not exist in the AWS config file.`

Make sure you replaced `{PROFILE}` with the actual name of the profile. The use of `{PROFILE}` in the code samples is just a placeholder.

## Comparisons

### Compared to aws-sso-cli

This project, and [aws-sso-cli], have some overlap. This project tries to follow the Unix philosophy of _doing one thing well_. Both projects are written in Go and have zero runtime dependencies. `aws-sso-cli` begins to cross the boundary into what [AWS Vault] does well, and has several additional integrations that are powerful at the cost of complexity. This project is intentionally less complex, and focuses more narrowly on managing SSO profiles. This project is licensed under the "open source" [Apache 2.0][Apache-2.0] license, while `aws-sso-cli` is licensed under the "free software" [GPL 3.0 (or later)][GPL-3.0-or-later] license.

### Compared to AWS CLI v2

This project, and [AWS CLI v2], are _complementary_. `aws-sso-manager` is intended to _replace_ the `aws configure sso` command. It streamlines the work of setting up AWS SSO; supports pattern-matching to rename accounts, roles, and profiles locally; and makes it much easier to update your current list of SSO roles as they change over time.

[AWS CLI v2] leverages the profile configurations that `aws-sso-manager` provides.

### Compared to AWS Vault

This project, and [AWS Vault], are _complementary_. For the purpose of managing AWS Identity Center (née _SSO_), [AWS Vault] leverages the same AWS profile configuration that [AWS CLI v2] uses to authenticate, which is generated and maintained by this project.

### Compared to Gimme Creds

[Gimme Creds] is a Python solution for organizations which use the older SAML integration with AWS instead of the newer AWS Identity Center solution. It also requires the use of Okta as the Identity Provider (IdP).

This project is focused on AWS Identity Center, therefore it never crosses paths with [Gimme Creds]. They solve entirely different problems.

### saml2aws

[saml2aws] is a Go solution for organizations which use the older SAML integration with AWS instead of the newer AWS Identity Center solution. It supports several Identity Providers (IdPs).

This project is focused on AWS Identity Center, therefore it never crosses paths with [saml2aws]. They solve entirely different problems.

[Apache-2.0]: https://choosealicense.com/licenses/apache-2.0/
[AWS CLI v2]: https://aws.amazon.com/cli/
[AWS Identity Center]: https://aws.amazon.com/iam/identity-center/
[AWS Vault]: https://github.com/ByteNess/aws-vault
[aws-sso-cli]: https://synfinatic.github.io/aws-sso-cli/latest/
[Gimme Creds]: https://github.com/Nike-Inc/gimme-aws-creds
[GPL-3.0-or-later]: https://choosealicense.com/licenses/gpl-3.0/
[saml2aws]: https://github.com/Versent/saml2aws

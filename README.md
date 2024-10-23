# Adding AWS Identity Center (née AWS SSO) accounts

Logging into a user interface to grab credentials that you copy-paste into your shell session is both **cumbersome** and **insecure**.

By leveraging [AWS Vault] backed by [AWS Identity Center] (née AWS SSO), we can pull credentials _easily_ and keep them secure by passing them exclusively to an ephemeral sub-shell.

## Prerequisites

### macOS

<details>
<summary>See list…</summary>

* [Xcode Command Line Tools](https://mac.install.guide/commandlinetools/)
* [Homebrew](https://mac.install.guide/homebrew/)
* [jq](https://jqlang.github.io/jq/)

    ```bash
    brew install jq
    ```

* [AWS CLI](https://aws.amazon.com/cli/)

    ```bash
    brew install awscli
    ```

* [AWS Vault](https://github.com/99designs/aws-vault/releases/latest)

    ```bash
    brew install aws-vault
    ```

    **Suggestion:** Unless you genuinely prefer using a separate macOS keychain for AWS Vault credentials, we **highly suggest** setting the `AWS_VAULT_KEYCHAIN_NAME=login` environment variable in your shell profile. The `login` keychain is unlocked when you log into macOS, and locked when you log out. Setting this will save you an extra password-entry step.

</details>

### Linux

<details>
<summary>See list…</summary><br>

* [jq](https://jqlang.github.io/jq/)
* [AWS CLI](https://aws.amazon.com/cli/)
* [AWS Vault](https://github.com/99designs/aws-vault/releases/latest)

</details>

### Windows

Use [WSL2](https://learn.microsoft.com/en-us/windows/wsl/install), then follow the instructions for Linux.

## Preface

These instructions were written using the following versions of tools:

```bash
$ aws --version
aws-cli/2.17.49 Python/3.11.10 Darwin/23.6.0 source/arm64
```

```bash
$ aws-vault --version
v7.2.0
```

## [Existing users] Updating your set of AWS credentials

1. Log into the SSO session. This will require a web browser to go through the Okta authentication flow.

    ```bash
    aws sso login --sso-session nwl
    ```

1. Get an understanding of the differences between what you have configured for AWS CLI and/or AWS Vault, and what you have access to via AWS SSO (both tools reference the SSO entries in `~/.aws/config`).

    ```bash
    ./account-diff.sh
    ```

## [New users] Setting-up your AWS credentials for the first time

If you have a file called `~/.aws/credentials`, the goal is to **delete** it so that only `~/.aws/config` remains. We will NOT be storing credentials in plain text.

1. Since this is the first time you're adding credentials, your `~/.aws/config` should be pretty much blank.

    Add the following INI entry to the file:

    ```ini
    [sso-session nwl]
    sso_start_url = https://d-9a6770fb65.awsapps.com/start
    sso_region = us-east-2
    sso_registration_scopes = sso:account:access
    ```

1. Log into the SSO session. This will require a web browser to go through the Okta authentication flow.

    ```bash
    aws sso login --sso-session nwl
    ```

1. A new browser tab will open asking you to confirm the code in your Terminal matches the code on-screen. **If they match**, choose _Allow_.

    <div><img src="docs/auth@2x.png" alt="Match the authorization code" width="50%"></div>

1. You will be asked to grant permission to `botocore-client-nwl`. Choose _Allow_.

    <div><img src="docs/approve@2x.png" alt="Grant permission to botocore" width="50%"></div>

### Generate updated configuration

> [!IMPORTANT]
> Make sure you've cloned this repo (or downloaded the latest copy of it), and your terminal is `cd`'d into the directory containing this README.

1. Run the script. This will write AWS configuration to `stdout`.

    ```bash
    ./all-sso-accounts.sh
    ```

    The AWS configuration profiles will look something like this:

    ```ini
    [profile nonprod]
    sso_session = nwl
    sso_account_id = 381492142681
    sso_role_name = NWL-PowerUserAccess
    region = us-east-2
    output = json
    ```

1. If you have multiple roles that you are allowed to assume, they will be split into multiple profiles in the `sso_role_name` key of each profile. `NWL-AdministratorAccess`, `NWL-PowerUserAccess`, and `NWL-ReadOnlyAccess` are handled. (If we roll-out another role, this script should be updated.)

## Usage

### With AWS CLI (via `--profile`)

```bash
aws s3 ls --profile {PROFILE}
```

### With AWS CLI (via `AWS_PROFILE`)

```bash
AWS_PROFILE={PROFILE} aws s3 ls
```

### With [AWS Vault](https://github.com/99designs/aws-vault/blob/master/USAGE.md) (via `exec` or `login`)

```bash
aws-vault exec {PROFILE} -- aws s3 ls
```

```bash
aws-vault login {PROFILE}
```

### With [AWS SDKs](https://aws.amazon.com/developer/tools/)

See the documentation for your specific SDK, but everything supported in the AWS CLI is also supported in the AWS SDKs.

## Troubleshooting

### `Invalid endpoint: https://portal.sso..amazonaws.com`

Log into the SSO session first.

```bash
aws sso login --sso-session nwl
```

### `The config profile ({PROFILE}) could not be found`

Make sure you replaced `{PROFILE}` with the actual name of the profile. The use of `{PROFILE}` in the code samples is just a placeholder.

[AWS Identity Center]: https://aws.amazon.com/iam/identity-center/
[AWS Vault]: https://github.com/99designs/aws-vault

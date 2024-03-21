# Adding AWS Identity Center (née AWS SSO) accounts to AWS Vault

Logging into a user interface to grab credentials that you copy-paste into your shell session is both **cumbersome** and **insecure**.

By leveraging [AWS Vault] backed by [AWS Identity Center] (née AWS SSO), we can pull credentials _easily_ and keep them secure by passing them exclusively to an ephemeral sub-shell.

## Preface

These instructions were written using the following versions of tools:

```bash
$ aws --version
aws-cli/2.15.23 Python/3.11.8 Darwin/23.4.0 source/arm64 prompt/off
```

```bash
$ aws-vault --version
v7.2.0
```

## Fetch tokens from AWS Identity Center

> [!CAUTION]
> This may mean that you'll need to re-auth things like VS Code, AWS CLI, and others.

1. Delete some (possibly) cached files related to AWS authorization so that we begin with a fresh slate.

    ```bash
    find ~/.aws/sso/cache/ -type f -name "*.json" | xargs -I% rm -Rf "%"
    ```

1. Begin the SSO process.

    ```bash
    aws configure sso
    ```

1. Enter the SSO session name. This is how you can identify _this_ company's SSO vs another company's SSO. I chose:

    ```text
    northwood-labs
    ```

1. Enter the SSO start URL.

    ```text
    https://d-9a6770fb65.awsapps.com/start
    ```

1. Enter the AWS region.

    ```text
    us-east-2
    ```

1. For _SSO registration scopes_, press the RETURN key to accept the default value.

1. You will be prompted to approve the code in your web browser. Check to make sure the code in the browser matches the code in your terminal.

    <div><img src="docs/auth@2x.png" alt="Verify code" width="50%"></div>

1. A new browser tab will open asking you to grant permission to `botocore-client-northwood-labs`. Choose _Allow_.

    <div><img src="docs/approve@2x.png" alt="Grant permission to botocore" width="50%"></div>

1. Once approved, your terminal should display a listing of accounts that you have access to.

    <div><img src="docs/terminal@2x.png" alt="Terminal view of AWS accounts"></div>

1. **Quit the CLI app** with `Ctrl+C`.

1. Verify that a file has been cached to disk. We will use the credentials in this file for looking up additional data.

    ```bash
    find ~/.aws/sso/cache/ -type f | grep -v botocore | grep -v aws-toolkit
    ```

## Understand the lay of the land

Get an understanding of the differences between what you have configured for AWS Vault already, and what you have access to via AWS Identity Center.

```bash
./account-diff.sh
```

## Generate updated configuration

1. Run the script. This will write AWS configuration to `stdout`.

    ```bash
    ./all-sso-accounts.sh
    ```

1. If there are any error messages in the output, remove them.

1. If you have multiple roles that you are allowed to assume, they will be split into multiple profiles in the `sso_role_name` key of each profile. `NWL-AdministratorAccess`, `NWL-PowerUserAccess`, and `NWL-ReadOnlyAccess` are handled. (If we roll-out another role, this script should be updated.)

## Updating `~/.aws/config`

Simply begin adding the appropriate `profile` blocks that were generated, inside `~/.aws/config`.

> [!NOTE]
> The `credential_process` profile key is not used by AWS Identity Center, so this can be removed.

## Adding single accounts from AWS Identity Center → `aws-vault`

You can pass the 12-digit AWS Account ID from the AWS Identity Center dashboard + the intended name to the `single-account.sh` script.

1. Open the AWS Identity Center dashboard with the complete account listing.

1. Copy-paste the values from the dashboard into your Terminal session.

    ```bash
    ./single-account.sh 0123456789012 sandbox
    ```

1. This will generate the output that can be copy-pasted into your `~/.aws/config` file.

    ```ini
    [sso-session northwood-labs-sso]
    sso_start_url = https://d-9a6770fb65.awsapps.com/start
    sso_region = us-east-2
    sso_registration_scopes = sso:account:access

    [profile sandbox-admin]
    sso_session = northwood-labs-sso
    sso_account_id = 590184084631
    sso_role_name = NWL-AdministratorAccess
    region = us-east-2
    output = json

    [profile devenv-sandbox-admin]
    source_profile=sandbox-admin
    role_arn=arn:aws:iam::590184084631:role/dev-env

    [profile sandbox]
    sso_session = northwood-labs-sso
    sso_account_id = 590184084631
    sso_role_name = NWL-PowerUserAccess
    region = us-east-2
    output = json

    [profile sandbox-ro]
    sso_session = northwood-labs-sso
    sso_account_id = 590184084631
    sso_role_name = NWL-ReadOnlyAccess
    region = us-east-2
    output = json
    ```

## Verify account

This can log you _directly_ into the AWS Management Console with the correct role.

```bash
aws-vault login sandbox
```

This can pass the appropriate AWS Session Tokens to any software (not just the official AWS CLI) that understands them.

```bash
aws-vault exec sandbox -- aws sts get-caller-identity
```

[AWS Identity Center]: https://aws.amazon.com/iam/identity-center/
[AWS Vault]: https://github.com/99designs/aws-vault

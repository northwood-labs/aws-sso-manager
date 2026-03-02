# Adding AWS Identity Center accounts

Logging into a user interface to grab credentials that you copy-paste into your shell session is both **cumbersome** and **insecure**.

This tool will help you manage your AWS Organizations and [AWS Identity Center] (née AWS SSO) profiles stored in `~/.aws/config`.

These SSO profiles can be used with `$AWS_PROFILE` or [AWS Vault], allowing you to pull credentials _easily_ and keep them secure by passing them exclusively to an ephemeral sub-shell.

It is _complementary_ to the AWS CLI and tools like [AWS Vault].

## Adding your AWS Identity Center account

> [!NOTE]
> If you have a file called `~/.aws/credentials`, the goal is to **delete** it so that only `~/.aws/config` remains. We will NOT be storing credentials in plain text.

1. Since this is the first time you're adding credentials, your `~/.aws/config` should be blank. If you already have credentials there, that's fine, but if you want this tool to manage them, you should delete them and set them up anew.

    ```bash
    aws-sso-vault init <ID>
    ```

    The `<ID>` value is simply how you want to refer to it in your configuration. We recommend something very short and easy to type (e.g., `goog`, `msft`, `appl`, `nwl`, `mhe`, `strp`, `rax`, `swa`).

    You will be asked for your _SSO Start URL_, and your SSO region. You may need to ask your AWS administrator or team mates for this information.

1. If you run `cat ~/.aws/config`, you should see the following at the bottom of the output:

    ```text
    ; -------- aws-sso-vault: start abc --------
    [sso-session abc]
    sso_region = us-east-1
    sso_registration_scopes = sso:account:access
    sso_start_url = https://abc.awsapps.com/start
    ; -------- aws-sso-vault: end abc --------
    ```

## Authenticate with your AWS Identity Center account

1. Log into the SSO session. This will require a web browser to go through the authentication flow for your SSO provider.

    ```bash
    aws-sso-vault auth <ID>
    ```

1. A new browser tab will open asking you to confirm the code in your Terminal matches the code on-screen. **If they match**, choose _Allow_.

    <div><img src="docs/auth@2x.png" alt="Match the authorization code" width="50%"></div>

1. You will be asked to grant permission to a client. Choose _Allow_.

    <div><img src="docs/approve@2x.png" alt="Grant permission to botocore" width="50%"></div>

## Updating your accounts

> [!NOTE]
> The `aws-sso-vault: start` and `end` markers are what allows the `update` functionality to work correctly. Removing them will break updates. Please leave these markers in-place.

1. If you want to see the list of accounts and roles you have access to via AWS Identity Center, run `list`.

    ```bash
    aws-sso-vault list <ID>
    ```

1. If you want to update your config file with the current set of accounts and profiles you have available, run the following:

    ```bash
    aws-sso-vault update <ID>
    ```

1. You can view the configuration with `cat ~/.aws/config`. If you do not like how the names were generated, or if you don't like the names that your AWS administrator configured on the server side, you can override them.

    See `docs/config_file.md` for more information.

## Generate console links

1. One of the things that AWS Identity Center enables is the ability to generate links to the AWS Console with a built-in AWS Account ID and Organizations Role.

    ```bash
    aws-sso-vault console <ID> <CONSOLE_URL>
    ```

    It will ask for you to make a few other decisions, then will generate a URL. It will bounce you through authentication, then log into the AWS console with a particular AWS Account ID and Organizations Role.

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

### `The profile [sso-session {PROFILE}] does not exist in the AWS config file.`

Make sure you replaced `{PROFILE}` with the actual name of the profile. The use of `{PROFILE}` in the code samples is just a placeholder.

[AWS Identity Center]: https://aws.amazon.com/iam/identity-center/
[AWS Vault]: https://github.com/ByteNess/aws-vault

#! /usr/bin/env bash

set -euo pipefail

# shellcheck disable=1091
source list-accounts.sh

data="$(accesstokenfromcache)"
SSO_ACCOUNTS="$(listaccounts | sed 's/ /:/')"

# echo "${SSO_ACCOUNTS}"
cat <<EOF
[sso-session nwl]
sso_start_url = $(echo "${data}" | jq -r '.startUrl' || true)
sso_region = $(echo "${data}" | jq -r '.region' || true)
sso_registration_scopes = sso:account:access

EOF

# Loop over each of the accounts.
for ACCT in ${SSO_ACCOUNTS}; do
    IFS='-'
    # shellcheck disable=SC2206
    arrIN=(${ACCT})
    unset IFS

    # Use the AWS Account ID for the account from the SSO lookup.
    AWS_ID="${arrIN[0]}"
    # echo "AWS_ID=${AWS_ID}"
    ACCT="${arrIN[1]}"
    # echo "NAME=${NAME}"

    # Get a comma-delimited list of AWS Roles that you have access to. After this
    # runs, you should select a single role to keep for that profile entry.
    ROLES=$(
        # shellcheck disable=2312
        aws sso list-account-roles --access-token "$(echo "${data}" | jq -r '.accessToken')" --region "$(echo "${data}" | jq -r '.region')" --account-id "${AWS_ID}" --output json |
            jq -Mr '.roleList[].roleName'
    )

    for role in ${ROLES}; do
        if [[ ${role} == *"AdministratorAccess"* ]]; then
            echo "[profile ${ACCT}-admin]"
        elif [[ ${role} == *"PowerUserAccess"* ]]; then
            echo "[profile ${ACCT}]"
        elif [[ ${role} == *"ReadOnlyAccess"* ]]; then
            echo "[profile ${ACCT}-ro]"
        fi
        cat <<EOF
sso_session = nwl
sso_account_id = ${AWS_ID}
sso_role_name = ${role}
region = $(echo "${data}" | jq -r '.region' || true)
output = json

EOF

        # Adapted from https://github.com/99designs/aws-vault/blob/master/USAGE.md#docker
        if [[ ${role} == *"AdministratorAccess"* ]]; then
            echo "[profile devenv-${ACCT}-admin]"
            echo "source_profile=${ACCT}-admin"
            echo "role_arn=arn:aws:iam::${AWS_ID}:role/dev-env"
            echo ""
        fi
    done
done

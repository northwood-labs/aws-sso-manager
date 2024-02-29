#! /usr/bin/env bash
set -euo pipefail

# shellcheck disable=1091
source list-accounts.sh

AWS_ID="$1"
ACCT="$2"
data="$(accesstokenfromcache)"

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
    cat << EOF
sso_start_url = $(echo "${data}" | jq -r '.startUrl' || true)
sso_account_id = ${AWS_ID}
sso_role_name = ${role}
sso_region = $(echo "${data}" | jq -r '.region' || true)
region = $(echo "${data}" | jq -r '.region' || true)
output = json

EOF

    # Adapted from https://github.com/99designs/aws-vault/blob/master/USAGE.md#docker
    if [[ ${role} == *"AdministratorAccess"* ]]; then
        echo "[profile devenv-${ACCT}-admin]"
        echo "source_profile=${ACCT}-admin"
    elif [[ ${role} == *"PowerUserAccess"* ]]; then
        echo "[profile devenv-${ACCT}]"
        echo "source_profile=${ACCT}"
    elif [[ ${role} == *"ReadOnlyAccess"* ]]; then
        echo "[profile devenv-${ACCT}-ro]"
        echo "source_profile=${ACCT}-ro"
    fi
    cat << EOF
role_arn=arn:aws:iam::${AWS_ID}:role/dev-env

EOF
done

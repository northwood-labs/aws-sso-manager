#! /usr/bin/env bash
set -euo pipefail

# shellcheck disable=1091
source list-accounts.sh

echo "==> Accounts in AWS SSO..."
listaccounts |
    awk '{ print $1 }' |
    tee /tmp/sso-list.txt |
    tr '[:space:]' ' '

echo " "
echo " "

echo "==> Accounts in AWS SSO but not in aws-vault..."
grep -v -f /tmp/local-list.txt /tmp/sso-list.txt | tr '[:space:]' ' ' || echo -n "None"

echo " "
echo " "

echo "==> List of commands for importing the remaining AWS SSO accounts into aws-vault..."
./generate-single-account-list.sh

#! /usr/bin/env bash
set -euo pipefail

# shellcheck disable=1091
source list-accounts.sh

LIST="$(listaccounts)"
MISSING="$(grep -v -f /tmp/local-list.txt /tmp/sso-list.txt | tr '[:space:]' ' ')"

IFS=$'\n' read -d "\034" -r -a array <<<"${LIST}\034"

for account in "${array[@]}"; do
    for mm in "${MISSING[@]}"; do
        if [[ "${account}" == *"${mm}"* ]]; then
            echo "./single-account.sh ${account}"
        fi
    done
done

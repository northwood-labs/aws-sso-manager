#! /usr/bin/env bash
set -euo pipefail

# shellcheck disable=1091
source list-accounts.sh

LIST="$(listaccounts)"

IFS=$'\n' read -d "\034" -r -a array <<< "${LIST}\034"

for account in "${array[@]}"; do
    echo "./single-account.sh ${account/-/ }"
done

#!/usr/bin/env bash
set -euo pipefail

accesstokenfromcache() {
    FILES="$(find ~/.aws/sso/cache/ -type f)"
    FILES="$(echo "${FILES}" | grep -v aws-toolkit)"
    FILES="$(echo "${FILES}" | grep -v botocore)"

    for FILE in ${FILES}; do
        valueOfAccessToken="$(jq -r "{accessToken} | .[]" "${FILE}")"

        if [[ ${valueOfAccessToken} != "null" ]]; then
            jq -Mrc "{accessToken, startUrl, region}" "${FILE}"
            return
        fi
    done
}

listaccounts() {
    data="$(accesstokenfromcache)"

    # shellcheck disable=2312
    aws sso list-accounts --access-token "$(echo "${data}" | jq -r '.accessToken')" --region "$(echo "${data}" | jq -r '.region')" --output json |
        jq -Mr '.accountList[] | "\(.accountId)-\(.accountName)"' |
        tr '[:upper:]' '[:lower:]' |
        sed "s/production/prod/g" |
        sed "s/non-prod/nonprod/g" |
        sort
}

#!/usr/bin/env bash

# @config-manager:start gommit
# shellcheck disable=2312
gommit check message "$(cat "$1")"
# @config-manager:end gommit

#!/usr/bin/env bash
# Annotates brew/cask lines in Brewfiles with their homepage URLs from `brew info`.
# Idempotent: strips existing URL comments before re-adding them.
# Usage: ./annotate-brewfiles.sh Brewfile.all Brewfile.standard

set -euo pipefail

if [[ $# -eq 0 ]]; then
    printf 'Usage: %s <Brewfile> [Brewfile2 ...]\n' "$0" >&2
    exit 1
fi

CACHE_DIR=$(mktemp -d)
SEEN_DIR=$(mktemp -d)
trap 'rm -rf "$CACHE_DIR" "$SEEN_DIR"' EXIT

lookup_url() {
    local type="$1" pkg="$2"
    local outfile="$CACHE_DIR/${type}__${pkg//\//__}"
    local url

    if [[ "$type" == "cask" ]]; then
        url=$(brew info --cask "$pkg" 2>/dev/null | awk '/^https?:\/\// { print; exit }')
    else
        url=$(brew info "$pkg" 2>/dev/null | awk '/^https?:\/\// { print; exit }')
    fi

    printf '%s' "$url" > "$outfile"
}

export -f lookup_url
export CACHE_DIR

# Phase 1: Collect unique packages and look them up in parallel
printf 'Launching parallel URL lookups...\n' >&2

for file in "$@"; do
    while IFS= read -r line; do
        type="" pkg=""

        if [[ "$line" =~ ^brew[[:space:]]+\"([^\"]+)\" ]]; then
            type="brew"; pkg="${BASH_REMATCH[1]}"
        elif [[ "$line" =~ ^cask[[:space:]]+\"([^\"]+)\" ]]; then
            type="cask"; pkg="${BASH_REMATCH[1]}"
        fi

        if [[ -n "$pkg" ]]; then
            seen_file="$SEEN_DIR/${type}__${pkg//\//__}"
            if [[ ! -f "$seen_file" ]]; then
                touch "$seen_file"
                lookup_url "$type" "$pkg" &
            fi
        fi
    done < "$file"
done

LAUNCHED=$(ls -1 "$SEEN_DIR" | wc -l | tr -d ' ')
printf 'Waiting for %s lookups across all CPU cores...\n' "$LAUNCHED" >&2
wait
printf 'Done. Annotating files...\n' >&2

# Phase 2: Rewrite files with URL comments inserted above each brew/cask line.
# Strips any existing URL comment lines first so the script is idempotent.
for file in "$@"; do
    tmpout=$(mktemp)

    while IFS= read -r line; do
        # Strip previously written URL-comment lines
        [[ "$line" =~ ^#[[:space:]]https?:// ]] && continue

        type="" pkg=""
        if [[ "$line" =~ ^brew[[:space:]]+\"([^\"]+)\" ]]; then
            type="brew"; pkg="${BASH_REMATCH[1]}"
        elif [[ "$line" =~ ^cask[[:space:]]+\"([^\"]+)\" ]]; then
            type="cask"; pkg="${BASH_REMATCH[1]}"
        fi

        if [[ -n "$pkg" ]]; then
            url=$(cat "$CACHE_DIR/${type}__${pkg//\//__}" 2>/dev/null || true)
            [[ -n "$url" ]] && printf '# %s\n' "$url"
        fi

        printf '%s\n' "$line"
    done < "$file" > "$tmpout"

    mv "$tmpout" "$file"
    printf '  Annotated: %s\n' "$file" >&2
done

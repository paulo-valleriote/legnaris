#!/usr/bin/env bash
# check-env-keys.sh <example-file> < production-env
# Compares the active KEY= names of the production environment (stdin) with the
# documented example file before the file reaches the VPS. Every active key of
# the example is required; a key the example documents only as a comment
# (`# KEY=value`) is optional and allowed. Anything else is unknown. Values are
# never printed.
set -euo pipefail

EXAMPLE="${1:-}"
if [[ -z "$EXAMPLE" || ! -r "$EXAMPLE" ]]; then
  echo "usage: check-env-keys.sh <example-file> < production-env" >&2
  exit 64
fi

# active_keys prints the sorted, unique variable names assigned in an env file.
# Comment lines never match, so a key documented only as a comment is absent.
active_keys() {
  { grep -E '^[A-Za-z_][A-Za-z0-9_]*=' "$1" || true; } | cut -d= -f1 | sort -u
}

# optional_keys prints the sorted, unique names the file documents as
# commented-out assignments. Only UPPER_CASE names directly followed by `=`
# count, so prose comments do not widen the allowed set.
optional_keys() {
  { grep -E '^#[[:space:]]*[A-Z][A-Z0-9_]*=' "$1" || true; } | sed -E 's/^#[[:space:]]*//' | cut -d= -f1 | sort -u
}

expected="$(active_keys "$EXAMPLE")"
allowed="$({ printf '%s\n' "$expected"; optional_keys "$EXAMPLE"; } | sed '/^$/d' | sort -u)"
actual="$(active_keys /dev/stdin)"
missing="$(comm -23 <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") | sed '/^$/d')"
unknown="$(comm -13 <(printf '%s\n' "$allowed") <(printf '%s\n' "$actual") | sed '/^$/d')"

status=0
while IFS= read -r key; do
  [[ -z "$key" ]] || { echo "missing in production env: $key" >&2; status=1; }
done <<<"$missing"
while IFS= read -r key; do
  [[ -z "$key" ]] || { echo "unknown in production env: $key" >&2; status=1; }
done <<<"$unknown"

if (( status == 0 )); then
  echo "production env keys match $EXAMPLE"
fi
exit "$status"

#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${GENEVAL_BIN:-}" ]]; then
  printf 'GENEVAL_BIN is not set\n' >&2
  exit 2
fi
if [[ $# -lt 1 ]]; then
  printf 'a concept argument is required\n' >&2
  exit 2
fi
exec "$GENEVAL_BIN" "$1"

#!/usr/bin/env bash
# Succeeds only when the commit is reachable from the main branch, so a manual
# redeploy cannot put an unreviewed commit on the VPS. The CI job checks out
# main with full history before calling this. Any git failure fails closed.
set -euo pipefail

SHA="${1:-}"
if [[ ! "$SHA" =~ ^[0-9a-f]{40}$ ]]; then
  echo "usage: check-ancestry.sh <40-hex sha>" >&2
  exit 64
fi

BASE="${BASE_REF-origin/main}"
if [[ -z "$BASE" || "$BASE" == -* ]]; then
  echo "usage: BASE_REF must be a non-empty ref that does not start with '-'" >&2
  exit 64
fi
if git merge-base --is-ancestor "$SHA" "$BASE"; then
  echo "$SHA is on $BASE"
  exit 0
fi
echo "$SHA is not an ancestor of $BASE" >&2
exit 1

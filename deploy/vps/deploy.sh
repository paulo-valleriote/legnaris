#!/usr/bin/env bash
# Deploys one immutable image tag on the VPS. Run by the CD workflow over SSH:
#   deploy/vps/deploy.sh <40-hex commit sha>
# The checkout is pinned to the same commit so compose files, Traefik and
# Prometheus configs always match the images being started. If the new stack
# does not become healthy, the previous commit is redeployed.
set -euo pipefail

SHA="${1:-}"
if [[ ! "$SHA" =~ ^[0-9a-f]{40}$ ]]; then
  echo "usage: deploy.sh <40-hex commit sha>" >&2
  exit 64
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STATE="$ROOT/deploy/.deployed_sha"
cd "$ROOT"

PREVIOUS="$(cat "$STATE" 2>/dev/null || true)"

compose() {
  IMAGE_TAG="sha-$1" docker compose --project-directory "$ROOT/deploy" \
    -f "$ROOT/deploy/docker-compose.yml" -f "$ROOT/deploy/docker-compose.vps.yml" "${@:2}"
}

# release must use explicit "|| return 1": a function called from an `if`
# condition runs with errexit disabled, so a bare failing command would be
# ignored and `up` would start a stack that was never fetched or pulled.
release() {
  local sha="$1"
  git fetch --depth 1 origin "$sha" || return 1
  git checkout --detach "$sha" || return 1
  compose "$sha" pull || return 1
  compose "$sha" up -d --remove-orphans --wait --wait-timeout 180 || return 1
}

# ensure_on_main refuses a commit that is not reachable from origin/main. It is
# skipped when no repository is present, and fails closed when main cannot be
# fetched, so an unreviewed commit never reaches the stack.
ensure_on_main() {
  local sha="$1"
  [[ -d "$ROOT/.git" ]] || return 0
  if ! git fetch --quiet origin main; then
    echo "could not fetch origin/main" >&2
    return 1
  fi
  if ! git merge-base --is-ancestor "$sha" origin/main; then
    echo "$SHA is not an ancestor of origin/main" >&2
    return 66
  fi
}

ensure_on_main "$SHA" || exit "$?"

if release "$SHA"; then
  echo "$SHA" >"$STATE"
  docker image prune -f >/dev/null
  echo "deployed $SHA"
  exit 0
fi

echo "deploy of $SHA failed" >&2
if [[ -n "$PREVIOUS" && "$PREVIOUS" != "$SHA" ]]; then
  echo "rolling back to $PREVIOUS" >&2
  release "$PREVIOUS" || echo "rollback also failed; inspect 'docker compose ps'" >&2
fi
exit 1

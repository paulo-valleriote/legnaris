#!/usr/bin/env bash
# Forced command for the CI key. The authorized_keys line runs this script with
# `restrict`, so the key can do nothing but the two requests below:
#   deploy <40-hex sha>          deploy that commit
#   deploy <40-hex sha> env      replace deploy/.env with stdin, then deploy
# SSH_ORIGINAL_COMMAND is only split into words and compared, never evaluated.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEPLOY_DIR="$ROOT/deploy"
SHA_PATTERN='^[0-9a-f]{40}$'

usage() {
  echo "usage: deploy <40-hex sha> [env]" >&2
  exit 64
}

# parse_request sets the global words array to the validated request tokens.
parse_request() {
  local command="${SSH_ORIGINAL_COMMAND:-}"
  [[ "$command" != *$'\n'* ]] || usage
  read -r -a words <<<"$command"
  (( ${#words[@]} == 2 || ${#words[@]} == 3 )) || usage
  [[ "${words[0]}" == "deploy" ]] || usage
  [[ "${words[1]}" =~ $SHA_PATTERN ]] || usage
  if (( ${#words[@]} == 3 )); then
    [[ "${words[2]}" == "env" ]] || usage
  fi
}

# write_env replaces deploy/.env atomically: the new content goes to a temporary
# file in the same directory at mode 600 and is renamed over the old one, so a
# dropped connection never leaves a partial or world-readable file.
write_env() {
  local tmp
  umask 077
  tmp="$(mktemp "$DEPLOY_DIR/.env.XXXXXX")"
  trap 'rm -f "$tmp"' EXIT
  cat >"$tmp"
  if [[ ! -s "$tmp" ]]; then
    echo "refusing to replace .env with empty input" >&2
    exit 65
  fi
  chmod 600 "$tmp"
  mv -f "$tmp" "$DEPLOY_DIR/.env"
  trap - EXIT
}

parse_request
if (( ${#words[@]} == 3 )); then
  write_env
fi
exec "$DEPLOY_DIR/vps/deploy.sh" "${words[1]}"

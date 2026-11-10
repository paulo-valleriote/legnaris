#!/usr/bin/env bash
# probe.sh <url>: succeeds once the URL answers with a 2xx status. Retries
# PROBE_ATTEMPTS times, PROBE_INTERVAL seconds apart; each try is capped at
# PROBE_TIMEOUT seconds, so a slow start does not fail the run and a dead
# endpoint does not hang it.
set -euo pipefail

URL="${1:-}"
ATTEMPTS="${PROBE_ATTEMPTS-5}"
INTERVAL="${PROBE_INTERVAL-5}"
TIMEOUT="${PROBE_TIMEOUT-10}"

usage() {
  echo "usage: probe.sh <http(s) url>  (PROBE_ATTEMPTS, PROBE_INTERVAL, PROBE_TIMEOUT)" >&2
  exit 64
}

[[ "$URL" =~ ^https?://[^[:space:]]+$ ]] || usage
[[ "$ATTEMPTS" =~ ^[1-9][0-9]*$ ]] || usage
[[ "$TIMEOUT" =~ ^[1-9][0-9]*$ ]] || usage
[[ "$INTERVAL" =~ ^[0-9]+$ ]] || usage

for ((attempt = 1; attempt <= ATTEMPTS; attempt++)); do
  if curl -fsS --max-time "$TIMEOUT" -o /dev/null "$URL"; then
    echo "ok $URL (attempt $attempt)"
    exit 0
  fi
  if (( attempt < ATTEMPTS )); then
    sleep "$INTERVAL"
  fi
done

echo "probe failed: $URL after $ATTEMPTS attempts" >&2
exit 1

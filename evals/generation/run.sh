#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"

if [[ -z "${LLM_API_KEY:-}" && -f .env ]]; then
  set -a
  . ./.env
  set +a
fi
if [[ -z "${LLM_API_KEY:-}" ]]; then
  printf 'LLM_API_KEY is not set. In GitHub Actions create the OPENROUTER_API_KEY secret; locally export LLM_API_KEY or add it to .env.\n' >&2
  exit 1
fi

work="$(mktemp -d "${TMPDIR:-/tmp}/legnaris-geneval.XXXXXX")"
cleanup() {
  rm -rf "$work"
}
trap cleanup EXIT

go build -trimpath -o "$work/geneval" ./backend/cmd/geneval

export GENEVAL_BIN="$work/geneval"
export EVAL_COST_LEDGER="$work/cost.jsonl"
export EVAL_MAX_COST_USD="${EVAL_MAX_COST_USD:-1.00}"

info="$("$GENEVAL_BIN" -info)"

set +e
PROMPTFOO_PASS_RATE_THRESHOLD=90 npx --yes promptfoo@0.126.0 eval \
  -c evals/generation/promptfooconfig.yaml \
  --no-cache --no-progress-bar --no-write --no-share -j 1 \
  -o "$work/results.json"
eval_status=$?
set -e

set +e
node evals/generation/report.mjs "$work/results.json" "$EVAL_COST_LEDGER" "$info" "$EVAL_MAX_COST_USD"
report_status=$?
set -e

if [[ "$eval_status" -ne 0 ]]; then
  exit "$eval_status"
fi
exit "$report_status"

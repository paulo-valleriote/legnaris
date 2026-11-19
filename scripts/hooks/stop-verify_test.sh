#!/usr/bin/env bash
# Exercises the classification and skip logic of scripts/hooks/stop-verify.sh
# through its dry-run mode. Run by `make hooks-test` and the CI go-lint job.
set -uo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
hook="$script_dir/stop-verify.sh"

if [[ ! -x $hook ]]; then
  printf 'stop-verify_test: %s is not executable\n' "$hook" >&2
  exit 1
fi

checks=0
failures=0

fail() {
  printf 'not ok: %s\n' "$1" >&2
  failures=$((failures + 1))
}

expect() {
  local name=$1 payload=$2 files=$3 expected=$4 output status=0
  output=$(printf '%s' "$payload" | env -u CLAUDE_PROJECT_DIR STOP_VERIFY_DRY_RUN=1 STOP_VERIFY_BASE=origin/main STOP_VERIFY_FILES="$files" "$hook" 2>/dev/null) || status=$?
  checks=$((checks + 1))
  if ((status != 0)); then
    fail "$name (want exit 0, got $status)"
    return
  fi
  if [[ $output != "$expected" ]]; then
    fail "$name (unexpected plan)"
    diff -u <(printf '%s\n' "$expected") <(printf '%s\n' "$output") | sed 's/^/    /' >&2
    return
  fi
  printf 'ok: %s\n' "$name"
}

go_plan=$'golangci-lint run --new-from-rev=origin/main\ngo test ./backend/api/... ./backend/internal/sheet/...'
web_plan=$'cd web && npm run lint\ncd web && npm run typecheck\ncd web && npx --no-install vitest related --passWithNoTests src/App.tsx src/test/App.test.tsx'
both_plan=$'golangci-lint run --new-from-rev=origin/main\ngo test ./backend/api/...\n'"$web_plan"

expect "skips: no changes at all" \
  '{}' "" ""

expect "skips: only unrelated files" \
  '{}' $'README.md\ndocs/TDD.md\nscripts/dev.sh' ""

expect "skips: stop_hook_active already true" \
  '{"stop_hook_active": true}' $'backend/api/handler.go\nweb/src/App.tsx' ""

expect "skips: stop_hook_active with an empty file list" \
  '{"stop_hook_active": true}' "" ""

expect "plans: Go files only, one package per directory" \
  '{}' $'backend/internal/sheet/b.go\nbackend/api/a.go\nbackend/internal/sheet/a.go' "$go_plan"

expect "plans: a root-level Go file tests the whole module" \
  '{}' $'main.go\nbackend/api/a.go' \
  $'golangci-lint run --new-from-rev=origin/main\ngo test ./... ./backend/api/...'

expect "plans: web files only, ignoring e2e and non-code assets" \
  '{}' $'web/package.json\nweb/public/logo.svg\nweb/e2e/flow.spec.ts\nweb/src/test/App.test.tsx\nweb/src/App.tsx' "$web_plan"

expect "plans: web assets without lintable sources skip vitest" \
  '{}' $'web/index.html\nweb/src/theme.css' \
  $'cd web && npm run lint\ncd web && npm run typecheck'

expect "plans: both Go and web changes" \
  '{}' $'backend/api/a.go\nweb/src/App.tsx\nweb/src/test/App.test.tsx' "$both_plan"

expect "plans: an unreadable payload is treated as inactive" \
  'not json' 'backend/api/a.go' \
  $'golangci-lint run --new-from-rev=origin/main\ngo test ./backend/api/...'

expect "plans: a payload without stop_hook_active is inactive" \
  '{"hook_event_name": "Stop"}' 'web/src/App.tsx' \
  $'cd web && npm run lint\ncd web && npm run typecheck\ncd web && npx --no-install vitest related --passWithNoTests src/App.tsx'

if ((failures > 0)); then
  printf '%d of %d checks failed\n' "$failures" "$checks" >&2
  exit 1
fi

printf '%d checks passed\n' "$checks"

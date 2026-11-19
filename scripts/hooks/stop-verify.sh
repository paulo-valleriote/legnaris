#!/usr/bin/env bash
# Claude Code Stop hook: lint and test the packages touched since the merge
# base with origin/main, and block the stop with the failure output so the
# agent keeps working. Wired in the versioned .claude/settings.json.
set -uo pipefail

readonly LIMIT=4000
readonly DRY_RUN=${STOP_VERIFY_DRY_RUN:-0}

payload=""
if [[ ! -t 0 ]]; then
  payload=$(cat)
fi

stop_hook_active=false
if [[ -n ${payload//[[:space:]]/} ]]; then
  parsed=""
  if command -v python3 >/dev/null 2>&1; then
    parsed=$(printf '%s' "$payload" | python3 -c 'import json, sys
try:
    data = json.load(sys.stdin)
except Exception:
    raise SystemExit(1)
print("true" if data.get("stop_hook_active") is True else "false")' 2>/dev/null) || parsed=""
  elif command -v jq >/dev/null 2>&1; then
    parsed=$(printf '%s' "$payload" | jq -r 'if .stop_hook_active == true then "true" else "false" end' 2>/dev/null) || parsed=""
  elif [[ $payload =~ \"stop_hook_active\"[[:space:]]*:[[:space:]]*true ]]; then
    parsed=true
  fi
  [[ $parsed == true ]] && stop_hook_active=true
fi

if [[ $stop_hook_active == true ]]; then
  exit 0
fi

root=${CLAUDE_PROJECT_DIR:-}
if [[ -z $root ]]; then
  root=$(git rev-parse --show-toplevel 2>/dev/null || true)
fi
[[ -n $root ]] || root=$PWD
cd "$root" || exit 0

files=()
base=""

if [[ $DRY_RUN == 1 ]]; then
  base=${STOP_VERIFY_BASE:-origin/main}
  while IFS= read -r line; do
    line=${line%$'\r'}
    [[ -n $line ]] && files+=("$line")
  done < <(printf '%s\n' "${STOP_VERIFY_FILES:-}")
else
  base=$(git merge-base HEAD origin/main 2>/dev/null || true)
  if [[ -z $base ]]; then
    base=$(git merge-base HEAD main 2>/dev/null || true)
  fi
  if [[ -n $base ]]; then
    while IFS= read -r line; do
      [[ -n $line ]] && files+=("$line")
    done < <(git diff --name-only --diff-filter=ACMRT "$base"...HEAD 2>/dev/null || true)
  fi
  while IFS= read -r line; do
    [[ -n $line ]] || continue
    path=${line:3}
    path=${path#* -> }
    if [[ $path == \"*\" ]]; then
      path=${path#\"}
      path=${path%\"}
    fi
    [[ -n $path ]] && files+=("$path")
  done < <(git status --porcelain=v1 -uall 2>/dev/null || true)
fi

go_files=()
web_files=()
if ((${#files[@]} > 0)); then
  mapfile -t files < <(printf '%s\n' "${files[@]}" | awk 'NF' | sort -u)
  for file in "${files[@]}"; do
    case $file in
      *.go) go_files+=("$file") ;;
    esac
    case $file in
      web/*) web_files+=("$file") ;;
    esac
  done
fi

if ((${#go_files[@]} == 0 && ${#web_files[@]} == 0)); then
  printf 'stop-verify: no Go or web changes to verify\n' >&2
  exit 0
fi

go_dirs=()
declare -A seen_dirs=()
for file in "${go_files[@]}"; do
  dir=${file%/*}
  [[ $dir == "$file" ]] && dir=.
  [[ -n ${seen_dirs[$dir]:-} ]] && continue
  seen_dirs[$dir]=1
  go_dirs+=("$dir")
done
if ((${#go_dirs[@]} > 0)); then
  mapfile -t go_dirs < <(printf '%s\n' "${go_dirs[@]}" | sort)
fi

go_pkgs=()
for dir in "${go_dirs[@]}"; do
  if [[ $dir == . ]]; then
    go_pkgs+=("./...")
  else
    go_pkgs+=("./$dir/...")
  fi
done

web_sources=()
for file in "${web_files[@]}"; do
  rel=${file#web/}
  case $rel in
    e2e/*) continue ;;
    *.ts | *.tsx | *.js | *.jsx) web_sources+=("$rel") ;;
  esac
done
if ((${#web_sources[@]} > 0)); then
  mapfile -t web_sources < <(printf '%s\n' "${web_sources[@]}" | sort -u)
fi

plan=()
if ((${#go_files[@]} > 0)); then
  plan+=("golangci-lint run --new-from-rev=$base")
  if ((${#go_pkgs[@]} > 0)); then
    plan+=("go test ${go_pkgs[*]}")
  fi
fi
if ((${#web_files[@]} > 0)); then
  plan+=("cd web && npm run lint")
  plan+=("cd web && npm run typecheck")
  if ((${#web_sources[@]} > 0)); then
    plan+=("cd web && npx --no-install vitest related --passWithNoTests ${web_sources[*]}")
  fi
fi

if [[ $DRY_RUN == 1 ]]; then
  if ((${#plan[@]} > 0)); then
    printf '%s\n' "${plan[@]}"
  fi
  exit 0
fi

failures=()
run_check() {
  local label=$1 dir=$2
  shift 2
  local output status=0
  output=$(cd "$dir" && "$@" 2>&1) || status=$?
  if ((status != 0)); then
    printf 'stop-verify: %s failed (exit %d)\n' "$label" "$status" >&2
    failures+=("$label (exit $status)"$'\n'"$output")
  else
    printf 'stop-verify: %s passed\n' "$label" >&2
  fi
}

if ((${#go_files[@]} > 0)); then
  lint_base=$base
  [[ -n $lint_base ]] || lint_base=HEAD
  if command -v golangci-lint >/dev/null 2>&1; then
    run_check "golangci-lint run --new-from-rev=$lint_base" "$root" golangci-lint run --new-from-rev="$lint_base"
  else
    printf 'stop-verify: golangci-lint not found; skipping the lint check\n' >&2
  fi
  if ((${#go_pkgs[@]} > 0)); then
    run_check "go test ${go_pkgs[*]}" "$root" go test "${go_pkgs[@]}"
  fi
fi

if ((${#web_files[@]} > 0)); then
  if ! command -v npm >/dev/null 2>&1; then
    printf 'stop-verify: npm not found; skipping the web checks\n' >&2
  elif [[ ! -d $root/web/node_modules ]]; then
    printf 'stop-verify: web/node_modules is missing; run npm ci in web/ to enable the web checks\n' >&2
  else
    run_check "npm run lint" "$root/web" npm run lint
    run_check "npm run typecheck" "$root/web" npm run typecheck
    existing=()
    for rel in "${web_sources[@]}"; do
      [[ -e $root/web/$rel ]] && existing+=("$rel")
    done
    if ((${#existing[@]} > 0)); then
      run_check "vitest related ${existing[*]}" "$root/web" npx --no-install vitest related --passWithNoTests "${existing[@]}"
    fi
  fi
fi

((${#failures[@]} > 0)) || exit 0

reason=""
for failure in "${failures[@]}"; do
  if [[ -n $reason ]]; then
    reason+=$'\n\n'
  fi
  reason+=$failure
done
reason=${reason%$'\n'}
if ((${#reason} > LIMIT)); then
  reason="...${reason: -LIMIT}"
fi

if command -v python3 >/dev/null 2>&1; then
  REASON=$reason python3 -c 'import json, os; print(json.dumps({"decision": "block", "reason": os.environ["REASON"]}))'
elif command -v jq >/dev/null 2>&1; then
  printf '%s' "$reason" | jq -Rsc '{decision: "block", reason: .}'
else
  printf '{"decision":"block","reason":"'
  printf '%s' "$reason" | awk 'BEGIN { ORS = "" } NR > 1 { print "\\n" } { gsub(/\\/, "\\\\"); gsub(/"/, "\\\""); print }'
  printf '"}\n'
fi

exit 0

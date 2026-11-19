#!/usr/bin/env bash
set -euo pipefail

payload=$(cat)
file_path=""

if command -v jq >/dev/null 2>&1; then
  file_path=$(printf '%s' "$payload" | jq -r '.tool_input.file_path // empty' || true)
elif command -v node >/dev/null 2>&1; then
  file_path=$(printf '%s' "$payload" | node -e 'let s="";process.stdin.on("data",c=>s+=c).on("end",()=>{try{const j=JSON.parse(s);process.stdout.write((j.tool_input&&j.tool_input.file_path)||"")}catch{}})' || true)
fi

[[ -n "$file_path" ]] || exit 0

case "$file_path" in
  *.go|*.ts|*.tsx) ;;
  *) exit 0 ;;
esac

[[ -f "$file_path" ]] || exit 0

cd "$(dirname "$file_path")"
root=$(git rev-parse --show-toplevel 2>/dev/null || true)
if [[ -n "$root" ]]; then
  cd "$root"
fi
while [[ ! -f go.mod && "$PWD" != "/" ]]; do
  cd ..
done
[[ -f go.mod ]] || exit 0

set +e
output=$(go run ./backend/cmd/nocomments -- "$file_path" 2>&1)
status=$?
set -e

if ((status != 0)); then
  printf '%s\n' "$output" | grep -v '^exit status ' >&2 || true
  exit 2
fi
